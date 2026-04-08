package database

import (
	"context"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AgentInstanceScheduler struct {
	ID             int64      `bun:",pk,autoincrement" json:"id"`
	UserUUID       string     `bun:",notnull" json:"user_uuid"`
	InstanceID     int64      `bun:",notnull" json:"instance_id"`
	Name           string     `bun:",notnull" json:"name"`
	Prompt         string     `bun:",type:text,notnull" json:"prompt"`
	ScheduleType   string     `bun:",notnull" json:"schedule_type"` // once, daily, weekly, monthly
	CronExpression string     `bun:",notnull" json:"cron_expression"`
	StartDate      time.Time  `bun:",notnull" json:"start_date"`
	StartTime      time.Time  `bun:",notnull" json:"start_time"`
	EndDate        *time.Time `bun:",nullzero" json:"end_date,omitempty"`
	ScheduleID     string     `bun:",nullzero" json:"schedule_id,omitempty"`
	Status         string     `bun:",notnull,default:'active'" json:"status"` // active, paused
	LastRunAt      *time.Time `bun:",nullzero" json:"last_run_at,omitempty"`
	times
}

type AgentInstanceSchedulerStore interface {
	Create(ctx context.Context, scheduler *AgentInstanceScheduler) (*AgentInstanceScheduler, error)
	FindByID(ctx context.Context, id int64) (*AgentInstanceScheduler, error)
	ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentSchedulerFilter, per, page int) ([]AgentInstanceScheduler, int, error)
	CountByUserUUID(ctx context.Context, userUUID string) (int, error)
	Update(ctx context.Context, scheduler *AgentInstanceScheduler) error
	Delete(ctx context.Context, id int64) error
}

type agentInstanceSchedulerStoreImpl struct {
	db *DB
}

func NewAgentInstanceSchedulerStore() AgentInstanceSchedulerStore {
	return &agentInstanceSchedulerStoreImpl{
		db: defaultDB,
	}
}

func NewAgentInstanceSchedulerStoreWithDB(db *DB) AgentInstanceSchedulerStore {
	return &agentInstanceSchedulerStoreImpl{
		db: db,
	}
}

func (s *agentInstanceSchedulerStoreImpl) Create(ctx context.Context, scheduler *AgentInstanceScheduler) (*AgentInstanceScheduler, error) {
	res, err := s.db.Core.NewInsert().Model(scheduler).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"user_uuid":   scheduler.UserUUID,
			"instance_id": scheduler.InstanceID,
		})
	}
	return scheduler, nil
}

func (s *agentInstanceSchedulerStoreImpl) FindByID(ctx context.Context, id int64) (*AgentInstanceScheduler, error) {
	scheduler := &AgentInstanceScheduler{}
	err := s.db.Core.NewSelect().Model(scheduler).Where("id = ?", id).Scan(ctx, scheduler)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]any{
			"scheduler_id": id,
		})
	}
	return scheduler, nil
}

// applySchedulerFilters applies status, instance_id, schedule_type, and search (name) filters to the query.
func (s *agentInstanceSchedulerStoreImpl) applySchedulerFilters(query *bun.SelectQuery, filter types.AgentSchedulerFilter) *bun.SelectQuery {
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.InstanceID != nil {
		query = query.Where("instance_id = ?", *filter.InstanceID)
	}
	if filter.ScheduleType != "" {
		query = query.Where("schedule_type = ?", filter.ScheduleType)
	}
	if filter.Search != "" {
		filter.Search = strings.TrimSpace(filter.Search)
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("LOWER(name) LIKE LOWER(?)", searchPattern)
	}
	if filter.NotFinished != nil && *filter.NotFinished {
		// Exclude schedulers with status=finished (once after run, or recurring after end_date passed)
		query = query.Where("status != ?", types.AgentSchedulerStatusFinished)
	}
	return query
}

const schedulerDefaultOrder = "created_at DESC"

func (s *agentInstanceSchedulerStoreImpl) ListByUserUUID(ctx context.Context, userUUID string, filter types.AgentSchedulerFilter, per, page int) ([]AgentInstanceScheduler, int, error) {
	var schedulers []AgentInstanceScheduler
	query := s.db.Core.NewSelect().Model(&schedulers).Where("user_uuid = ?", userUUID)
	query = s.applySchedulerFilters(query, filter)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}

	err = query.Order(schedulerDefaultOrder).Limit(per).Offset((page - 1) * per).Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}

	return schedulers, total, nil
}

// CountByUserUUID returns the count of schedulers for a user, excluding finished ones (active and paused only).
func (s *agentInstanceSchedulerStoreImpl) CountByUserUUID(ctx context.Context, userUUID string) (int, error) {
	count, err := s.db.Core.NewSelect().
		Model((*AgentInstanceScheduler)(nil)).
		Where("user_uuid = ?", userUUID).
		Where("status != ?", types.AgentSchedulerStatusFinished).
		Count(ctx)
	if err != nil {
		return 0, errorx.HandleDBError(err, map[string]any{
			"user_uuid": userUUID,
		})
	}
	return count, nil
}

func (s *agentInstanceSchedulerStoreImpl) Update(ctx context.Context, scheduler *AgentInstanceScheduler) error {
	res, err := s.db.Core.NewUpdate().Model(scheduler).Where("id = ?", scheduler.ID).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"scheduler_id": scheduler.ID,
		})
	}
	return nil
}

func (s *agentInstanceSchedulerStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := s.db.Core.NewDelete().Model((*AgentInstanceScheduler)(nil)).Where("id = ?", id).Exec(ctx)
	if err = assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"scheduler_id": id,
		})
	}
	return nil
}
