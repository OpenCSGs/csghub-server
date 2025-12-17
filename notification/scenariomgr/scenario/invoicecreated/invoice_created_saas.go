//go:build saas

package invoicecreated

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

func GetEmailDataFunc(userSrv rpc.UserSvcClient) scenariomgr.GetDataFunc {
	getDataFunc := func(ctx context.Context, cfg *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
		var req types.EmailInvoiceCreatedNotification
		if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
			return nil, err
		}

		receiver := &notifychannel.Receiver{
			IsBroadcast: false,
		}
		receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, []string{req.ReceiverEmail})
		receiver.SetLanguage("zh-CN")

		user, err := userSrv.GetUserByUUID(ctx, req.UserUUID)
		if err != nil {
			return nil, err
		}

		if user == nil {
			return nil, fmt.Errorf("user not found: %s", req.UserUUID)
		}

		userInfoURL := fmt.Sprintf("%s/admin_panel/users/%s", cfg.Frontend.URL, user.Username)

		return &scenariomgr.NotificationData{
			Payload: map[string]any{
				"user_name":     user.Username,
				"email":         user.Email,
				"phone":         user.Phone,
				"amount":        req.Amount,
				"user_info_url": userInfoURL,
			},
			Receiver: receiver,
		}, nil
	}
	return getDataFunc
}
