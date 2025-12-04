package component

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestUserComponent_CheckIfUserHasOrgs(t *testing.T) {
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().GetUserOwnOrgs(context.TODO(), "user1").Return([]database.Organization{}, 0, nil)
	mockOrgStore.EXPECT().GetUserOwnOrgs(context.TODO(), "user2").Return([]database.Organization{
		{ID: 1},
	}, 1, nil)
	uc := &userComponentImpl{
		orgStore: mockOrgStore,
	}

	has, err := uc.CheckIfUserHasOrgs(context.TODO(), "user1")
	require.Nil(t, err)
	require.False(t, has)

	has, err = uc.CheckIfUserHasOrgs(context.TODO(), "user2")
	require.Nil(t, err)
	require.True(t, has)
}
func TestUserComponent_FindByUUIDs(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)

	uuids := []string{"uuid1", "uuid2"}

	mockUserStore.EXPECT().FindByUUIDs(context.TODO(), uuids).Return([]*database.User{
		{
			ID:       1,
			Username: "user1",
		},
		{
			ID:       2,
			Username: "user2",
		},
	}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
	}

	users, err := uc.FindByUUIDs(context.TODO(), uuids)

	require.Nil(t, err)
	require.Len(t, users, 2)

	require.Equal(t, int64(1), users[0].ID)
	require.Equal(t, "user1", users[0].Username)

	require.Equal(t, int64(2), users[1].ID)
	require.Equal(t, "user2", users[1].Username)
}

func TestUserComponent_SoftDelete(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	user := database.User{
		Username: "user1",
	}
	mockAuditStore.EXPECT().Create(context.TODO(), mock.Anything).Return(nil)
	mockUserStore.EXPECT().SoftDeleteUserAndRelations(context.TODO(), user, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsername(context.TODO(), user.Username).Return(user, nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(context.TODO(), user.Username).Return(user, nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		audit:     mockAuditStore,
	}

	err := uc.SoftDelete(context.TODO(), "user1", "user2", types.CloseAccountReq{})
	require.NotNil(t, err)

	err = uc.SoftDelete(context.TODO(), "user1", "user1", types.CloseAccountReq{})
	require.Nil(t, err)
}

func TestUserComponent_ResetUserTags(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		Username: "user1",
		UUID:     "uuid1",
	}
	tagIds := []int64{1, 2}
	mockUserStore.EXPECT().FindByUUID(context.TODO(), user.UUID).Return(user, nil)
	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), tagIds).Return(nil)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), user.ID, mock.Anything).Return(nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}

	err := uc.ResetUserTags(context.TODO(), user.UUID, tagIds)
	require.Nil(t, err)
}

func TestUserComponent_Delete(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockPendingDeletionStore := mockdb.NewMockPendingDeletionStore(t)
	mockGitserver := mockgit.NewMockGitServer(t)
	user1 := database.User{
		Username: "user1",
	}
	user2 := database.User{
		Username: "user2",
	}
	mockAuditStore.EXPECT().Create(context.TODO(), mock.Anything).Return(nil)
	mockUserStore.EXPECT().DeleteUserAndRelations(context.TODO(), user2, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(context.TODO(), user2.Username).Return(user2, nil)
	mockUserStore.EXPECT().FindByUsername(context.TODO(), user1.Username).Return(user1, nil)
	mockRepoStore.EXPECT().ByUser(context.TODO(), user2.ID, 1000, 0).Return([]database.Repository{{
		Path:           "foo/bar",
		RepositoryType: types.ModelRepo,
	}}, nil)
	mockRepoStore.EXPECT().ByUser(context.TODO(), user2.ID, 1000, 1).Return([]database.Repository{}, nil)
	mockPendingDeletionStore.EXPECT().Create(context.TODO(), &database.PendingDeletion{
		TableName: "repositories",
		Value:     "models_foo/bar.git",
	}).Return(nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		audit:     mockAuditStore,
		repo:      mockRepoStore,
		gs:        mockGitserver,
		pdStore:   mockPendingDeletionStore,
		config:    &config.Config{},
	}
	uc.config.GitServer.Type = types.GitServerTypeGitaly

	err := uc.Delete(context.TODO(), "user1", "user2")
	require.Nil(t, err)
}

func TestUserComponent_SendSMSCode(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockNotificationSvcClient := mockrpc.NewMockNotificationSvcClient(t)
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(&database.User{
		ID: 1,
	}, nil)
	mockNotificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		return req.Scenario == types.MessageScenarioSMSVerifyCode
	})).Return(nil)

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	config := &config.Config{}
	config.Notification.SMSTemplateCodeForVerifyCodeOversea = "test"
	config.Notification.SMSTemplateCodeForVerifyCodeCN = "test"
	config.Notification.SMSSign = "test"

	uc := &userComponentImpl{
		userStore:       mockUserStore,
		notificationSvc: mockNotificationSvcClient,
		cache:           cache,
		config:          config,
	}
	resp, err := uc.SendSMSCode(context.TODO(), "user1", types.SendSMSCodeRequest{
		Phone:     "13626487789",
		PhoneArea: "+86",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.ExpiredAt)
}

