package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
	"opencsg.com/csghub-server/common/utils/common"
)

const GitalyRepoNotFoundErr = "rpc error: code = NotFound desc = repository does not exist"

type userComponentImpl struct {
	userStore database.UserStore
	orgStore  database.OrgStore
	nsStore   database.NamespaceStore
	repo      database.RepoStore
	ds        database.DeployTaskStore
	ams       database.AccountMeteringStore
	asqs      database.AccountSyncQuotaStore
	aus       database.AccountUserStore
	audit     database.AuditLogStore
	pdStore   database.PendingDeletionStore

	gs          gitserver.GitServer
	jwtc        JwtComponent
	tokenc      AccessTokenComponent
	userPhonec  UserPhoneComponent
	invitationc InvitationComponent

	// casc      *casdoorsdk.Client
	// casConfig *casdoorsdk.AuthConfig
	sso    rpc.SSOInterface
	once   *sync.Once
	sfnode *snowflake.Node
	config *config.Config

	cache           cache.RedisClient
	notificationSvc rpc.NotificationSvcClient
	ts              database.TagStore
	uts             database.UserTagStore
}

type UserComponent interface {
	// ChangeUserName(ctx context.Context, oldUserName, newUserName, opUser string) error
	UpdateByUUID(ctx context.Context, req *types.UpdateUserRequest) error
	// Update(ctx context.Context, req *types.UpdateUserRequest, opUser string) error
	Delete(ctx context.Context, operator, username string) error
	// CanAdmin checks if a user has admin privileges.
	//
	// Parameters:
	// - ctx: The context.Context object for the function.
	// - username: The username of the user to check.
	//
	// Returns:
	// - bool: True if the user has admin privileges, false otherwise.
	// - error: An error if the user cannot be found in the database.
	CanAdmin(ctx context.Context, username string) (bool, error)
	// GetInternal get *full* user info by username or uuid
	//
	// should only be called by other *internal* services
	GetInternal(ctx context.Context, userNameOrUUID string, useUUID bool) (*types.User, error)
	Get(ctx context.Context, userNameOrUUID, visitorName string, useUUID bool) (*types.User, error)
	CheckOperatorAndUser(ctx context.Context, operator, username string) (bool, error)
	CheckIfUserHasOrgs(ctx context.Context, userName string) (bool, error)
	CheckIfUserHasRunningOrBuildingDeployments(ctx context.Context, userName string) (bool, error)
	CheckIfUserHasBills(ctx context.Context, userName string) (bool, error)
	Index(ctx context.Context, req types.UserListReq) ([]*types.User, int, error)
	Signin(ctx context.Context, code, state string) (*types.JWTClaims, string, error)
	FixUserData(ctx context.Context, userName string) error
	UpdateUserLabels(ctx context.Context, req *types.UserLabelsRequest) error
	FindByUUIDs(ctx context.Context, uuids []string) ([]*types.User, error)
	SoftDelete(ctx context.Context, operator, username string, req types.CloseAccountReq) error
	// get all user mail addresses with pagination
	GetEmails(ctx context.Context, visitorName string, per, page int) ([]string, int, error)
	// should only be called by other *internal* services (no permission check)
	GetEmailsInternal(ctx context.Context, per, page int) ([]string, int, error)
	GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error)
	GenerateVerificationCodeAndSendEmail(ctx context.Context, uid, email string) error
	ResetUserTags(ctx context.Context, uid string, tagIDs []int64) error
	StreamExportUsers(ctx context.Context, req types.UserIndexReq) (data chan types.UserIndexResp, err error)
}

func NewUserComponent(config *config.Config) (UserComponent, error) {
	var err error
	c := &userComponentImpl{}
	c.userStore = database.NewUserStore()
	c.orgStore = database.NewOrgStore()
	c.nsStore = database.NewNamespaceStore()
	c.repo = database.NewRepoStore()
	c.ds = database.NewDeployTaskStore()
	c.ams = database.NewAccountMeteringStore()
	c.asqs = database.NewAccountSyncQuotaStore()
	c.aus = database.NewAccountUserStore()
	c.audit = database.NewAuditLogStore()
	c.pdStore = database.NewPendingDeletionStore()
	c.jwtc = NewJwtComponent(config.JWT.SigningKey, config.JWT.ValidHour)
	c.tokenc, err = NewAccessTokenComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create access token component, error: %w", err)
	}
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		return nil, newError
	}
	c.once = new(sync.Once)
	c.config = config
	c.sso, err = rpc.NewSSOClient(c.config)
	if err != nil {
		slog.Error("failed to create sso client", "error", err)
		return nil, fmt.Errorf("failed to create sso client, error: %w", err)
	}

	cache, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create redis cache, error: %w", err)
	}
	c.cache = cache

	c.notificationSvc = rpc.NewNotificationSvcHttpClientBuilder(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
		rpc.AuthWithApiKey(config.APIToken)).WithRetry(3).WithDelay(time.Millisecond * 200).Build()

	c.ts = database.NewTagStore()
	c.uts = database.NewUserTagStore()

	c.invitationc, err = NewInvitationComponent(c.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation component, error: %w", err)
	}

	c.userPhonec, err = NewUserPhoneComponent(c.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user phone component, error: %w", err)
	}

	return c, nil
}

