package internalnotification

import (
	"context"
	"encoding/json"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// implement scenariomgr.GetDataFunc to get site internal message data
func GetSiteInternalMessageData(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var notificationMessage types.NotificationMessage
	if err := json.Unmarshal([]byte(msg.Parameters), &notificationMessage); err != nil {
		return nil, err
	}

	var receiver *notifychannel.Receiver
	if len(notificationMessage.UserUUIDs) == 0 {
		receiver = &notifychannel.Receiver{
			IsBroadcast: true,
		}
	} else {
		receiver = &notifychannel.Receiver{
			IsBroadcast: false,
		}
		receiver.AddRecipients(notifychannel.RecipientKeyUserUUIDs, notificationMessage.UserUUIDs)
	}

	return &scenariomgr.NotificationData{
		MessageData: notificationMessage,
		Receiver:    receiver,
	}, nil
}

// implement scenariomgr.GetDataFunc to get internal notification email data
func NewGetInternalNotificationEmailDataFunc(storage database.NotificationStore) func(ctx context.Context, _ *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	getDataFunc := func(ctx context.Context, _ *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		var notificationMessage types.NotificationMessage
		if err := json.Unmarshal([]byte(msg.Parameters), &notificationMessage); err != nil {
			return nil, err
		}

		notificationEmailContent := types.NotificationEmailContent{
			Subject:     notificationMessage.Title,
			ContentType: types.ContentTypeTextHTML,
			Source:      types.EmailSourceNotificationSetting,
		}

		if len(notificationMessage.UserUUIDs) == 0 {
			return &scenariomgr.NotificationData{
				MessageData: notificationMessage,

				Receiver: &notifychannel.Receiver{
					IsBroadcast: true,
					Metadata: map[string]any{
						"content": notificationEmailContent,
					},
				},
			}, nil
		}

		receiver := &notifychannel.Receiver{
			IsBroadcast: false,
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

		receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, userEmails)
		receiver.SetMetadata("content", notificationEmailContent)

		return &scenariomgr.NotificationData{
			MessageData: notificationMessage,
			Receiver:    receiver,
		}, nil
	}
	return getDataFunc
}
