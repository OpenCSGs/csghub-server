package resourceapplication

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
	"opencsg.com/csghub-server/notification/utils"
)

func GetInternalMessageDataFunc(userSvc rpc.UserSvcClient) scenariomgr.GetDataFunc {
	return func(ctx context.Context, _ *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		req, err := parseRequest(msg)
		if err != nil {
			return nil, err
		}

		adminUUIDs, err := getAdminUUIDs(ctx, userSvc)
		if err != nil {
			return nil, err
		}

		message := buildInternalNotificationMessage(req, adminUUIDs)
		receiver := &notifychannel.Receiver{IsBroadcast: false}
		receiver.AddRecipients(notifychannel.RecipientKeyUserUUIDs, adminUUIDs)

		return &scenariomgr.NotificationData{
			Message:  message,
			Payload:  message.Payload,
			Receiver: receiver,
		}, nil
	}
}

func GetEmailDataFunc(userSvc rpc.UserSvcClient) scenariomgr.GetDataFunc {
	return func(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		req, err := parseRequest(msg)
		if err != nil {
			return nil, err
		}

		adminEmails, err := getAdminEmails(ctx, userSvc)
		if err != nil {
			return nil, err
		}

		receiver := &notifychannel.Receiver{IsBroadcast: false}
		receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, adminEmails)
		receiver.SetLanguage("zh-CN")

		return &scenariomgr.NotificationData{
			Message: types.EmailReq{
				Source: types.EmailSourceUser,
			},
			Payload:  buildEmailPayload(req, conf),
			Receiver: receiver,
		}, nil
	}
}

func parseRequest(msg types.ScenarioMessage) (types.ResourceApplicationNotificationReq, error) {
	var req types.ResourceApplicationNotificationReq
	if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
		return req, err
	}
	if req.UserUUID == "" {
		return req, fmt.Errorf("user_uuid is required")
	}
	if req.ResourceSKU == "" {
		return req, fmt.Errorf("resource_sku is required")
	}
	return req, nil
}

func getAdminUUIDs(ctx context.Context, userSvc rpc.UserSvcClient) ([]string, error) {
	adminUUIDs, _, err := userSvc.GetAdminUserUUIDs(ctx)
	if err != nil {
		return nil, utils.NewErrSendMsg(err, "failed to get admin user uuids")
	}
	if len(adminUUIDs) == 0 {
		return nil, fmt.Errorf("no admin users found to receive resource application")
	}
	return adminUUIDs, nil
}

func getAdminEmails(ctx context.Context, userSvc rpc.UserSvcClient) ([]string, error) {
	adminEmails, _, err := userSvc.GetAdminEmails(ctx)
	if err != nil {
		return nil, utils.NewErrSendMsg(err, "failed to get admin emails")
	}
	if len(adminEmails) == 0 {
		return nil, fmt.Errorf("no admin emails found to receive resource application")
	}
	return adminEmails, nil
}

func buildInternalNotificationMessage(req types.ResourceApplicationNotificationReq, adminUUIDs []string) types.NotificationMessage {
	return types.NotificationMessage{
		UserUUIDs:        adminUUIDs,
		NotificationType: types.NotificationSystem,
		Template:         string(types.MessageScenarioResourceApplication),
		Payload:          buildPayload(req),
	}
}

func buildEmailPayload(req types.ResourceApplicationNotificationReq, conf *config.Config) map[string]any {
	userName := req.UserName
	if userName == "" {
		userName = req.UserUUID
	}

	portalURL := ""
	if conf != nil {
		portalURL = conf.Frontend.URL
	}

	return map[string]any{
		"user_uuid":    html.EscapeString(req.UserUUID),
		"user_name":    html.EscapeString(userName),
		"resource_sku": html.EscapeString(req.ResourceSKU),
		"portal_url":   portalURL,
		"is_staging":   strings.Contains(portalURL, "-stg."),
	}
}

func buildPayload(req types.ResourceApplicationNotificationReq) map[string]any {
	userName := req.UserName
	if userName == "" {
		userName = req.UserUUID
	}

	return map[string]any{
		"user_uuid":    req.UserUUID,
		"user_name":    userName,
		"resource_sku": req.ResourceSKU,
	}
}
