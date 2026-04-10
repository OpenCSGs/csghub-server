//go:build !ee && !saas

package factory

import (
	"testing"

	"opencsg.com/csghub-server/common/config"
)

func TestDefaultSMSFactory_CreateSMSClient(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{
			name:     "Create Aliyun client",
			provider: "aliyun",
			wantErr:  false, // May error with incomplete config, but factory will create
		},
		{
			name:     "Create Tencent Cloud client",
			provider: "tencent",
			wantErr:  false, // May error with incomplete config, but factory will create
		},
		{
			name:     "Create Huawei Cloud client",
			provider: "huawei",
			wantErr:  false, // May error with incomplete config, but factory will create
		},
		{
			name:     "Unknown provider falls back to Aliyun",
			provider: "unknown",
			wantErr:  false, // Will fall back to Aliyun
		},
	}

	factory := NewDefaultSMSFactory()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Notification.SMSProvider = tt.provider

			// Set some test configurations
			cfg.Notification.SMSAccessKeyID = "test-ak"
			cfg.Notification.SMSAccessKeySecret = "test-sk"
			cfg.Notification.SMSAppID = "test-tencent-id"
			cfg.Notification.SMSEndpoint = "test-tencent-endpoint"
			cfg.Notification.SMSRegion = "test-region"

			client, err := factory.CreateSMSClient(cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSMSClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if client == nil && !tt.wantErr {
				t.Error("CreateSMSClient() returned nil client but no error")
			}
		})
	}
}

func TestProviderTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{"Aliyun", ProviderAliyun, "aliyun"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.provider) != tt.expected {
				t.Errorf("ProviderType %v = %v, want %v", tt.name, tt.provider, tt.expected)
			}
		})
	}
}
