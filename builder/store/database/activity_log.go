package database

import (
	"context"
	"time"
)

type activityLogStoreImpl struct {
	db *DB
}

type ActivityLogStore interface {
	BatchCreate(ctx context.Context, logs []ActivityLog) error
	FindByTimeAfter(ctx context.Context, after time.Time, per, page int) ([]ActivityLog, int, error)
}

func NewActivityLogStore() ActivityLogStore {
	return &activityLogStoreImpl{db: defaultDB}
}

func NewActivityLogStoreWithDB(db *DB) ActivityLogStore {
	return &activityLogStoreImpl{db: db}
}

func (s *activityLogStoreImpl) BatchCreate(ctx context.Context, logs []ActivityLog) error {
	if len(logs) == 0 {
		return nil
	}
	result, err := s.db.Core.NewInsert().Model(&logs).Exec(ctx)
	return assertAffectedXRows(int64(len(logs)), result, err)
}

func (s *activityLogStoreImpl) FindByTimeAfter(ctx context.Context, after time.Time, per, page int) ([]ActivityLog, int, error) {
	var logs []ActivityLog
	q := s.db.Core.NewSelect().
		Model(&logs).
		Where("operation_time > ?", after).
		Order("operation_time DESC")

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	err = q.Limit(per).Offset((page - 1) * per).Scan(ctx)
	return logs, count, err
}
