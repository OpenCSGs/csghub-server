package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_PublicAIGatewayURL(t *testing.T) {
	t.Run("explicit config", func(t *testing.T) {
		cfg := &Config{}
		cfg.AIGateway.PublicAIGatewayURL = "https://gateway.example.com/v1/"
		require.Equal(t, "https://gateway.example.com/v1", cfg.PublicAIGatewayURL())
	})

	t.Run("default internal service", func(t *testing.T) {
		cfg := &Config{}
		cfg.AIGateway.Port = 8094
		require.Equal(t, "http://aigateway:8094/v1", cfg.PublicAIGatewayURL())
	})
}
