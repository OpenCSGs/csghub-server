package rpc

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

type NotificationSvcClient interface {
	Send(ctx context.Context, message *types.MessageRequest) error
}

type NotificationSvcHttpClient struct {
	hc *HttpClient
}

func NewNotificationSvcHttpClient(endpoint string, opts ...RequestOption) NotificationSvcClient {
	return &NotificationSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

func (c *NotificationSvcHttpClient) Send(ctx context.Context, message *types.MessageRequest) error {
	url := "/api/v1/notifications"
	var r httpbase.R
	err := c.hc.Post(ctx, url, message, &r)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}
