package types

import (
	"time"

	"k8s.io/client-go/kubernetes"
)

// LogCollectorConfig defines the configuration for the log collector
type LogCollectorConfig struct {
	// Kubernetes client
	Client kubernetes.Interface

	// Loki endpoint URL
	LokiURL string

	// Maximum number of concurrent pod log streams
	MaxConcurrentStreams int

	// Buffer size for log entries before sending to Loki
	BatchSize int

	// Maximum time to wait before sending a batch
	BatchDelay time.Duration

	// Maximum time to wait before drop msg after chan blocked
	DropMsgTimeout time.Duration

	// Retry configuration
	MaxRetries    int
	RetryInterval time.Duration

	// Rate limiting
	RateLimit int // logs per second per pod

	// Health check interval
	HealthCheckInterval time.Duration

	// Log level filter (DEBUG, INFO, WARN, ERROR)
	LogLevel string
}

// CollectorStats provides statistics about the collector
type CollectorStats struct {
	ActiveStreams      int                       `json:"active_streams"`
	TotalLogsCollected int64                     `json:"total_logs_collected"`
	TotalLogsSent      int64                     `json:"total_logs_sent"`
	ErrorCount         int64                     `json:"error_count"`
	NamespaceStats     map[string]NamespaceStats `json:"namespace_stats"`
	LastUpdate         time.Time                 `json:"last_update"`
}

// NamespaceStats provides per-namespace statistics
type NamespaceStats struct {
	PodCount      int   `json:"pod_count"`
	ActiveStreams int   `json:"active_streams"`
	LogsCollected int64 `json:"logs_collected"`
	LogsSent      int64 `json:"logs_sent"`
	ErrorCount    int64 `json:"error_count"`
}