func TestUserComponent_SendSMSCode_InvalidPhoneNumber(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(&database.User{
		ID: 1,
	}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
	}
	resp, err := uc.SendSMSCode(context.TODO(), "user1", types.SendSMSCodeRequest{
		Phone:     "66668877",
		PhoneArea: "+86",
	})
	require.NotNil(t, err)
	require.Nil(t, resp)
}

func TestUserComponent_UpdatePhone(t *testing.T) {
	var code = "123456"
	var phone = "13626487789"
	var phoneArea = "+86"

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(&database.User{
		ID:          int64(1),
		Phone:       "13626487711",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}, nil)
	mockUserStore.EXPECT().UpdatePhone(mock.Anything, int64(1), "13626487789", "+86").Return(nil)

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)
	cache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByPhone(mock.Anything, phone).Return(false, nil)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	uc := &userComponentImpl{
		userStore: mockUserStore,
		cache:     cache,
		sso:       ssomock,
		config:    config,
	}
	req := &types.UpdateUserPhoneRequest{
		Phone:            &phone,
		PhoneArea:        &phoneArea,
		VerificationCode: &code,
	}
	err := uc.UpdatePhone(context.TODO(), "user1", *req)
	require.Nil(t, err)
}

func TestUserComponent_SendPublicSMSCode(t *testing.T) {
	mockNotificationSvcClient := mockrpc.NewMockNotificationSvcClient(t)
	mockNotificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		return req.Scenario == types.MessageScenarioSMSVerifyCode
	})).Return(nil)

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	config := &config.Config{}
	config.Notification.SMSTemplateCodeForVerifyCodeOversea = "test"
	config.Notification.SMSTemplateCodeForVerifyCodeCN = "test"
	config.Notification.SMSSign = "test"

	uc := &userComponentImpl{
		notificationSvc: mockNotificationSvcClient,
		cache:           cache,
		config:          config,
	}
	resp, err := uc.SendPublicSMSCode(context.TODO(), types.SendPublicSMSCodeRequest{
		Scene:     "submit-form",
		Phone:     "13626487789",
		PhoneArea: "+86",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.ExpiredAt)
}

func TestUserComponent_SendPublicSMSCode_InvalidPhoneNumber(t *testing.T) {
	uc := &userComponentImpl{}
	resp, err := uc.SendPublicSMSCode(context.TODO(), types.SendPublicSMSCodeRequest{
		Scene:     "submit-form",
		Phone:     "66668877",
		PhoneArea: "+86",
	})
	require.NotNil(t, err)
	require.Nil(t, resp)
	require.ErrorIs(t, err, errorx.ErrInvalidPhoneNumber)
}