// // This function creates a user when user register from portal, without casdoor
// func (c *userComponentImpl) createFromPortalRegistry(ctx context.Context, req types.CreateUserRequest) (*database.User, error) {
// 	// Panic if the function has not been implemented
// 	panic("implement me later")
// }

func (c *userComponentImpl) checkUserConflictsInDB(ctx context.Context, username, email string) error {
	exists, err := c.userStore.IsExist(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to check username existence: %w", err)
	}
	if exists {
		return errorx.UsernameExists(username)
	}

	// Check email existence if email is provided
	if email != "" {
		user, err := c.userStore.FindByEmail(ctx, email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check email existence: %w", err)
		}
		if user.ID > 0 {
			return errorx.EmailExists(email)
		}
	}

	return nil
}

func (c *userComponentImpl) createFromSSOUser(ctx context.Context, cu *rpc.SSOUserInfo) (*database.User, error) {
	var (
		gsUserResp        *gitserver.CreateUserResponse
		err               error
		userName          string
		email             string
		canChangeUserName bool
	)
	//wechat user need to change username later
	if cu.WeChat != "" {
		userName, err = c.genUniqueName()
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique user name,error:%w", err)
		}
		canChangeUserName = true
		//set email to "", make sure not to create git user
		email = ""
	} else {
		userName = cu.Name
		canChangeUserName = false
		email = cu.Email
	}

	// Check for conflicts before proceeding
	if err := c.checkUserConflictsInDB(ctx, userName, email); err != nil {
		return nil, err
	}
	//skip creating git user if email is empty, it will be created later when user set email
	if email != "" {
		gsUserReq := gitserver.CreateUserRequest{
			Nickname: userName,
			Username: userName,
			Email:    email,
		}
		gsUserResp, err = c.gs.CreateUser(gsUserReq)
		if err != nil {
			newError := fmt.Errorf("failed to create gitserver user '%s',error:%w", cu.Name, err)
			return nil, newError
		}
	}

	namespace := &database.Namespace{
		Path: userName,
	}
	user := &database.User{
		Username:    userName,
		NickName:    userName,
		Email:       email,
		UUID:        cu.UUID,
		RegProvider: c.config.SSOType,
		Gender:      cu.Gender,
		// RoleMask:        "", //will be updated when admin set user role
		Phone:           cu.Phone,
		PhoneArea:       cu.PhoneArea,
		PhoneVerified:   false,
		EmailVerified:   false,
		LastLoginAt:     cu.LastSigninTime,
		Avatar:          cu.Avatar,
		CompanyVerified: false,
		// PasswordHash:    cu.Password,
		Homepage:          cu.Homepage,
		Bio:               cu.Bio,
		CanChangeUserName: canChangeUserName,
	}
	if gsUserResp != nil {
		user.GitID = gsUserResp.GitID
		user.Password = gsUserResp.Password
	}
	err = c.userStore.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user in db,error:%w", err)
		return nil, newError
	}

	return user, nil
}

