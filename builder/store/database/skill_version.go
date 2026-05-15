package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

type skillVersionStoreImpl struct {
	db *DB
}

type SkillVersionStore interface {
	BySkillID(ctx context.Context, skillID int64) ([]SkillVersion, error)
	BySkillIDAndVersion(ctx context.Context, skillID int64, version string) (*SkillVersion, error)
	LatestBySkillID(ctx context.Context, skillID int64) (*SkillVersion, error)
	LatestBySkillIDs(ctx context.Context, skillIDs []int64) (map[int64]*SkillVersion, error)
	Create(ctx context.Context, input SkillVersion) (*SkillVersion, error)
	Update(ctx context.Context, input SkillVersion) error
	Delete(ctx context.Context, input SkillVersion) error
}

func NewSkillVersionStore() SkillVersionStore {
	return &skillVersionStoreImpl{db: defaultDB}
}

func NewSkillVersionStoreWithDB(db *DB) SkillVersionStore {
	return &skillVersionStoreImpl{db: db}
}

type SkillVersion struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	SkillID     int64  `bun:",notnull" json:"skill_id"`
	Version     string `bun:",notnull" json:"version"`
	Hash        string `bun:"," json:"hash"`
	Changelog   string `bun:",type:text" json:"changelog"`
	License     string `bun:"," json:"license"`
	StoragePath string `bun:"," json:"storage_path"`
	times
}

func (s *skillVersionStoreImpl) BySkillID(ctx context.Context, skillID int64) ([]SkillVersion, error) {
	var versions []SkillVersion
	err := s.db.Operator.Core.NewSelect().
		Model(&versions).
		Where("skill_id = ?", skillID).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill versions: %w", err)
	}
	return versions, nil
}

func (s *skillVersionStoreImpl) BySkillIDAndVersion(ctx context.Context, skillID int64, version string) (*SkillVersion, error) {
	var sv SkillVersion
	err := s.db.Operator.Core.NewSelect().
		Model(&sv).
		Where("skill_id = ? AND version = ?", skillID, version).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get skill version: %w", err)
	}
	return &sv, nil
}

func (s *skillVersionStoreImpl) LatestBySkillID(ctx context.Context, skillID int64) (*SkillVersion, error) {
	var sv SkillVersion
	err := s.db.Operator.Core.NewSelect().
		Model(&sv).
		Where("skill_id = ?", skillID).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest skill version: %w", err)
	}
	return &sv, nil
}

func (s *skillVersionStoreImpl) LatestBySkillIDs(ctx context.Context, skillIDs []int64) (map[int64]*SkillVersion, error) {
	if len(skillIDs) == 0 {
		return map[int64]*SkillVersion{}, nil
	}

	var versions []SkillVersion
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("DISTINCT ON (skill_id) *").
		Model(&versions).
		Where("skill_id IN (?)", bun.In(skillIDs)).
		Order("skill_id", "created_at DESC", "id DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest skill versions: %w", err)
	}

	result := make(map[int64]*SkillVersion, len(versions))
	for i := range versions {
		result[versions[i].SkillID] = &versions[i]
	}
	return result, nil
}

func (s *skillVersionStoreImpl) Create(ctx context.Context, input SkillVersion) (*SkillVersion, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create skill version in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create skill version in db failed,error:%w", err)
	}
	return &input, nil
}

func (s *skillVersionStoreImpl) Update(ctx context.Context, input SkillVersion) error {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return err
}

func (s *skillVersionStoreImpl) Delete(ctx context.Context, input SkillVersion) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete skill version failed,error:%w", err)
	}
	return nil
}
