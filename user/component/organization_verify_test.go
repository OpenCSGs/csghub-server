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

func TestOrganizationVerifyComponent_Create(t *testing.T) {
	req := &types.OrgVerifyReq{
		Name:               "org_1",
		CompanyName:        "Test Company",
		UnifiedCreditCode:  "123456789",
		Username:           "testuser",
		ContactName:        "Test Contact",
		ContactEmail:       "test@example.com",
		BusinessLicenseImg: "license.png",
	}

	mockVerifyStore := mockdb.NewMockOrganizationVerifyStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)

	expected := &database.OrganizationVerify{
		Name:               req.Name,
		CompanyName:        req.CompanyName,
		UnifiedCreditCode:  req.UnifiedCreditCode,
		Username:           req.Username,
		ContactName:        req.ContactName,
		ContactEmail:       req.ContactEmail,
		BusinessLicenseImg: req.BusinessLicenseImg,
		Status:             types.VerifyStatusPending,
		Reason:             "",
	}

	mockVerifyStore.EXPECT().
		CreateOrganizationVerify(mock.Anything, mock.AnythingOfType("*database.OrganizationVerify")).
		Return(expected, nil).Once()

	mockOrgStore.EXPECT().
		UpdateVerifyStatus(mock.Anything, req.Name, types.VerifyStatusPending).
		Return(nil).Once()

	c := &OrganizationVerifyComponentImpl{
		orgVerifyStore: mockVerifyStore,
		orgStore:       mockOrgStore,
	}

	orgVerify, err := c.Create(context.Background(), req)
	require.NoError(t, err)
	require.EqualValues(t, expected, orgVerify)
}

func TestOrganizationVerifyComponent_Update(t *testing.T) {
	mockVerifyStore := mockdb.NewMockOrganizationVerifyStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)

	config := &config.Config{}
	config.Notification.NotificationRetryCount = 3
	config.Notification.Host = "localhost"
	config.Notification.Port = 8095
	config.APIToken = "test-api-token"

	expected := &database.OrganizationVerify{
		ID:       1,
		Name:     "org_1",
		Status:   types.VerifyStatusApproved,
		Reason:   "Verified",
		UserUUID: "user-123",
	}

	mockVerifyStore.EXPECT().
		UpdateOrganizationVerify(mock.Anything, int64(1), types.VerifyStatusApproved, "Verified").
		Return(expected, nil).Once()

	mockOrgStore.EXPECT().
		UpdateVerifyStatus(mock.Anything, expected.Name, types.VerifyStatusApproved).
		Return(nil).Once()

	var wg sync.WaitGroup
	wg.Add(1)
	mockNotificationRpc.EXPECT().
		Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			// Check the basic structure
			if req.Scenario != types.MessageScenarioOrgVerify || req.Priority != types.MessagePriorityHigh {
				return false
			}

			// Parse the parameters to check the content
			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			// Check the important fields (ignore UUID and timestamp)
			res := msg.UserUUIDs[0] == expected.UserUUID &&
				msg.NotificationType == types.NotificationSystem &&
				msg.Template == string(types.MessageScenarioOrgVerify)
			return res
		})).
		Return(nil).Once()

	c := &OrganizationVerifyComponentImpl{
		orgVerifyStore:        mockVerifyStore,
		orgStore:              mockOrgStore,
		notificationSvcClient: mockNotificationRpc,
		config:                config,
	}

	result, err := c.Update(context.Background(), 1, types.VerifyStatusApproved, "Verified")
	require.NoError(t, err)
	require.EqualValues(t, expected, result)

	wg.Wait()
}

func TestOrganizationVerifyComponent_Get(t *testing.T) {
	mockVerifyStore := mockdb.NewMockOrganizationVerifyStore(t)
	expected := &database.OrganizationVerify{
		ID:                1,
		Name:              "org_1",
		CompanyName:       "Test Co",
		UnifiedCreditCode: "ABC123456",
	}

	mockVerifyStore.EXPECT().
		GetOrganizationVerify(mock.Anything, expected.Name).
		Return(expected, nil).Once()

	c := &OrganizationVerifyComponentImpl{
		orgVerifyStore: mockVerifyStore,
	}

	orgVerify, err := c.Get(context.Background(), "org_1")
	require.NoError(t, err)
	require.EqualValues(t, expected, orgVerify)
}