func (c *userComponentImpl) UpdateByUUID(ctx context.Context, req *types.UpdateUserRequest) error {
	c.lazyInit()

	if req.UUID == nil {
		return errors.New("can not update user without uuid in request")
	}
	uuid := *req.UUID
	user, err := c.userStore.FindByUUID(ctx, uuid)
	if err != nil {
		return fmt.Errorf("failed to find user by uuid in db,error:%w", err)
	}
	if user == nil {
		return errorx.ErrUserNotFound
	}
	var oldUser = *user
	opUserName := req.OpUser
	var opUser database.User
	if user.Username != opUserName {
		//find op user by username
		opUser, err = c.userStore.FindByUsername(ctx, opUserName)
		if err != nil {
			return fmt.Errorf("failed to find op user by name in db,user: '%s', error:%w", opUserName, err)
		}
	} else {
		opUser = *user
	}

	shouldSyncToIAM := false
	if req.Roles != nil {
		if can, reason := c.canChangeRole(*user, opUser); !can {
			return errors.New(reason)
		}
	}

	if req.NewUserName != nil && user.Username != *req.NewUserName {
		if can, reason := c.canChangeUserName(ctx, *user, opUser, *req.NewUserName); !can {
			return errors.New(reason)
		}
		shouldSyncToIAM = true
	}

	if req.Email != nil && user.Email != *req.Email {
		if can, reason := c.canChangeEmail(ctx, *user, opUser, *req.Email); !can {
			return errors.New(reason)
		}

		if req.EmailVerificationCode == nil {
			return errors.New("email verification code is required")
		}
		err = c.VerifyVerificationCode(ctx, user.UUID, *req.Email, *req.EmailVerificationCode)
		if err != nil {
			return err
		}
		shouldSyncToIAM = true
	}

	// check phone
	// check whether phone needs to be updated
	needUpdatePhone := c.userPhonec.NeedPhoneChange()
	if !needUpdatePhone {
		req.Phone = nil
		req.PhoneArea = nil
	}

	var phoneArea = user.PhoneArea
	if req.Phone != nil && user.Phone != *req.Phone {
		can, err := c.userPhonec.CanChangePhone(ctx, user, *req.Phone)
		if err != nil {
			return err
		}
		if !can {
			return errorx.ErrForbidChangePhone
		}
		if req.PhoneArea != nil {
			normalizedPhoneArea := common.NormalizePhoneArea(*req.PhoneArea)
			if user.PhoneArea != normalizedPhoneArea {
				phoneArea = normalizedPhoneArea
			}
		}
		shouldSyncToIAM = true
	}

	if err := c.ResetUserTags(ctx, uuid, req.TagIDs); err != nil {
		return fmt.Errorf("failed to reset user tags,error:%w", err)
	}

	// update user in IAM first, then update user in db
	if shouldSyncToIAM && c.IsSSOUser(user.RegProvider) {
		var params = rpc.SSOUpdateUserInfo{
			UUID: uuid,
		}

		if req.Email != nil {
			params.Email = *req.Email
		}
		if req.NewUserName != nil {
			params.Name = *req.NewUserName
		}

		if req.Phone != nil {
			params.Phone = *req.Phone
			params.PhoneArea = phoneArea
		}

		err := c.sso.UpdateUserInfo(ctx, &params)
		if err != nil {
			return fmt.Errorf("failed to update user in sso, uuid:'%s',error:%w", user.UUID, err)
		}
	}

	changedUser := c.setChangedProps(user, req)
	if err := c.userStore.Update(ctx, changedUser, user.Username); err != nil {
		// rollback casdoor user change only if SSO update was performed
		if shouldSyncToIAM && c.IsSSOUser(user.RegProvider) {
			params := rpc.SSOUpdateUserInfo{
				UUID:   uuid,
				Name:   oldUser.Username,
				Email:  oldUser.Email,
				Gender: oldUser.Gender,
			}
			// update phone if needed (ce)
			if needUpdatePhone {
				params.Phone = oldUser.Phone
				params.PhoneArea = oldUser.PhoneArea
			}
			err := c.sso.UpdateUserInfo(ctx, &params)
			if err != nil {
				return fmt.Errorf("failed to rollback user change in sso, uuid:'%s',error:%w", user.UUID, err)
			}
		}
		return fmt.Errorf("failed to update user in db,error:%w", err)
	}

	return nil

}

func (c *userComponentImpl) canChangeRole(user, opuser database.User) (can bool, reason string) {
	if opuser.ID == user.ID {
		return false, "user can not change roles of self"
	}
	if !opuser.CanAdmin() {
		return false, "op user is not admin"
	}
	return true, ""
}

func (c *userComponentImpl) canChangeUserName(ctx context.Context, user, opuser database.User, newUserName string) (can bool, reason string) {
	if opuser.ID != user.ID {
		return false, "user name can only be changed by the user itself"
	}
	if !user.CanChangeUserName {
		return false, "user name can only be changed once"
	}
	// check username existence in db and casdoor
	u, err := c.userStore.FindByUsername(ctx, newUserName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, "failed to check new username existence in db"
		}
	}
	if u.ID > 0 {
		return false, fmt.Sprintf("new username '%s' already exists", newUserName)
	}

	if !c.IsSSOUser(user.RegProvider) {
		return true, ""
	}

	exist, err := c.sso.IsExistByName(ctx, newUserName)
	if err != nil {
		return false, "failed to check new username existence in casdoor"
	}
	if exist {
		return false, "user name already exists in casdoor"
	}
	return true, ""
}

func (c *userComponentImpl) canChangeEmail(ctx context.Context, user, opuser database.User, newEmail string) (can bool, reason string) {
	if opuser.ID != user.ID {
		return false, "email can only be changed by the user itself"
	}
	// check email existence in db and casdoor
	u, err := c.userStore.FindByEmail(ctx, newEmail)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, "failed to check new email existence in db"
		}
	}
	if u.ID > 0 {
		return false, fmt.Sprintf("email '%s' already exists", newEmail)
	}

	if !c.IsSSOUser(user.RegProvider) {
		return true, ""
	}

	exist, err := c.sso.IsExistByEmail(ctx, newEmail)
	if err != nil {
		return false, "failed to check new email existence in casdoor"
	}
	if exist {
		return false, "email already exists in casdoor"
	}

	return true, ""
}