func TestUserComponent_SendPublicSMSCode_PhoneAreaWithoutPrefix(t *testing.T) {
	mockNotificationSvcClient := mockrpc.NewMockNotificationSvcClient(t)
	mockNotificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
		return req.Scenario == types.MessageScenarioSMSVerifyCode
	})).Return(nil)

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().SetNX(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	config := &config.Config{}
	config.Notification.SMSTemplateCodeForVerifyCodeOversea = "test"
	config.Notification.SMSTemplateCodeForVerifyCodeCN = "test"
	config.Notification.SMSSign = "test"

	uc := &userComponentImpl{
		notificationSvc: mockNotificationSvcClient,
		cache:           cache,
		config:          config,
	}
	resp, err := uc.SendPublicSMSCode(context.TODO(), types.SendPublicSMSCodeRequest{
		Scene:     "submit-form",
		Phone:     "13626487789",
		PhoneArea: "86",
	})
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.ExpiredAt)
}

func TestUserComponent_VerifyPublicSMSCode(t *testing.T) {
	code := "123456"
	phone := "13626487789"
	phoneArea := "+86"
	scene := "submit-form"

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	cache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	uc := &userComponentImpl{
		cache: cache,
	}
	req := types.VerifyPublicSMSCodeRequest{
		Scene:            scene,
		Phone:            phone,
		PhoneArea:        phoneArea,
		VerificationCode: code,
	}
	err := uc.VerifyPublicSMSCode(context.TODO(), req)
	require.Nil(t, err)
}

func TestUserComponent_VerifyPublicSMSCode_InvalidCode(t *testing.T) {
	wrongCode := "654321"
	phone := "13626487789"
	phoneArea := "+86"
	scene := "submit-form"

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)

	uc := &userComponentImpl{
		cache: cache,
	}
	req := types.VerifyPublicSMSCodeRequest{
		Scene:            scene,
		Phone:            phone,
		PhoneArea:        phoneArea,
		VerificationCode: wrongCode,
	}
	err := uc.VerifyPublicSMSCode(context.TODO(), req)
	require.NotNil(t, err)
	require.ErrorIs(t, err, errorx.ErrPhoneVerifyCodeInvalid)
}

func TestUserComponent_VerifyPublicSMSCode_ExpiredCode(t *testing.T) {
	code := "123456"
	phone := "13626487789"
	phoneArea := "+86"
	scene := "submit-form"

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Get(mock.Anything, mock.Anything).Return("", redis.Nil)

	uc := &userComponentImpl{
		cache: cache,
	}
	req := types.VerifyPublicSMSCodeRequest{
		Scene:            scene,
		Phone:            phone,
		PhoneArea:        phoneArea,
		VerificationCode: code,
	}
	err := uc.VerifyPublicSMSCode(context.TODO(), req)
	require.NotNil(t, err)
	require.ErrorIs(t, err, errorx.ErrPhoneVerifyCodeExpiredOrNotFound)
}

func TestUserComponent_VerifyPublicSMSCode_PhoneAreaWithoutPrefix(t *testing.T) {
	code := "123456"
	phone := "13626487789"
	phoneArea := "86"
	scene := "submit-form"

	cache := mockcache.NewMockRedisClient(t)
	cache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	cache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	uc := &userComponentImpl{
		cache: cache,
	}
	req := types.VerifyPublicSMSCodeRequest{
		Scene:            scene,
		Phone:            phone,
		PhoneArea:        phoneArea,
		VerificationCode: code,
	}
	err := uc.VerifyPublicSMSCode(context.TODO(), req)
	require.Nil(t, err)
}

// test update UpdateByUUID
func TestUserComponent_UpdateByUUID_UpdateUserName(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:                1,
		UUID:              "user1",
		Username:          "user1",
		Phone:             "13626487789",
		PhoneArea:         "+86",
		RegProvider:       "casdoor",
		CanChangeUserName: true,
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByUUID(context.TODO(), mock.Anything).Return(user, nil)

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "new_user1").Return(database.User{}, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Name == "new_user1"
	})).Return(nil)
	ssomock.EXPECT().IsExistByName(mock.Anything, "new_user1").Return(false, nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)
	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}
	var userUUID = "user1"
	var newUserName = "new_user1"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:        &userUUID,
		OpUser:      "user1",
		NewUserName: &newUserName,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdateEmail(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByEmail(mock.Anything, "new@example.com").Return(false, nil)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "new@example.com"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
		cache:     mockCache,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenUsernameNotChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when username doesn't change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}
	var userUUID = "user1"
	var sameUserName = "user1" // same as current username
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:        &userUUID,
		OpUser:      "user1",
		NewUserName: &sameUserName,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenEmailNotChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when email doesn't change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}
	var userUUID = "user1"
	var sameEmail = "user1@example.com" // same as current email
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &sameEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncForNonSSOUser(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "default", // non-SSO user
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called for non-SSO users

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
		cache:     mockCache,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenOnlyOtherFieldsChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when only other fields (like avatar, bio) change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}
	var userUUID = "user1"
	var avatar = "https://example.com/avatar.jpg"
	var bio = "New bio"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Avatar: &avatar,
		Bio:    &bio,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_RollbackWhenDBUpdateFails(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByEmail(mock.Anything, "new@example.com").Return(false, nil)
	// First SSO update should succeed
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "new@example.com"
	})).Return(nil)
	// Rollback should be called when DB update fails
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "old@example.com" && params.Name == "user1"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
		cache:     mockCache,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to update user in db")
}

