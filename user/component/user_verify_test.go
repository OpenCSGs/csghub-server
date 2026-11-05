package component

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestUserVerifyComponent_Create(t *testing.T) {
	req := &types.UserVerifyReq{
		UUID:        "uuid_1",
		RealName:    "John Doe",
		Username:    "johndoe",
		IDCardFront: "idcard-front.png",
		IDCardBack:  "idcard-back.png",
	}

	mockUserVerifyStore := mockdb.NewMockUserVerifyStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)

	// SaaS resolves the authenticated owner; the other editions retain their existing behavior.
	mockUserStore.EXPECT().FindByUUID(mock.Anything, req.UUID).
		Return(&database.User{UUID: req.UUID, Username: req.Username}, nil).Maybe()

	expected := &database.UserVerify{
		UUID:        req.UUID,
		RealName:    req.RealName,
		Username:    req.Username,
		IDCardFront: req.IDCardFront,
		IDCardBack:  req.IDCardBack,
		Status:      "pending",
	}

	mockUserVerifyStore.EXPECT().
		CreateUserVerify(mock.Anything, mock.AnythingOfType("*database.UserVerify")).
		Return(expected, nil).Once()

	mockUserStore.EXPECT().
		UpdateVerifyStatus(mock.Anything, req.UUID, types.VerifyStatusPending).
		Return(nil).Once()

	c := &UserVerifyComponentImpl{
		userVerifyStore: mockUserVerifyStore,
		userStore:       mockUserStore,
	}

	ctx := context.WithValue(context.Background(), "currentUserUUID", req.UUID)
	result, err := c.Create(ctx, req)
	require.NoError(t, err)
	require.EqualValues(t, expected, result)
}

// TestUserVerifyComponent_Update dispatches notification after the atomic store operation succeeds.
func TestUserVerifyComponent_Update(t *testing.T) {
	mockUserVerifyStore := mockdb.NewMockUserVerifyStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)

	config := &config.Config{}
	config.Notification.NotificationRetryCount = 3

	expected := &database.UserVerify{
		ID:     1,
		UUID:   "uuid_1",
		Status: "approved",
		Reason: "Valid",
	}

	mockUserVerifyStore.EXPECT().
		UpdateUserVerify(mock.Anything, int64(1), types.VerifyStatusApproved, "Valid").
		Return(expected, nil).Once()

	var wg sync.WaitGroup
	wg.Add(1)
	mockNotificationRpc.EXPECT().
		Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			// Check the basic structure
			if req.Scenario != types.MessageScenarioUserVerify || req.Priority != types.MessagePriorityHigh {
				return false
			}

			// Parse the parameters to check the content
			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			// Check the important fields (ignore UUID and timestamp)
			return msg.UserUUIDs[0] == expected.UUID &&
				msg.NotificationType == types.NotificationSystem &&
				msg.Template == string(types.MessageScenarioUserVerify)
		})).
		Return(nil).Once()

	c := &UserVerifyComponentImpl{
		userVerifyStore:       mockUserVerifyStore,
		userStore:             mockUserStore,
		notificationSvcClient: mockNotificationRpc,
		config:                config,
	}

	result, err := c.Update(context.Background(), 1, types.VerifyStatusApproved, "Valid")
	require.NoError(t, err)
	require.EqualValues(t, expected, result)

	wg.Wait()
}

// TestUserVerifyComponent_Get reads the authenticated owner's verification across editions.
func TestUserVerifyComponent_Get(t *testing.T) {
	mockUserVerifyStore := mockdb.NewMockUserVerifyStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)

	expected := &database.UserVerify{
		ID:       1,
		UUID:     "uuid_1",
		RealName: "John Doe",
		Username: "johndoe",
		Status:   "pending",
	}

	mockUserVerifyStore.EXPECT().
		GetUserVerify(mock.Anything, expected.UUID).
		Return(expected, nil).Once()
	// Only SaaS resolves the owner before reading verification data.
	mockUserStore.EXPECT().FindByUUID(mock.Anything, expected.UUID).
		Return(&database.User{UUID: expected.UUID, Username: expected.Username}, nil).Maybe()

	c := &UserVerifyComponentImpl{
		userVerifyStore: mockUserVerifyStore,
		userStore:       mockUserStore,
	}

	ctx := context.WithValue(context.Background(), "currentUserUUID", expected.UUID)
	result, err := c.Get(ctx, "uuid_1")
	require.NoError(t, err)
	require.EqualValues(t, expected, result)
}