// Depricated: only useful for gitea, will be removed in the future
// user registry with wechat does not have email, so git user is not created after signin
// when user set email, a git user needs to be created
// func (c *userComponentImpl) upsertGitUser(username string, nickname *string, oldEmail, newEmail string) error {
// 	var err error
// 	if nickname == nil {
// 		nickname = &username
// 	}
// 	if oldEmail == "" {
// 		// create git user
// 		gsUserReq := gitserver.CreateUserRequest{
// 			Nickname: *nickname,
// 			Username: username,
// 			Email:    newEmail,
// 		}
// 		_, err = c.gs.CreateUser(gsUserReq)
// 		if err != nil {
// 			newError := fmt.Errorf("failed to create git user '%s',error:%w", username, err)
// 			return newError
// 		}
// 	} else {
// 		// update git user
// 		err = c.gs.UpdateUserV2(gitserver.UpdateUserRequest{
// 			Nickname: nickname,
// 			Username: username,
// 			Email:    &newEmail,
// 		})
// 		if err != nil {
// 			newError := fmt.Errorf("failed to update git user '%s',error:%w", username, err)
// 			return newError
// 		}
// 	}

// 	return nil
// }

func (c *userComponentImpl) setChangedProps(oldUser *database.User, req *types.UpdateUserRequest) *database.User {
	user := *oldUser
	if req.NewUserName != nil {
		user.CanChangeUserName = false // user name can only be changed once
		user.Username = *req.NewUserName
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.Homepage != nil {
		user.Homepage = *req.Homepage
	}
	if req.Nickname != nil {
		user.NickName = *req.Nickname
	}
	if req.Roles != nil {
		user.SetRoles(*req.Roles)
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.PhoneArea != nil {
		user.PhoneArea = common.NormalizePhoneArea(*req.PhoneArea) // normalize phone area
	}

	return &user
}

func (c *userComponentImpl) Delete(ctx context.Context, operator, username string) error {
	var retainData types.CloseAccountReq
	user, err := c.userStore.FindByUsernameWithDeleted(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return newError
	}

	opUser, err := c.userStore.FindByUsername(ctx, operator)
	if err != nil {
		newError := fmt.Errorf("failed to find operator by name in db,error:%w", err)
		return newError
	}

	// TODO:delete user from git server
	slog.DebugContext(ctx, "delete user from git server", slog.String("operator", operator), slog.String("username", user.Username))

	// if c.config.GitServer.Type == types.GitServerTypeGitea {
	// 	// gitea gitserver does not support delete user, you could create a pr to our repo to fix it
	// }

	if user.RetainData != "" {
		err = json.Unmarshal([]byte(user.RetainData), &retainData)
		if err != nil {
			return fmt.Errorf("error unmarshalling retain data: %w", err)
		}
	}

	if !retainData.Repository && c.config.GitServer.Type == types.GitServerTypeGitaly {
		var (
			batchSize = 1000
			batch     = 0
		)
		for {
			repos, err := c.repo.ByUser(ctx, user.ID, batchSize, batch)
			if err != nil {
				slog.ErrorContext(ctx, "failed to find all repos for user", slog.String("username", user.Username), slog.Any("error", err))
				return fmt.Errorf("failed to find all repos for user: %v", err)
			}

			if len(repos) == 0 {
				break
			}

			for _, repo := range repos {
				if repo.Path == "" {
					continue
				}
				err = c.pdStore.Create(ctx, &database.PendingDeletion{
					TableName: database.PendingDeletionTableNameRepository,
					Value:     repo.GitalyPath(),
				})
				if err != nil {
					slog.ErrorContext(ctx, "failed to create pending deletion", slog.Any("error", err))
					return fmt.Errorf("failed to create pending deletion: %w", err)
				}
			}
			batch++
		}
	}
	// generate audit log
	before, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user before delete")
	}
	audit := &database.AuditLog{
		TableName:  "users",
		Action:     enum.AuditActionDeletion,
		OperatorID: opUser.ID,
		Before:     string(before),
		After:      "",
	}

	// delete user from db
	err = c.userStore.DeleteUserAndRelations(ctx, user, retainData)
	if err != nil {
		return fmt.Errorf("failed to delete user and user relations: %v", err)
	}

	// create audit log after delete user
	err = c.audit.Create(ctx, audit)
	if err != nil {
		return fmt.Errorf("failed to create audit log,error:%w", err)
	}

	// delete user from casdoor
	if user.UUID == "" {
		return nil
	}

	if c.IsSSOUser(user.RegProvider) {
		err = c.sso.DeleteUser(ctx, user.UUID)
		if err != nil {
			return fmt.Errorf("failed to delete user in sso: %v", err)
		}
	}

	return nil
}

// CanAdmin checks if a user has admin privileges.
//
// Parameters:
// - ctx: The context.Context object for the function.
// - username: The username of the user to check.
//
// Returns:
// - bool: True if the user has admin privileges, false otherwise.
// - error: An error if the user cannot be found in the database.
func (c *userComponentImpl) CanAdmin(ctx context.Context, username string) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name '%s' in db,error:%w", username, err)
		return false, newError
	}
	return user.CanAdmin(), nil
}

