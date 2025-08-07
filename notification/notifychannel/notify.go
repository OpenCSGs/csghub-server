package notifychannel

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type NotifyRequest struct {
	// template variables OR direct message object
	Message any
	// payload data for template rendering
	Payload any
	// template formated output
	FormattedData *types.TemplateOutput
	// target recipients
	Receiver *Receiver
	// message priority
	Priority types.MessagePriority
}

type Notifier interface {
	Send(ctx context.Context, req *NotifyRequest) error
	IsFormatRequired() bool
}
