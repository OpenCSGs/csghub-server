package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type recomStoreImpl struct {
	db *DB
}

type RecomStore interface {
	// Index returns repos in descend order of score.
	Index(ctx context.Context, page, pageSize int) ([]*RecomRepoScore, error)
	// Upsert recom repo score
	UpsertScore(ctx context.Context, scores []*RecomRepoScore) error
	LoadWeights(ctx context.Context) ([]*RecomWeight, error)
	LoadRepoOpWeights(ctx context.Context, repoIDs []int64) (map[int64]int, error)
	UpsetOpWeights(ctx context.Context, repoID, weight int64) error
	FindScoreByRepoIDs(ctx context.Context, repoIDs []int64) ([]*RecomRepoScore, error)
}

func NewRecomStore() RecomStore {
	return &recomStoreImpl{
		db: defaultDB,
	}
}

func NewRecomStoreWithDB(db *DB) RecomStore {
	return &recomStoreImpl{
		db: db,
	}
}

// Index returns repos in descend order of score.
func (s *recomStoreImpl) Index(ctx context.Context, page, pageSize int) ([]*RecomRepoScore, error) {
	items := make([]*RecomRepoScore, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomRepoScore{}).
		Order("score desc").
		Offset(page*pageSize).Limit(pageSize).
		Scan(ctx, &items)
	return items, err
}

// Upsert recom repo score
func (s *recomStoreImpl) UpsertScore(ctx context.Context, scores []*RecomRepoScore) error {
	_, err := s.db.Operator.Core.NewInsert().
		Model(&scores).
		On("CONFLICT (repository_id, weight_name) DO UPDATE").
		Exec(ctx)
	return err
}

func (s *recomStoreImpl) LoadWeights(ctx context.Context) ([]*RecomWeight, error) {
	weights := make([]*RecomWeight, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomWeight{}).Scan(ctx, &weights)
	return weights, err
}

func (s *recomStoreImpl) LoadRepoOpWeights(ctx context.Context, repoIDs []int64) (map[int64]int, error) {
	weights := make([]*RecomRepoScore, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomRepoScore{}).Where("repository_id IN (?)", bun.In(repoIDs)).
		Where("weight_name = ?", RecomWeightOp).
		Column("repository_id", "score").
		Scan(ctx, &weights)
	if err != nil {
		return nil, fmt.Errorf("failed to load repo op weights: %w", err)
	}
	repoWeights := make(map[int64]int)
	for _, weight := range weights {
		repoWeights[weight.RepositoryID] = int(weight.Score)
	}
	return repoWeights, nil
}

func (s *recomStoreImpl) UpsetOpWeights(ctx context.Context, repoID, weight int64) error {
	_, err := s.db.Core.NewInsert().
		Model(&RecomOpWeight{
			RepositoryID: repoID,
			Weight:       int(weight),
		}).
		On("CONFLICT (repository_id) DO UPDATE").
		Exec(ctx)
	return err
}

// FindScoreByRepoIDs implements RecomStore.
func (s *recomStoreImpl) FindScoreByRepoIDs(ctx context.Context, repoIDs []int64) ([]*RecomRepoScore, error) {
	items := make([]*RecomRepoScore, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomRepoScore{}).
		Where("repository_id IN (?)", bun.In(repoIDs)).
		Scan(ctx, &items)
	return items, err
}
