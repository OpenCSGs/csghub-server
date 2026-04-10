//go:build !ee && !saas

package factory

import (
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/notification/notifychannel/channel/sms/client"
)

// ProviderType SMS provider type
type ProviderType string

const (
	ProviderAliyun ProviderType = "aliyun"
)

type SMSFactory interface {
	CreateSMSClient(config *config.Config) (client.SMSService, error)
}

type DefaultSMSFactory struct{}

func NewDefaultSMSFactory() SMSFactory {
	return &DefaultSMSFactory{}
}

func (f *DefaultSMSFactory) CreateSMSClient(config *config.Config) (client.SMSService, error) {
	provider := ProviderType(config.Notification.SMSProvider)

	switch provider {
	case ProviderAliyun:
		return createAliyunSMSClient(config)
	default:
		slog.Warn("Unknown SMS provider, using default Aliyun", slog.String("provider", string(provider)))
		return createAliyunSMSClient(config)
	}
}

// createAliyunSMSClient creates Aliyun SMS client
func createAliyunSMSClient(config *config.Config) (client.SMSService, error) {
	// Call the existing NewAliyunSMSClient function
	// Note: Need to update the existing NewAliyunSMSClient function to use new config fields
	return client.NewAliyunSMSClient(config)
}
