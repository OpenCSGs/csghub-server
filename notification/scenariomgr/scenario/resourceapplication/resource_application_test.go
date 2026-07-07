package resourceapplication

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/tmplmgr"
	"opencsg.com/csghub-server/notification/utils"
)

func TestGetInternalMessageDataFuncSendsToAdmins(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	userSvc.EXPECT().GetAdminUserUUIDs(ctx).Return([]string{"admin-1", "admin-2"}, 2, nil)

	data, err := GetInternalMessageDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","user_name":"alice","resource_sku":"A100"}`,
	})

	require.NoError(t, err)
	require.False(t, data.Receiver.IsBroadcast)
	require.Equal(t, []string{"admin-1", "admin-2"}, data.Receiver.GetRecipients(notifychannel.RecipientKeyUserUUIDs))

	message, ok := data.Message.(types.NotificationMessage)
	require.True(t, ok)
	require.Equal(t, []string{"admin-1", "admin-2"}, message.UserUUIDs)
	require.Equal(t, types.NotificationSystem, message.NotificationType)
	require.Equal(t, string(types.MessageScenarioResourceApplication), message.Template)
	require.Empty(t, message.Title)
	require.Empty(t, message.Content)
	require.Equal(t, map[string]any{
		"user_uuid":    "user-1",
		"user_name":    "alice",
		"resource_sku": "A100",
	}, message.Payload)
}

func TestGetInternalMessageDataFuncRequiresUserUUID(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)

	_, err := GetInternalMessageDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_id":"user-1","resource_sku":"A100"}`,
	})

	require.ErrorContains(t, err, "user_uuid is required")
}

func TestGetInternalMessageDataFuncRequiresAdminRecipients(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	userSvc.EXPECT().GetAdminUserUUIDs(ctx).Return([]string{}, 0, nil)

	_, err := GetInternalMessageDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","resource_sku":"A100"}`,
	})

	require.ErrorContains(t, err, "no admin users found")
	require.False(t, utils.IsErrSendMsg(err))
}

func TestGetInternalMessageDataFuncReturnsUserServiceError(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	userSvc.EXPECT().GetAdminUserUUIDs(ctx).Return(nil, 0, errors.New("user service unavailable"))

	_, err := GetInternalMessageDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","resource_sku":"A100"}`,
	})

	require.ErrorContains(t, err, "user service unavailable")
	require.True(t, utils.IsErrSendMsg(err))
}

func TestGetInternalMessageDataFuncFallbacksUserNameToUserUUID(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	userSvc.EXPECT().GetAdminUserUUIDs(ctx).Return([]string{"admin-1"}, 1, nil)

	data, err := GetInternalMessageDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","resource_sku":"A100"}`,
	})

	require.NoError(t, err)
	message, ok := data.Message.(types.NotificationMessage)
	require.True(t, ok)
	require.Equal(t, "user-1", message.Payload["user_name"])
}

func TestGetEmailDataFuncSendsToAdminEmails(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	adminEmails := []string{"admin1@example.com", "admin2@example.com"}
	conf := &config.Config{}
	conf.Frontend.URL = "https://portal-stg.opencsg.com"

	userSvc.EXPECT().GetAdminEmails(ctx).Return(adminEmails, 2, nil)

	data, err := GetEmailDataFunc(userSvc)(ctx, conf, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","user_name":"alice","resource_sku":"A100"}`,
	})

	require.NoError(t, err)
	require.False(t, data.Receiver.IsBroadcast)
	require.Equal(t, "zh-CN", data.Receiver.GetLanguage())
	require.Equal(t, adminEmails, data.Receiver.GetRecipients(notifychannel.RecipientKeyUserEmails))

	emailReq, ok := data.Message.(types.EmailReq)
	require.True(t, ok)
	require.Equal(t, types.EmailSourceUser, emailReq.Source)

	payload, ok := data.Payload.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "alice", payload["user_name"])
	require.Equal(t, "user-1", payload["user_uuid"])
	require.Equal(t, "A100", payload["resource_sku"])
	require.Equal(t, "https://portal-stg.opencsg.com", payload["portal_url"])
	require.Equal(t, true, payload["is_staging"])

	formatted, err := tmplmgr.NewTemplateManager().Format(
		types.MessageScenarioResourceApplication,
		types.MessageChannelEmail,
		payload,
		data.Receiver.GetLanguage(),
	)
	require.NoError(t, err)
	require.Equal(t, "资源申请 [staging]", formatted.Title)
	require.Contains(t, formatted.Content, "alice")
	require.Contains(t, formatted.Content, "user-1")
	require.Contains(t, formatted.Content, "A100")
	require.Contains(t, formatted.Content, "https://portal-stg.opencsg.com")
}

func TestGetEmailDataFuncRequiresAdminEmails(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)

	userSvc.EXPECT().GetAdminEmails(ctx).Return([]string{}, 0, nil)

	_, err := GetEmailDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","resource_sku":"A100"}`,
	})

	require.ErrorContains(t, err, "no admin emails found")
	require.False(t, utils.IsErrSendMsg(err))
}

func TestGetEmailDataFuncReturnsAdminEmailsError(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)

	userSvc.EXPECT().GetAdminEmails(ctx).Return(nil, 0, errors.New("user service unavailable"))

	_, err := GetEmailDataFunc(userSvc)(ctx, nil, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","resource_sku":"A100"}`,
	})

	require.ErrorContains(t, err, "user service unavailable")
	require.True(t, utils.IsErrSendMsg(err))
}

func TestGetEmailDataFuncEscapesHTMLPayload(t *testing.T) {
	ctx := context.Background()
	userSvc := mockrpc.NewMockUserSvcClient(t)
	userSvc.EXPECT().GetAdminEmails(ctx).Return([]string{"admin@example.com"}, 1, nil)

	data, err := GetEmailDataFunc(userSvc)(ctx, &config.Config{}, types.ScenarioMessage{
		Scenario:   types.MessageScenarioResourceApplication,
		Parameters: `{"user_uuid":"user-1","user_name":"<b>attacker</b>","resource_sku":"<img src=x>"}`,
	})

	require.NoError(t, err)
	payload, ok := data.Payload.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "&lt;b&gt;attacker&lt;/b&gt;", payload["user_name"])
	require.Equal(t, "&lt;img src=x&gt;", payload["resource_sku"])

	formatted, err := tmplmgr.NewTemplateManager().Format(
		types.MessageScenarioResourceApplication,
		types.MessageChannelEmail,
		payload,
		data.Receiver.GetLanguage(),
	)
	require.NoError(t, err)
	require.NotContains(t, formatted.Content, "<b>attacker</b>")
	require.NotContains(t, formatted.Content, "<img src=x>")
}
