package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// AIGatewayMetricsCheckpoint stores the resume position for the aigateway
// metrics collector at minute granularity.  Unlike CronCheckpoint (which uses
// a SQL date column suited for daily jobs), this table uses a timestamp column
// so the exact minute is preserved across restarts.
type AIGatewayMetricsCheckpoint struct {
	bun.BaseModel `bun:"table:aigateway_metrics_checkpoints,alias:amc"`
	JobName       string    `bun:"job_name,pk" json:"job_name"`
	LastMinute    time.Time `bun:"last_minute,type:timestamp with time zone,notnull" json:"last_minute"`
	times
}

type aigatewayMetricsCheckpointStoreImpl struct {
	db *DB
}

type AIGatewayMetricsCheckpointStore interface {
	// GetLastMinute returns the last fully-processed minute for the given job,
	// or the zero time if no checkpoint exists yet (first run).
	GetLastMinute(ctx context.Context, jobName string) (time.Time, error)
	// SaveLastMinute upserts the checkpoint for the given job to minute.
	SaveLastMinute(ctx context.Context, jobName string, minute time.Time) error
}

func NewAIGatewayMetricsCheckpointStore() AIGatewayMetricsCheckpointStore {
	return &aigatewayMetricsCheckpointStoreImpl{
		db: defaultDB,
	}
}

func NewAIGatewayMetricsCheckpointStoreWithDB(db *DB) AIGatewayMetricsCheckpointStore {
	return &aigatewayMetricsCheckpointStoreImpl{
		db: db,
	}
}

func (s *aigatewayMetricsCheckpointStoreImpl) GetLastMinute(ctx context.Context, jobName string) (time.Time, error) {
	var cp AIGatewayMetricsCheckpoint
	err := s.db.Core.NewSelect().Model(&cp).Where("job_name = ?", jobName).Scan(ctx, &cp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return cp.LastMinute, nil
}

func (s *aigatewayMetricsCheckpointStoreImpl) SaveLastMinute(ctx context.Context, jobName string, minute time.Time) error {
	cp := AIGatewayMetricsCheckpoint{
		JobName:    jobName,
		LastMinute: minute,
	}
	_, err := s.db.Core.NewInsert().Model(&cp).
		On("CONFLICT (job_name) DO UPDATE").
		Set("last_minute = EXCLUDED.last_minute").
		Set("updated_at = current_timestamp").
		Exec(ctx)
	return err
}
