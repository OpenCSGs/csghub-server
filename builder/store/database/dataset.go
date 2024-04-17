package database

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var sortBy = map[string]string{
	"trending":        "download_count DESC NULLS LAST",
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

func (s *DatasetStore) PublicToUser(ctx context.Context, user *User, search, sort string, tags []TagReq, per, page int) (datasets []Dataset, count int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Relation("Repository.Tags")

	if user != nil {
		query = query.Where("repository.private = ? or repository.user_id = ?", false, user.ID)
	} else {
		query = query.Where("repository.private = ?", false)
	}

	if search != "" {
		search = strings.ToLower(search)
		query = query.Where(
			"LOWER(repository.path) like ? or LOWER(repository.description) like ? or LOWER(repository.nickname) like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	// TODO：Optimize SQL
	if len(tags) > 0 {
		for _, tag := range tags {
			query = query.Where("dataset.repository_id IN (SELECT repository_id FROM repository_tags JOIN tags ON repository_tags.tag_id = tags.id WHERE tags.category = ? AND tags.name = ?)", tag.Category, tag.Name)
		}
	}
	count, err = query.Count(ctx)
	if err != nil {
		return
	}

	query = query.Order(fmt.Sprintf("repository.%s", sortBy[sort]))
	query = query.Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return
	}
	return
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
	input.UpdatedAt = time.Now()
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
		Relation("Tags").
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
