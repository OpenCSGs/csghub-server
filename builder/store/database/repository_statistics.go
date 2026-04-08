package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/errorx"
)

// RepositoryStatistics represents the repository_statistics table
type RepositoryStatistics struct {
	ID           int64     `bun:"id,pk,autoincrement"`
	RepositoryID int64     `bun:"repository_id,notnull"`
	Branch       string    `bun:"branch,notnull"`
	TotalSize    int64     `bun:"total_size,notnull,default:0"`
	NonLfsSize   int64     `bun:"non_lfs_size,notnull,default:0"`
	LfsSize      int64     `bun:"lfs_size,notnull,default:0"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// Relations
	Repository *Repository `bun:"rel:belongs-to,join:repository_id=id"`
}

type RepositoryStatisticsStore interface {
	// Create creates a new repository statistics record
	Create(ctx context.Context, stats *RepositoryStatistics) error

	// FindByRepositoryID finds repository statistics by repository ID
	FindByRepositoryID(ctx context.Context, repoID int64) (*RepositoryStatistics, error)

	// FindByRepositoryIDAndBranch finds repository statistics by repository ID and branch
	FindByRepositoryIDAndBranch(ctx context.Context, repoID int64, branch string) (*RepositoryStatistics, error)

	// Update updates repository statistics
	Update(ctx context.Context, stats *RepositoryStatistics) error

	// Delete deletes repository statistics
	Delete(ctx context.Context, stats *RepositoryStatistics) error

	// BatchUpdate batch updates repository statistics
	BatchUpdate(ctx context.Context, stats []*RepositoryStatistics) error
}

// RepositoryStatisticsStoreImpl implements RepositoryStatisticsStore
type RepositoryStatisticsStoreImpl struct {
	db *DB
}

// NewRepositoryStatisticsStore creates a new RepositoryStatisticsStore
func NewRepositoryStatisticsStore() RepositoryStatisticsStore {
	return &RepositoryStatisticsStoreImpl{
		db: defaultDB,
	}
}

// NewRepositoryStatisticsStoreWithDB creates a new RepositoryStatisticsStore with a specific DB
func NewRepositoryStatisticsStoreWithDB(db *DB) RepositoryStatisticsStore {
	return &RepositoryStatisticsStoreImpl{
		db: db,
	}
}

// Create creates a new repository statistics record
func (s *RepositoryStatisticsStoreImpl) Create(ctx context.Context, stats *RepositoryStatistics) error {
	_, err := s.db.Operator.Core.NewInsert().Model(stats).Exec(ctx, stats)
	return errorx.HandleDBError(err, nil)
}

// FindByRepositoryID finds repository statistics by repository ID
func (s *RepositoryStatisticsStoreImpl) FindByRepositoryID(ctx context.Context, repoID int64) (*RepositoryStatistics, error) {
	var stats RepositoryStatistics
	_, err := s.db.Operator.Core.NewSelect().Model(&stats).Where("repository_id = ?", repoID).Exec(ctx, &stats)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &stats, nil
}

// FindByRepositoryIDAndBranch finds repository statistics by repository ID and branch
func (s *RepositoryStatisticsStoreImpl) FindByRepositoryIDAndBranch(ctx context.Context, repoID int64, branch string) (*RepositoryStatistics, error) {
	var stats RepositoryStatistics
	_, err := s.db.Operator.Core.NewSelect().Model(&stats).Where("repository_id = ? AND branch = ?", repoID, branch).Exec(ctx, &stats)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &stats, nil
}

// Update updates repository statistics
func (s *RepositoryStatisticsStoreImpl) Update(ctx context.Context, stats *RepositoryStatistics) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(stats).Where("id = ?", stats.ID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// Delete deletes repository statistics
func (s *RepositoryStatisticsStoreImpl) Delete(ctx context.Context, stats *RepositoryStatistics) error {
	_, err := s.db.Operator.Core.NewDelete().Model(stats).Where("id = ?", stats.ID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// BatchUpdate batch updates repository statistics
func (s *RepositoryStatisticsStoreImpl) BatchUpdate(ctx context.Context, stats []*RepositoryStatistics) error {
	for _, stat := range stats {
		if err := s.Update(ctx, stat); err != nil {
			return err
		}
	}
	return nil
}
