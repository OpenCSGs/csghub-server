package rpc

import (
	"context"
	"fmt"
	"time"

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

func NewNotificationSvcHttpClientBuilder(endpoint string, opts ...RequestOption) NotificationSvcClientBuilder {
	return &NotificationSvcHttpClient{
		hc: NewHttpClient(endpoint, opts...),
	}
}

type NotificationSvcClientBuilder interface {
	WithRetry(attempts uint) NotificationSvcClientBuilder
	WithDelay(delay time.Duration) NotificationSvcClientBuilder
	Build() NotificationSvcClient
}

func (c *NotificationSvcHttpClient) WithRetry(attempts uint) NotificationSvcClientBuilder {
	c.hc = c.hc.WithRetry(attempts)
	return c
}

func (c *NotificationSvcHttpClient) WithDelay(delay time.Duration) NotificationSvcClientBuilder {
	c.hc = c.hc.WithDelay(delay)
	return c
}

func (c *NotificationSvcHttpClient) Build() NotificationSvcClient {
	return c
}
