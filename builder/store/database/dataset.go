package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

var sortBy = map[string]string{
	"trending":        "popularity DESC NULLS LAST",
	"recently_update": "updated_at DESC NULLS LAST",
	"most_download":   "download_count DESC NULLS LAST",
	"most_favorite":   "likes DESC NULLS LAST",
}

type DatasetStore struct {
	db *DB
}

func NewDatasetStore() *DatasetStore {
	return &DatasetStore{db: defaultDB}
}

type Dataset struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func (s *DatasetStore) ByRepoIDs(ctx context.Context, repoIDs []int64) (datasets []Dataset, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&datasets).
		Relation("Repository").
		Where("repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return
}

func (s *DatasetStore) ByRepoID(ctx context.Context, repoID int64) (*Dataset, error) {
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

func (s *DatasetStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
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

func (s *DatasetStore) UserLikesDatasets(ctx context.Context, userID int64, per, page int) (datasets []Dataset, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=?)", userID)

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

func (s *DatasetStore) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (datasets []Dataset, total int, err error) {
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

func (s *DatasetStore) Create(ctx context.Context, input Dataset) (*Dataset, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *DatasetStore) Update(ctx context.Context, input Dataset) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *DatasetStore) FindByPath(ctx context.Context, namespace string, repoPath string) (dataset *Dataset, err error) {
	resDataset := new(Dataset)
	err = s.db.Operator.Core.
		NewSelect().
		Model(resDataset).
		Relation("Repository.User").
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

func (s *DatasetStore) Delete(ctx context.Context, input Dataset) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete dataset in tx failed,error:%w", err)
	}
	return nil
}

func (s *DatasetStore) ListByPath(ctx context.Context, paths []string) ([]Dataset, error) {
	var datasets []Dataset
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Dataset{}).
		Relation("Repository").
		Where("path IN (?)", bun.In(paths)).
		Scan(ctx, &datasets)
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

func (s *DatasetStore) CreateIfNotExist(ctx context.Context, input Dataset) (*Dataset, error) {
	err := s.db.Core.NewSelect().
		Model(&input).
		Where("repository_id = ?", input.RepositoryID).
		Relation("Repository").
		Scan(ctx)
	if err == nil {
		return &input, nil
	}

	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create dataset in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create dataset in db failed,error:%w", err)
	}

	return &input, nil
}
