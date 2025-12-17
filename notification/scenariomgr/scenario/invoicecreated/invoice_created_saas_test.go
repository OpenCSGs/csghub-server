//go:build saas

package invoicecreated

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"

	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
)

func TestGetEmailDataFunc_Success(t *testing.T) {
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Frontend.URL = "https://example.com"

	req := types.EmailInvoiceCreatedNotification{
		ReceiverEmail: "admin@example.com",
		UserUUID:      "test-uuid-123",
		Amount:        "100.50",
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	mockUser := &types.User{
		UUID:     "test-uuid-123",
		Username: "testuser",
		Email:    "testuser@example.com",
		Phone:    "1234567890",
	}

	mockUserSvc.EXPECT().GetUserByUUID(ctx, "test-uuid-123").Return(mockUser, nil)

	getDataFunc := GetEmailDataFunc(mockUserSvc)
	result, err := getDataFunc(ctx, cfg, msg)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Receiver)
	assert.False(t, result.Receiver.IsBroadcast)
	assert.Equal(t, "zh-CN", result.Receiver.GetLanguage())

	// Verify receiver emails
	emails := result.Receiver.GetRecipients(notifychannel.RecipientKeyUserEmails)
	assert.Len(t, emails, 1)
	assert.Equal(t, "admin@example.com", emails[0])

	// Verify payload
	assert.NotNil(t, result.Payload)
	payload := result.Payload.(map[string]any)
	assert.Equal(t, "testuser", payload["user_name"])
	assert.Equal(t, "testuser@example.com", payload["email"])
	assert.Equal(t, "1234567890", payload["phone"])
	assert.Equal(t, "100.50", payload["amount"])
	assert.Equal(t, "https://example.com/admin_panel/users/testuser", payload["user_info_url"])
}

func TestGetEmailDataFunc_InvalidJSON(t *testing.T) {
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	ctx := context.Background()
	cfg := &config.Config{}

	msg := types.ScenarioMessage{
		Parameters: "invalid json",
	}

	getDataFunc := GetEmailDataFunc(mockUserSvc)
	result, err := getDataFunc(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetEmailDataFunc_GetUserByUUIDError(t *testing.T) {
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	ctx := context.Background()
	cfg := &config.Config{}

	req := types.EmailInvoiceCreatedNotification{
		ReceiverEmail: "admin@example.com",
		UserUUID:      "test-uuid-123",
		Amount:        "100.50",
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	mockUserSvc.EXPECT().GetUserByUUID(ctx, "test-uuid-123").Return(nil, assert.AnError)

	getDataFunc := GetEmailDataFunc(mockUserSvc)
	result, err := getDataFunc(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetEmailDataFunc_UserNotFound(t *testing.T) {
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	ctx := context.Background()
	cfg := &config.Config{}

	req := types.EmailInvoiceCreatedNotification{
		ReceiverEmail: "admin@example.com",
		UserUUID:      "test-uuid-123",
		Amount:        "100.50",
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	mockUserSvc.EXPECT().GetUserByUUID(ctx, "test-uuid-123").Return(nil, nil)

	getDataFunc := GetEmailDataFunc(mockUserSvc)
	result, err := getDataFunc(ctx, cfg, msg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
	assert.Contains(t, err.Error(), "test-uuid-123")
	assert.Nil(t, result)
}

func TestGetEmailDataFunc_EmptyUserFields(t *testing.T) {
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Frontend.URL = "https://example.com"

	req := types.EmailInvoiceCreatedNotification{
		ReceiverEmail: "admin@example.com",
		UserUUID:      "test-uuid-123",
		Amount:        "100.50",
	}
	reqBytes, _ := json.Marshal(req)

	msg := types.ScenarioMessage{
		Parameters: string(reqBytes),
	}

	mockUser := &types.User{
		UUID:     "test-uuid-123",
		Username: "testuser",
		Email:    "", // Empty email
		Phone:    "", // Empty phone
	}

	mockUserSvc.EXPECT().GetUserByUUID(ctx, "test-uuid-123").Return(mockUser, nil)

	getDataFunc := GetEmailDataFunc(mockUserSvc)
	result, err := getDataFunc(ctx, cfg, msg)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	payload := result.Payload.(map[string]any)
	assert.Equal(t, "testuser", payload["user_name"])
	assert.Equal(t, "", payload["email"])
	assert.Equal(t, "", payload["phone"])
	assert.Equal(t, "100.50", payload["amount"])
}