func TestUserComponent_UpdateByUUID_NoRollbackWhenNoSync(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))

	ssomock := mockrpc.NewMockSSOInterface(t)
	// No SSO sync should be called since no username/email change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	uc := &userComponentImpl{
		userStore: mockUserStore,
		sso:       ssomock,
		config:    config,
		once:      &once,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}
	var userUUID = "user1"
	var avatar = "https://example.com/avatar.jpg"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Avatar: &avatar,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to update user in db")
}

func TestUserComponent_checkUserConflictsInDB(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		email         string
		mockSetup     func(*mockdb.MockUserStore)
		expectedError error
		expectError   bool
	}{
		{
			name:     "no conflicts - username and email available",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "no conflicts - username available, no email provided",
			username: "newuser",
			email:    "",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "username conflict",
			username: "existinguser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "existinguser").Return(true, nil)
			},
			expectedError: errorx.UsernameExists("existinguser"),
			expectError:   true,
		},
		{
			name:     "email conflict",
			username: "newuser",
			email:    "existing@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "existing@example.com").Return(database.User{ID: 123}, nil)
			},
			expectedError: errorx.EmailExists("existing@example.com"),
			expectError:   true,
		},
		{
			name:     "username check database error",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, errors.New("database connection error"))
			},
			expectedError: nil,
			expectError:   true,
		},
		{
			name:     "email check database error",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{}, errors.New("database connection error"))
			},
			expectedError: nil,
			expectError:   true,
		},
		{
			name:     "email check returns ErrNoRows - no conflict",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "email check returns user with ID 0 - no conflict",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, nil)
			},
			expectedError: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserStore := mockdb.NewMockUserStore(t)
			tt.mockSetup(mockUserStore)

			uc := &userComponentImpl{
				userStore: mockUserStore,
			}

			err := uc.checkUserConflictsInDB(context.Background(), tt.username, tt.email)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedError != nil {
					// Check if the error is the expected custom error type
					var customErr errorx.CustomError
					if errors.As(err, &customErr) {
						require.True(t, customErr.Is(tt.expectedError), "Expected error type %v, got %v", tt.expectedError, err)
					} else {
						require.Contains(t, err.Error(), "failed to check", "Expected database error message")
					}
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUserComponent_StreamExportUsers(t *testing.T) {
	ctx := context.Background()
	req := types.UserIndexReq{
		Search: "u-foo",
		Per:    10,
	}

	data := make(chan database.Wrapper)
	go func() {
		defer close(data)
		data <- database.Wrapper{
			Users: []database.User{
				{Username: "u-foo"},
			},
		}
	}()
	us := mockdb.NewMockUserStore(t)
	us.EXPECT().IndexWithCursor(mock.Anything, req).Return(data, nil)

	uc := &userComponentImpl{
		userStore: us,
	}

	ch, err := uc.StreamExportUsers(ctx, req)
	require.NoError(t, err)

	total := 0
	for resp := range ch {
		require.NoError(t, resp.Error)
		for _, u := range resp.Users {
			require.Equal(t, "u-foo", u.Username)
		}
		total += len(resp.Users)
	}

	require.Equal(t, 1, total)
}
