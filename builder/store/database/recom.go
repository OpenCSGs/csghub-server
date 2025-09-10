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
	// Upsert recom repo score
	UpsertScore(ctx context.Context, scores []*RecomRepoScore) error
	LoadWeights(ctx context.Context) ([]*RecomWeight, error)
	LoadRepoOpWeights(ctx context.Context, repoIDs []int64) (map[int64]int, error)
	UpsetOpWeights(ctx context.Context, repoID, weight int64) error
	FindByRepoIDs(ctx context.Context, repoIDs []int64) ([]*RecomRepoScore, error)
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
	items := make([]*RecomRepoScore, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomRepoScore{}).
		Where("repository_id = ?", repoID).
		Scan(ctx, &items)
	if err != nil {
		return fmt.Errorf("failed to load recom repo scores: %w", err)
	}

	var opScore, totalScore *RecomRepoScore
	var total float64
	for _, item := range items {
		if item.WeightName == RecomWeightOp {
			opScore = item
			opScore.Score = float64(weight)
		} else if item.WeightName == RecomWeightTotal {
			totalScore = item
			totalScore.Score = 0.0 // reset total
		}

		total += item.Score
	}

	if opScore == nil {
		items = append(items, &RecomRepoScore{
			RepositoryID: repoID,
			WeightName:   RecomWeightOp,
			Score:        float64(weight),
		})
		total += float64(weight)
	}
	if totalScore == nil {
		totalScore = &RecomRepoScore{
			RepositoryID: repoID,
			WeightName:   RecomWeightTotal,
			Score:        total,
		}
		items = append(items, totalScore)
	} else {
		totalScore.Score = total
	}

	return s.UpsertScore(ctx, items)
}

// FindByRepoIDs implements RecomStore.
func (s *recomStoreImpl) FindByRepoIDs(ctx context.Context, repoIDs []int64) ([]*RecomRepoScore, error) {
	items := make([]*RecomRepoScore, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomRepoScore{}).
		Where("repository_id IN (?)", bun.In(repoIDs)).
		Scan(ctx, &items)
	return items, err
}
