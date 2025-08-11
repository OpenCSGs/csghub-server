package emailverifycode

import (
	"context"
	"encoding/json"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// implement scenariomgr.GetDataFunc to get email data
func GetEmailData(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var req types.EmailVerifyCodeNotificationReq
	if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
		return nil, err
	}

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyUserEmails, []string{req.Email})

	return &scenariomgr.NotificationData{
		Payload: map[string]any{
			"code": req.Code,
			"ttl":  req.TTL,
		},
		Receiver: receiver,
	}, nil
}
