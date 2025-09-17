package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var sortBy = map[string]string{
	"trending":        "popularity DESC NULLS LAST",
	"recently_update": "updated_at DESC NULLS LAST",
	"most_download":   "download_count DESC NULLS LAST",
	"most_favorite":   "likes DESC NULLS LAST",
	"most_star":       "star_count DESC NULLS LAST",
}

type datasetStoreImpl struct {
	db *DB
}

type DatasetStore interface {
	ByRepoIDs(ctx context.Context, repoIDs []int64) (datasets []Dataset, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Dataset, error)
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error)
	UserLikesDatasets(ctx context.Context, userID int64, per, page int) (datasets []Dataset, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error)
	Create(ctx context.Context, input Dataset) (*Dataset, error)
	Update(ctx context.Context, input Dataset) (err error)
	FindByPath(ctx context.Context, namespace string, repoPath string) (dataset *Dataset, err error)
	Delete(ctx context.Context, input Dataset) error
	FindByOriginPath(ctx context.Context, path string) (dataset *Dataset, err error)
	ListByPath(ctx context.Context, paths []string) ([]Dataset, error)
	CreateIfNotExist(ctx context.Context, input Dataset) (*Dataset, error)
	CreateAndUpdateRepoPath(ctx context.Context, input Dataset, path string) (*Dataset, error)
}

func NewDatasetStore() DatasetStore {
	return &datasetStoreImpl{db: defaultDB}
}

func NewDatasetStoreWithDB(db *DB) DatasetStore {
	return &datasetStoreImpl{db: db}
}

type Dataset struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func (s *datasetStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (datasets []Dataset, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&datasets).
		Relation("Repository").
		Relation("Repository.User").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Where("dataset.repository_id in (?)", bun.In(repoIDs))
	err = q.Scan(ctx)
	return
}

func (s *datasetStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Dataset, error) {
	var dataset Dataset
	err := s.db.Operator.Core.NewSelect().
		Model(&dataset).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to select dataset by repository id: %d, error: %w", repoID, err)
	}

	return &dataset, nil
}

func (s *datasetStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("dataset.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *datasetStoreImpl) UserLikesDatasets(ctx context.Context, userID int64, per, page int) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=? and deleted_at is NULL)", userID)

	query = query.Order("dataset.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *datasetStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("dataset.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &datasets)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *datasetStoreImpl) Create(ctx context.Context, input Dataset) (*Dataset, error) {
	input.LastUpdatedAt = time.Now()
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *datasetStoreImpl) Update(ctx context.Context, input Dataset) (err error) {
	input.LastUpdatedAt = time.Now()
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *datasetStoreImpl) FindByPath(ctx context.Context, namespace string, repoPath string) (dataset *Dataset, err error) {
	resDataset := new(Dataset)
	err = s.db.Operator.Core.
		NewSelect().
		Model(resDataset).
		Relation("Repository.User").
		Relation("Repository.Mirror").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resDataset.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	return resDataset, err
}

func (s *datasetStoreImpl) Delete(ctx context.Context, input Dataset) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete dataset in tx failed,error:%w", err)
	}
	return nil
}

func (s *datasetStoreImpl) ListByPath(ctx context.Context, paths []string) (datasets []Dataset, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository").
		Relation("Repository.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "evaluation")
		}).
		Where("path IN (?)", bun.In(paths)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find models by path,error: %w", err)
	}

	var sortedDatasets []Dataset
	for _, path := range paths {
		for _, ds := range datasets {
			if ds.Repository.Path == path {
				sortedDatasets = append(sortedDatasets, ds)
			}
		}
	}

	datasets = nil
	return sortedDatasets, nil
}

func (s *datasetStoreImpl) FindByOriginPath(ctx context.Context, path string) (*Dataset, error) {
	dataset := new(Dataset)
	err := s.db.Operator.Core.
		NewSelect().
		Model(dataset).
		Relation("Repository").
		Relation("Repository.Tags", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("category = ?", "evaluation")
		}).
		Where("repository.hf_path = ? or repository.ms_path = ? or repository.path = ?", path, path, path).
		Order("created_at desc").Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("path", path))
	}
	return dataset, nil
}

func (s *datasetStoreImpl) CreateIfNotExist(ctx context.Context, input Dataset) (*Dataset, error) {
	err := s.db.Core.NewSelect().
		Model(&input).
		Where("repository_id = ?", input.RepositoryID).
		Relation("Repository").
		Scan(ctx)
	if err == nil {
		return &input, nil
	}

	input.LastUpdatedAt = time.Now()
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *datasetStoreImpl) CreateAndUpdateRepoPath(ctx context.Context, input Dataset, path string) (*Dataset, error) {
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repo Repository
		_, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err != nil {
			return fmt.Errorf("failed to create dataset: %w", err)
		}
		repo, err = updateRepoPath(ctx, tx, types.DatasetRepo, path, input.RepositoryID)
		if err != nil {
			return fmt.Errorf("failed to update repository path: %w", err)
		}
		input.Repository = &repo
		return nil
	})
	return &input, err
}
