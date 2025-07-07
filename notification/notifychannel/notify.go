package notifychannel

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type NotifyRequest struct {
	// template variables OR direct message object
	MessageData any
	// template formated output
	Payload string
	// target recipients
	Receiver *Receiver
	// message priority
	Priority types.MessagePriority
}

type Notifier interface {
	Send(ctx context.Context, req *NotifyRequest) error
	IsFormatRequired() bool
}
