package database

import "context"

type recomStoreImpl struct {
	db *DB
}

type RecomStore interface {
	// Index returns repos in descend order of score.
	Index(ctx context.Context, page, pageSize int) ([]*RecomRepoScore, error)
	// Upsert recom repo score
	UpsertScore(ctx context.Context, repoID int64, score float64) error
	LoadWeights(ctx context.Context) ([]*RecomWeight, error)
	LoadOpWeights(ctx context.Context) ([]*RecomOpWeight, error)
	UpsetOpWeights(ctx context.Context, repoID, weight int64) error
}

func NewRecomStore() RecomStore {
	return &recomStoreImpl{
		db: defaultDB,
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
func (s *recomStoreImpl) UpsertScore(ctx context.Context, repoID int64, score float64) error {
	_, err := s.db.Operator.Core.NewInsert().
		Model(&RecomRepoScore{
			RepositoryID: repoID,
			Score:        score,
		}).
		On("CONFLICT (repository_id) DO UPDATE").
		Exec(ctx)
	return err
}

func (s *recomStoreImpl) LoadWeights(ctx context.Context) ([]*RecomWeight, error) {
	weights := make([]*RecomWeight, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomWeight{}).Scan(ctx, &weights)
	return weights, err
}

func (s *recomStoreImpl) LoadOpWeights(ctx context.Context) ([]*RecomOpWeight, error) {
	weights := make([]*RecomOpWeight, 0)
	err := s.db.Operator.Core.NewSelect().Model(&RecomOpWeight{}).Scan(ctx, &weights)
	return weights, err
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
