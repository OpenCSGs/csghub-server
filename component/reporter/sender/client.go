package sender

import (
	"context"
	"time"

	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/common/types"
)

// LogSender is the interface for sending logs to a backend
type LogSender interface {
	// SendLogs sends a batch of log entries
	SendLogs(ctx context.Context, entries []types.LogEntry) error
	// Health checks the health of the log sending backend
	Health(ctx context.Context) error
	// GetLastReportedTimestamp gets the timestamp of the last successfully sent log
	GetLastReportedTimestamp(ctx context.Context) (time.Time, error)
	// StreamAllLogs streams all logs from the backend
	StreamAllLogs(ctx context.Context, id string, start time.Time, lables map[string]string, timeLoc *time.Location) (chan string, error)
	QueryRange(ctx context.Context, params loki.QueryRangeParams) (*loki.LokiQueryResponse, error)
}
