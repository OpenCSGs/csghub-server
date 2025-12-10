package database

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/common/errorx"
)

// implement SensitiveWordSetStore
type sensitiveWordSetStoreImpl struct {
	db *DB
}

func (s *sensitiveWordSetStoreImpl) Create(ctx context.Context, input SensitiveWordSet) error {
	_, err := s.db.Core.NewInsert().Model(&input).Exec(ctx)
	return err
}

func (s *sensitiveWordSetStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model(&SensitiveWordSet{}).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *sensitiveWordSetStoreImpl) Get(ctx context.Context, id int64) (*SensitiveWordSet, error) {
	ws := &SensitiveWordSet{}
	err := s.db.Core.NewSelect().Model(ws).
		Relation("Category").
		Where("sensitive_word_set.id = ?", id).Scan(ctx)
	return ws, err
}

func (s *sensitiveWordSetStoreImpl) GetByName(ctx context.Context, name string) (*SensitiveWordSet, error) {
	ws := &SensitiveWordSet{}
	err := s.db.Core.NewSelect().Model(ws).
		Relation("Category").
		Where("sensitive_word_set.name = ?", name).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		slog.ErrorContext(ctx, "get sensitive word set by name failed", "error", err)
		return nil, errorx.HandleDBError(err, nil)
	}
	return ws, nil
}

func (s *sensitiveWordSetStoreImpl) List(ctx context.Context, filter *SensitiveWordSetFilter) ([]SensitiveWordSet, error) {
	var res []SensitiveWordSet
	q := s.db.Core.NewSelect().Model(&res).
		Relation("Category")
	if filter != nil {
		if s, ok := filter.GetSearch(); ok {
			q.Where("word_list like ?", "%"+s+"%")
		}
		if enabled, ok := filter.GetEnabled(); ok {
			q.Where("enabled = ?", enabled)
		}
	}
	err := q.Scan(ctx)
	return res, err
}

func (s *sensitiveWordSetStoreImpl) Update(ctx context.Context, input SensitiveWordSet) error {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().Model(&input).Where("id = ?", input.ID).Exec(ctx)
	return err
}

func NewSensitiveWordSetStore() SensitiveWordSetStore {
	return &sensitiveWordSetStoreImpl{
		db: GetDB(),
	}
}

func NewSensitiveWordSetStoreWithDB(db *DB) SensitiveWordSetStore {
	return &sensitiveWordSetStoreImpl{
		db: db,
	}
}