// GetInternal get *full* user info by username or uuid
//
// should only be called by other *internal* services
func (c *userComponentImpl) GetInternal(ctx context.Context, userNameOrUUID string, useUUID bool) (*types.User, error) {
	var dbuser = new(database.User)
	var err error
	if useUUID {
		dbuser, err = c.userStore.FindByUUID(ctx, userNameOrUUID)
	} else {
		*dbuser, err = c.userStore.FindByUsername(ctx, userNameOrUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by name or uuid '%s' in db,error:%w", userNameOrUUID, err)
	}
	return c.buildUserInfo(ctx, dbuser, false)
}

func (c *userComponentImpl) Get(ctx context.Context, userNameOrUUID, visitorName string, useUUID bool) (*types.User, error) {
	var dbuser = new(database.User)
	var err error
	if useUUID {
		dbuser, err = c.userStore.FindByUUID(ctx, userNameOrUUID)
	} else {
		*dbuser, err = c.userStore.FindByUsername(ctx, userNameOrUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by name or uuid  '%s' in db,error:%w", userNameOrUUID, err)
	}
	userName := dbuser.Username
	var onlyBasicInfo bool
	//allow anonymous user to get basic info
	if visitorName == "" {
		onlyBasicInfo = true
	} else if userName != visitorName {
		canAdmin, err := c.CanAdmin(ctx, visitorName)
		if err != nil {
			return nil, fmt.Errorf("failed to check visitor user permission, visitor: %s, error: %w", visitorName, err)
		}

		if !canAdmin {
			onlyBasicInfo = true
		}
	}

	return c.buildUserInfo(ctx, dbuser, onlyBasicInfo)
}

func (c *userComponentImpl) CheckOperatorAndUser(ctx context.Context, operator, username string) (bool, error) {
	opUser, err := c.userStore.FindByUsername(ctx, operator)
	if err != nil {
		newError := fmt.Errorf("failed to find operator by name in db,error:%w", err)
		return true, newError
	}

	user, err := c.userStore.FindByUsernameWithDeleted(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return true, newError
	}
	if !opUser.CanAdmin() {
		return false, errors.New("only admin user or the user can delete the user")
	}

	if user.CanAdmin() {
		return false, errors.New("admin user can not be deleted")
	}
	return false, nil
}

func (c *userComponentImpl) CheckIfUserHasOrgs(ctx context.Context, userName string) (bool, error) {
	var (
		err   error
		total int
	)
	if _, total, err = c.orgStore.GetUserOwnOrgs(ctx, userName); err != nil {
		return false, fmt.Errorf("failed to find orgs by username in db,error:%w", err)
	}
	return total > 0, nil
}

func (c *userComponentImpl) CheckIfUserHasRunningOrBuildingDeployments(ctx context.Context, userName string) (bool, error) {
	user, err := c.userStore.FindByUsernameWithDeleted(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("failed to find user by username in db, error: %v", err)
	}
	deploys, err := c.ds.ListAllDeployByUID(ctx, user.ID)
	if err != nil {
		return false, fmt.Errorf("failed to list all deployments for user %s in db, error:  %v", userName, err)
	}
	if len(deploys) > 0 {
		return true, nil
	}
	return false, nil
}

func (c *userComponentImpl) CheckIfUserHasBills(ctx context.Context, userName string) (bool, error) {
	user, err := c.userStore.FindByUsernameWithDeleted(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("failed to find user by username in db, error: %v", err)
	}
	ams, err := c.ams.ListAllByUserUUID(ctx, user.UUID)
	if err != nil {
		return false, fmt.Errorf("failed to list all account meterings for user %s in db, error: %w", userName, err)
	}
	if len(ams) > 0 {
		return true, nil
	}

	aus, err := c.aus.ListAllByUserUUID(ctx, user.UUID)
	if err != nil {
		return false, fmt.Errorf("failed to list all account users for user %s in db, error: %w", userName, err)
	}
	if len(aus) > 0 {
		return true, nil
	}

	return false, nil
}

func (c *userComponentImpl) buildUserInfo(ctx context.Context, dbuser *database.User, onlyBasicInfo bool) (*types.User, error) {
	var tags []types.RepoTag

	utags, err := c.uts.GetUserTags(ctx, dbuser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tags for user %s,error:%w", dbuser.Username, err)
	}

	for _, utag := range utags {
		tags = append(tags, types.RepoTag{
			ID:       utag.ID,
			Name:     utag.Name,
			Category: utag.Category,
			Group:    utag.Group,
			BuiltIn:  utag.BuiltIn,
			Scope:    utag.Scope,
			I18nKey:  utag.I18nKey,
		})
	}

	u := types.User{
		Username: dbuser.Username,
		Nickname: dbuser.NickName,
		Avatar:   dbuser.Avatar,
		Tags:     tags,
	}

	if !onlyBasicInfo {
		u.ID = dbuser.ID
		u.Email = dbuser.Email
		u.UUID = dbuser.UUID
		u.Bio = dbuser.Bio
		u.Homepage = dbuser.Homepage
		u.PhoneArea = dbuser.PhoneArea
		u.Phone = dbuser.Phone
		u.Roles = dbuser.Roles()
		u.VerifyStatus = string(dbuser.VerifyStatus)
		u.Labels = dbuser.Labels
		u.CreatedAt = dbuser.CreatedAt
	}

	dborgs, err := c.orgStore.GetUserBelongOrgs(ctx, dbuser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get orgs for user %s,error:%w", dbuser.Username, err)
	}

	if len(dborgs) > 0 {
		for _, org := range dborgs {
			u.Orgs = append(u.Orgs, types.Organization{
				Name:     org.Name,
				Nickname: org.Nickname,
				Homepage: org.Homepage,
				Logo:     org.Logo,
				OrgType:  org.OrgType,
				Verified: org.Verified,
				UserID:   org.UserID,
			})
		}
	}

	return &u, nil
}

func (c *userComponentImpl) Index(ctx context.Context, req types.UserListReq) ([]*types.User, int, error) {
	var (
		respUsers     []*types.User
		onlyBasicInfo bool
	)
	canAdmin, err := c.CanAdmin(ctx, req.VisitorName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check visitor user permission, visitor: %s, error: %w", req.VisitorName, err)
	}
	if !canAdmin {
		onlyBasicInfo = true
	}
	dbusers, count, err := c.userStore.IndexWithSearch(ctx, req)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return nil, count, newError
	}

	for _, dbuser := range dbusers {
		var tags []types.RepoTag

		for _, utag := range dbuser.Tags {
			tags = append(tags, types.RepoTag{
				ID:       utag.ID,
				Name:     utag.Tag.Name,
				Category: utag.Tag.Category,
				Group:    utag.Tag.Group,
				BuiltIn:  utag.Tag.BuiltIn,
				Scope:    utag.Tag.Scope,
				I18nKey:  utag.Tag.I18nKey,
			})
		}

		user := &types.User{
			Username: dbuser.Username,
			Nickname: dbuser.NickName,
			Avatar:   dbuser.Avatar,
			Tags:     tags,
		}

		if !onlyBasicInfo {
			user.Email = dbuser.Email
			user.UUID = dbuser.UUID
			user.Bio = dbuser.Bio
			user.Homepage = dbuser.Homepage
			user.Phone = dbuser.Phone
			user.PhoneArea = dbuser.PhoneArea
			user.Roles = dbuser.Roles()
			user.VerifyStatus = string(dbuser.VerifyStatus)
			user.Labels = dbuser.Labels
			user.LastLoginAt = dbuser.LastLoginAt
			user.CreatedAt = dbuser.CreatedAt
		}

		respUsers = append(respUsers, user)
	}

	return respUsers, count, nil
}

func (c *userComponentImpl) Signin(ctx context.Context, code, state string) (*types.JWTClaims, string, error) {
	c.lazyInit()

	casToken, err := c.sso.GetOAuthToken(ctx, code, state)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token from casdoor,error:%w", err)
	}
	// claims, err := c.casc.ParseJwtToken(casToken.AccessToken)
	// if err != nil {
	// 	return nil, "", fmt.Errorf("failed to parse token from casdoor,error:%w", err)
	// }

	cu, err := c.sso.GetUserInfo(ctx, casToken.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user info from casdoor,error:%w", err)
	}

	exists, err := c.userStore.IsExistByUUID(ctx, cu.UUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check user existence by name in db,error:%w", err)
	}

	var dbu *database.User
	if !exists {
		dbu, err = c.createFromSSOUser(ctx, cu)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create user,error:%w", err)
		}
		// auto create git access token for the new user
		go func(username string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			_, err := c.tokenc.Create(ctx, &types.CreateUserTokenRequest{
				Username:    username,
				TokenName:   uuid.NewString(),
				Application: types.AccessTokenAppGit,
			})
			if err != nil {
				slog.ErrorContext(ctx, "failed to create git user access token", "error", err, slog.Any("username", dbu.Username))
			}
		}(dbu.Username)

		if dbu.Phone != "" {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
				defer cancel()
				if err := c.invitationc.AwardCreditToInvitee(ctx, types.AwardCreditToInviteeReq{
					InviteeUUID: dbu.UUID,
					InviteeName: dbu.Username,
					RegisterAt:  dbu.CreatedAt,
				}); err != nil {
					slog.ErrorContext(ctx, "failed to award credit to invitee", "error", err, "invitee_uuid", dbu.UUID)
				}
			}()
		}
	} else {
		// get user from db for username, as casdoor may have different username
		dbu, err = c.userStore.FindByUUID(ctx, cu.UUID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to find user by uuid in db, uuid:%s, error:%w", cu.UUID, err)
		}
		// update user login time asynchronously
		go func() {
			updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second*5)
			defer cancel()
			dbu.LastLoginAt = time.Now().Format("2006-01-02 15:04:05")
			err := c.userStore.Update(updateCtx, dbu, "")
			if err != nil {
				slog.ErrorContext(ctx, "failed to update user login time", "error", err, "username", dbu.Username)
			}
		}()
	}
	hubToken, signed, err := c.jwtc.GenerateToken(ctx, types.CreateJWTReq{
		UUID: dbu.UUID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate jwt token,error:%w", err)
	}

	return hubToken, signed, nil
}

