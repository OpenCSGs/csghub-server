package database

import (
	"context"
	"fmt"
	"log/slog"
)

type BroadcastStore interface {
	Save(ctx context.Context, broadcast Broadcast) error
	Get(ctx context.Context, id int64) (*Broadcast, error)
	Update(ctx context.Context, broadcast Broadcast) (*Broadcast, error)
	FindAll(ctx context.Context) ([]Broadcast, error)
}

type broadcastStoreImpl struct {
	db *DB
}

func NewBroadcastStore() BroadcastStore {
	return &broadcastStoreImpl{
		db: defaultDB,
	}
}

func NewBroadcastStoreWithDB(db *DB) BroadcastStore {
	return &broadcastStoreImpl{
		db: db,
	}
}

func (s *broadcastStoreImpl) Save(ctx context.Context, broadcast Broadcast) error {
	return assertAffectedOneRow(s.db.Core.NewInsert().Model(&broadcast).Exec(ctx))
}

func (s *broadcastStoreImpl) FindAll(ctx context.Context) ([]Broadcast, error) {
	var broadcasts []Broadcast
	err := s.db.Operator.Core.NewSelect().Model(&Broadcast{}).Order("created_at DESC").Scan(ctx, &broadcasts)
	if err != nil {
		slog.Error("Failed to select broadcasts", "error", err)
		return nil, err
	}
	return broadcasts, nil
}

func (s *broadcastStoreImpl) Get(ctx context.Context, id int64) (*Broadcast, error) {
	broadcast := new(Broadcast)
	err := s.db.Operator.Core.
		NewSelect().
		Model(broadcast).
		Where("id =?", id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("can not find collection: %w", err)
	}

	return broadcast, err
}

func (s *broadcastStoreImpl) Update(ctx context.Context, broadcast Broadcast) (*Broadcast, error) {
	_, err := s.db.Core.NewUpdate().Model(&broadcast).WherePK().Exec(ctx)
	return &broadcast, err
}
