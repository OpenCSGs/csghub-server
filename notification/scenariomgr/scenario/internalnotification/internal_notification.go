package internalnotification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// implement scenariomgr.GetDataFunc to get site internal message data
func GetSiteInternalMessageData(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var req types.NotificationMessage
	if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
		return nil, err
	}

	receiver := &notifychannel.Receiver{}
	if len(req.UserUUIDs) == 0 {
		receiver.IsBroadcast = true
	} else {
		receiver.IsBroadcast = false
		receiver.AddRecipients(notifychannel.RecipientKeyUserUUIDs, req.UserUUIDs)
	}

	return &scenariomgr.NotificationData{
		Message:  req,
		Receiver: receiver,
	}, nil
}

// implement scenariomgr.GetDataFunc to get internal notification email data
func NewGetInternalNotificationEmailDataFunc(storage database.NotificationStore) scenariomgr.GetDataFunc {
	getDataFunc := func(ctx context.Context, _ *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		var notificationMessage types.NotificationMessage
		if err := json.Unmarshal([]byte(msg.Parameters), &notificationMessage); err != nil {
			return nil, err
		}

		if len(notificationMessage.UserUUIDs) == 0 {
			return &scenariomgr.NotificationData{
				Message: types.EmailReq{
					Source: types.EmailSourceNotificationSetting,
				},
				Payload: getEmailPayload(notificationMessage),
				Receiver: &notifychannel.Receiver{
					IsBroadcast: true,
				},
			}, nil
		}

		settings, err := storage.GetSettingByUserUUIDs(ctx, notificationMessage.UserUUIDs)
		if err != nil {
			return nil, err
		}
		var userEmails []string
		for _, setting := range settings {
			if setting.IsEmailNotificationEnabled && setting.EmailAddress != "" {
				userEmails = append(userEmails, setting.EmailAddress)
			}
		}

		receiver := &notifychannel.Receiver{
			IsBroadcast: false,
		}
		receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, userEmails)

		return &scenariomgr.NotificationData{
			Payload:  getEmailPayload(notificationMessage),
			Receiver: receiver,
		}, nil
	}
	return getDataFunc
}

func getEmailPayload(notificationMessage types.NotificationMessage) map[string]any {
	return map[string]any{
		"title":   notificationMessage.Title,
		"summary": notificationMessage.Summary,
		"content": notificationMessage.Content,
	}
}

func GetEmailDataFunc(storage database.NotificationStore) scenariomgr.GetDataFunc {
	getDataFunc := func(ctx context.Context, _ *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		var req types.NotificationMessage
		if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
			return nil, err
		}

		settings, err := storage.GetSettingByUserUUIDs(ctx, req.UserUUIDs)
		if err != nil {
			return nil, err
		}
		var userEmails []string
		for _, setting := range settings {
			if setting.IsEmailNotificationEnabled && setting.EmailAddress != "" {
				userEmails = append(userEmails, setting.EmailAddress)
			}
		}

		if len(userEmails) == 0 {
			return nil, fmt.Errorf("no email found in notification setting for users %s", strings.Join(req.UserUUIDs, ","))
		}

		receiver := &notifychannel.Receiver{
			IsBroadcast: false,
		}
		receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, userEmails)

		return &scenariomgr.NotificationData{
			Payload:  req.Payload,
			Receiver: receiver,
		}, nil
	}
	return getDataFunc
}