func (c *userComponentImpl) genUniqueName() (string, error) {
	c.lazyInit()

	if c.sfnode == nil {
		return "", fmt.Errorf("user component sfnode is nil, %w", errorx.ErrInternalServerError)
	}
	id := c.sfnode.Generate().Base36()
	return "user_" + id, nil
}

func (c *userComponentImpl) lazyInit() {
	c.once.Do(func() {
		var err error
		c.sfnode, err = snowflake.NewNode(1)
		if err != nil {
			slog.Error("failed to create snowflake node", "error", err)
		}
	})
}

func (c *userComponentImpl) FixUserData(ctx context.Context, userName string) error {
	err := c.gs.FixUserData(ctx, userName)
	if err != nil {
		return err
	}

	return nil
}

func (c *userComponentImpl) UpdateUserLabels(ctx context.Context, req *types.UserLabelsRequest) error {
	isAdmin, err := c.CanAdmin(ctx, req.OpUser)
	if err != nil {
		return fmt.Errorf("failed to check visitor user permission, userName: %s, error: %w", req.OpUser, err)
	}
	if !isAdmin {
		return fmt.Errorf("permission denied: cannot modify user labels. username: %s", req.OpUser)
	}
	err = c.userStore.UpdateLabels(ctx, req.UUID, req.Labels)
	if err != nil {
		newError := fmt.Errorf("failed to update user labels '%s',error:%w", req.UUID, err)
		return newError
	}

	return nil
}

