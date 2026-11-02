package smsverifycode

import (
	"context"
	"encoding/json"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// implement scenariomgr.GetDataFunc to get sms data
func GetSMSData(ctx context.Context, conf *config.Config, msg types.ScenarioMessage) (*scenariomgr.NotificationData, error) {
	var req types.SMSReq
	if err := json.Unmarshal([]byte(msg.Parameters), &req); err != nil {
		return nil, err
	}

	receiver := &notifychannel.Receiver{
		IsBroadcast: false,
	}
	receiver.AddRecipients(notifychannel.RecipientKeyUserPhoneNumbers, req.PhoneNumbers)

	return &scenariomgr.NotificationData{
		Message:  req,
		Receiver: receiver,
	}, nil
}
