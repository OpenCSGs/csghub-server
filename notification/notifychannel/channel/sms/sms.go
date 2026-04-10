package sms

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/notifychannel"
	"opencsg.com/csghub-server/notification/notifychannel/channel/sms/client"
	"opencsg.com/csghub-server/notification/utils"
)

type SMSChannel struct {
	smsService client.SMSService
}

func NewSMSChannel(smsService client.SMSService) notifychannel.Notifier {
	return &SMSChannel{
		smsService: smsService,
	}
}

var _ notifychannel.Notifier = (*SMSChannel)(nil)

func (s *SMSChannel) IsFormatRequired() bool {
	return false
}

func (s *SMSChannel) Send(ctx context.Context, req *notifychannel.NotifyRequest) error {
	if err := req.Receiver.Validate(); err != nil {
		return fmt.Errorf("invalid receiver: %w", err)
	}

	var smsReq types.SMSReq
	if req.Message != nil {
		if extractedSMSReq, ok := req.Message.(types.SMSReq); ok {
			smsReq = extractedSMSReq
		} else {
			slog.Error("invalid sms message format", "message type", fmt.Sprintf("%T", req.Message))
			return fmt.Errorf("invalid sms message format")
		}
	}

	if err := s.smsService.Send(smsReq); err != nil {
		return utils.NewErrSendMsg(err, "failed to send sms") // should not print the message, it contains sensitive information
	}

	return nil
}