func (c *userComponentImpl) FindByUUIDs(ctx context.Context, uuids []string) ([]*types.User, error) {
	usersRes := make([]*types.User, 0)

	dbUsers, err := c.userStore.FindByUUIDs(ctx, uuids)
	if err != nil {
		return usersRes, fmt.Errorf("failed find user by uuids, error:%w", err)
	}
	if len(dbUsers) == 0 {
		return usersRes, nil
	}
	for _, dbuser := range dbUsers {
		if dbuser != nil {
			usersRes = append(usersRes, &types.User{
				ID:       dbuser.ID,
				Username: dbuser.Username,
				UUID:     dbuser.UUID,
			})
		}
	}
	return usersRes, nil
}

func (c *userComponentImpl) SoftDelete(ctx context.Context, operator, username string, req types.CloseAccountReq) error {
	if operator != username {
		return fmt.Errorf("invalid request")
	}

	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to find user by name in db,error:%w", err)
	}
	before, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user before delete")
	}
	audit := &database.AuditLog{
		TableName:  "users",
		Action:     enum.AuditActionSoftDeletion,
		OperatorID: user.ID,
		Before:     string(before),
	}

	err = c.userStore.SoftDeleteUserAndRelations(ctx, user, req)
	if err != nil {
		return fmt.Errorf("failed to delete user in db,error:%w", err)
	}

	after, err := c.userStore.FindByUsernameWithDeleted(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to find user by name in db,error:%w", err)
	}
	afterBytes, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("failed to marshal user after delete")
	}
	audit.After = string(afterBytes)

	err = c.audit.Create(ctx, audit)
	if err != nil {
		return fmt.Errorf("failed to create audit log,error:%w", err)
	}

	return nil
}

func (c *userComponentImpl) GetEmails(ctx context.Context, visitorName string, per, page int) ([]string, int, error) {
	// check if user has permission to get all user emails
	canAdmin, err := c.CanAdmin(ctx, visitorName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check visitor user permission, visitor: %s, error: %w", visitorName, err)
	}
	if !canAdmin {
		return nil, 0, errorx.ErrForbiddenMsg("current user does not have permission to get all user emails")
	}

	emails, count, err := c.userStore.GetEmails(ctx, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get all user emails,error:%w", err)
	}
	return emails, count, nil
}

func (c *userComponentImpl) GetEmailsInternal(ctx context.Context, per, page int) ([]string, int, error) {
	emails, count, err := c.userStore.GetEmails(ctx, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get all user emails,error:%w", err)
	}
	return emails, count, nil
}

func (c *userComponentImpl) GetUserUUIDs(ctx context.Context, per, page int) ([]string, int, error) {
	userUUIDs, total, err := c.userStore.GetUserUUIDs(ctx, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user uuids,error:%w", err)
	}

	return userUUIDs, total, nil
}

