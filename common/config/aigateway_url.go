package config

import (
	"fmt"
	"strings"
)

// PublicAIGatewayURL returns the OpenAI-compatible base URL for platform AIGateway.
// Example: http://aigateway:8094/v1
func (cfg *Config) PublicAIGatewayURL() string {
	if cfg == nil {
		return "http://aigateway:8094/v1"
	}
	if url := strings.TrimSpace(cfg.AIGateway.PublicAIGatewayURL); url != "" {
		return strings.TrimSuffix(url, "/")
	}
	return fmt.Sprintf("http://aigateway:%d/v1", cfg.AIGateway.Port)
}
