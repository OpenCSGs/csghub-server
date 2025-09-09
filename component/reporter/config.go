package reporter

import (
	"opencsg.com/csghub-server/common/config"
	"time"

	ltypes "opencsg.com/csghub-server/logcollector/types"
)

// Config returns a default configuration
func Config(config *config.Config) *ltypes.LogCollectorConfig {
	return &ltypes.LogCollectorConfig{
		LokiURL:              config.LogCollector.LokiURL,
		MaxConcurrentStreams: config.LogCollector.MaxConcurrentStreams,
		BatchSize:            config.LogCollector.BatchSize,
		BatchDelay:           time.Duration(config.LogCollector.BatchDelay) * time.Second,
		DropMsgTimeout:       time.Duration(config.LogCollector.DropMsgTimeout) * time.Second,
		MaxRetries:           config.LogCollector.MaxRetries,
		RetryInterval:        time.Duration(config.LogCollector.RetryInterval) * time.Second,
		HealthCheckInterval:  time.Duration(config.LogCollector.HeathInterval) * time.Second,
	}
}