func (c *userComponentImpl) GenerateVerificationCodeAndSendEmail(ctx context.Context, uid, email string) error {
	user, err := c.userStore.FindByUUID(ctx, uid)
	if err != nil {
		return err
	}
	if user == nil {
		return errorx.ErrUserNotFound
	}

	verificationCode, err := c.generateVerificationCode(ctx, uid, email)
	if err != nil {
		return err
	}

	err = c.sendVerificationCodeEmail(ctx, uid, email, verificationCode)
	if err != nil {
		return err
	}

	return nil
}

func (c *userComponentImpl) generateVerificationCode(ctx context.Context, uid, email string) (string, error) {
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	key := fmt.Sprintf("email_verification_code:%s:%s", uid, email)
	if err := c.cache.SetEx(ctx, key, code, time.Minute*5); err != nil {
		return "", err
	}

	return code, nil
}

func (c *userComponentImpl) sendVerificationCodeEmail(ctx context.Context, uid, email, verificationCode string) error {
	parameters := types.EmailVerifyCodeNotificationReq{
		Email: email,
		Code:  verificationCode,
		TTL:   5,
	}
	parametersBytes, err := json.Marshal(parameters)
	if err != nil {
		return err
	}
	return c.notificationSvc.Send(
		ctx,
		&types.MessageRequest{
			Scenario:   "email-verify-code",
			Parameters: string(parametersBytes),
			Priority:   "high",
		},
	)

}

func (e *userComponentImpl) VerifyVerificationCode(ctx context.Context, uid, email string, verificationCode string) error {
	exists, err := e.cache.Exists(ctx, fmt.Sprintf("email_verification_code:%s:%s", uid, email))
	if err != nil {
		return err
	}
	if exists == 0 {
		return errors.New("verification code expired or not available")
	}
	code, err := e.cache.Get(ctx, fmt.Sprintf("email_verification_code:%s:%s", uid, email))
	if err != nil {
		return err
	}
	if code != verificationCode {
		return errors.New("email verification code is invalid")
	}
	err = e.cache.Del(ctx, fmt.Sprintf("email_verification_code:%s:%s", uid, email))
	if err != nil {
		return err
	}

	return nil
}

func (e *userComponentImpl) IsSSOUser(regProvider string) bool {
	if regProvider == "" {
		return false
	}
	return regProvider == e.config.SSOType
}

func (c *userComponentImpl) ResetUserTags(ctx context.Context, uid string, tagIDs []int64) error {
	user, err := c.userStore.FindByUUID(ctx, uid)
	if err != nil {
		return err
	}
	if user == nil {
		return errorx.ErrUserNotFound
	}

	if err := c.ts.CheckTagIDsExist(ctx, tagIDs); err != nil {
		return err
	}

	if err := c.uts.ResetUserTags(ctx, user.ID, tagIDs); err != nil {
		return err
	}
	return nil
}

func (c *userComponentImpl) StreamExportUsers(ctx context.Context, req types.UserIndexReq) (data chan types.UserIndexResp, err error) {
	data = make(chan types.UserIndexResp)

	ch, err := c.userStore.IndexWithCursor(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query users by cursor",
			slog.Any("req", req),
			slog.Any("error", err),
		)
		return data, errorx.ErrInternalServerError
	}

	go func() {
		defer close(data)
		for wrapper := range ch {
			if wrapper.Err != nil {
				slog.ErrorContext(ctx, "failed to query users by cursor",
					slog.Any("req", req),
					slog.Any("error", wrapper.Err),
				)
				data <- types.UserIndexResp{Error: wrapper.Err}
				return
			}
			for _, originalUser := range wrapper.Users {
				var tags []types.RepoTag
				for _, utag := range originalUser.Tags {
					tags = append(tags, types.RepoTag{
						ID:       utag.ID,
						Name:     utag.Tag.Name,
						Category: utag.Tag.Category,
						Group:    utag.Tag.Group,
						BuiltIn:  utag.Tag.BuiltIn,
						Scope:    utag.Tag.Scope,
						I18nKey:  utag.Tag.I18nKey,
					})
				}
				exportUser := &types.User{
					Username:     originalUser.Username,
					Nickname:     originalUser.NickName,
					Avatar:       originalUser.Avatar,
					Tags:         tags,
					Email:        originalUser.Email,
					UUID:         originalUser.UUID,
					Bio:          originalUser.Bio,
					Homepage:     originalUser.Homepage,
					Phone:        originalUser.Phone,
					PhoneArea:    originalUser.PhoneArea,
					Roles:        originalUser.Roles(),
					VerifyStatus: string(originalUser.VerifyStatus),
					Labels:       originalUser.Labels,
					LastLoginAt:  originalUser.LastLoginAt,
					CreatedAt:    originalUser.CreatedAt,
				}

				select {
				case <-ctx.Done():
					slog.InfoContext(ctx, "stream export canceled while writing data", slog.String("reason", ctx.Err().Error()))
					data <- types.UserIndexResp{Error: ctx.Err()}
					return
				case data <- types.UserIndexResp{Users: []*types.User{exportUser}}:
				}
			}
		}

	}()

	slog.InfoContext(ctx, "stream export completed successfully")
	return data, nil
}
