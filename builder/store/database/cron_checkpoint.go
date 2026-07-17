package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CronCheckpoint stores the resume position for a periodic job (the last
// fully-processed day). One row per job_name.
type CronCheckpoint struct {
	JobName  string    `bun:"job_name,pk" json:"job_name"`
	LastDate time.Time `bun:"type:date,notnull" json:"last_date"`
	times
}

type cronCheckpointStoreImpl struct {
	db *DB
}

type CronCheckpointStore interface {
	// GetLastDate returns the last fully-processed day for the given job, or the
	// zero time if no checkpoint exists yet (first run).
	GetLastDate(ctx context.Context, jobName string) (time.Time, error)
	// SaveLastDate upserts the checkpoint for the given job to date.
	SaveLastDate(ctx context.Context, jobName string, date time.Time) error
}

func NewCronCheckpointStore() CronCheckpointStore {
	return &cronCheckpointStoreImpl{
		db: defaultDB,
	}
}

func NewCronCheckpointStoreWithDB(db *DB) CronCheckpointStore {
	return &cronCheckpointStoreImpl{
		db: db,
	}
}

func (s *cronCheckpointStoreImpl) GetLastDate(ctx context.Context, jobName string) (time.Time, error) {
	var cp CronCheckpoint
	err := s.db.Operator.Core.NewSelect().Model(&cp).Where("job_name = ?", jobName).Scan(ctx, &cp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return cp.LastDate, nil
}

func (s *cronCheckpointStoreImpl) SaveLastDate(ctx context.Context, jobName string, date time.Time) error {
	cp := CronCheckpoint{
		JobName:  jobName,
		LastDate: date,
	}
	_, err := s.db.Operator.Core.NewInsert().Model(&cp).
		On("CONFLICT (job_name) DO UPDATE").
		Set("last_date = EXCLUDED.last_date").
		Set("updated_at = current_timestamp").
		Exec(ctx)
	return err
}
