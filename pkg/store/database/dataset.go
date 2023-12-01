package database

import (
	"context"
	"fmt"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type DatasetStore struct {
	db *model.DB
}

func NewDatasetStore(db *model.DB) *DatasetStore {
	return &DatasetStore{
		db: db,
	}
}

func (s *DatasetStore) Index(ctx context.Context, per, page int) (datasets []*Repository, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&datasets).
		Where("repository_type = ?", DatasetRepo).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", DatasetRepo).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *DatasetStore) CreateRepo(ctx context.Context, repo *Repository) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(repo).Scan(ctx)
	return
}

func (s *DatasetStore) UpdateRepo(ctx context.Context, repo *Repository) (err error) {
	repo.UpdatedAt = time.Now()
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().Model(repo).WherePK().Exec(ctx))
	return
}

func (s *DatasetStore) FindyByRepoPath(ctx context.Context, owner string, repoPath string) (model *Repository, err error) {
	var repos []Repository
	err = s.db.Operator.Core.
		NewSelect().
		Model(&repos).
		Where("path =?", fmt.Sprintf("%s/%s", owner, repoPath)).
		Where("name =?", repoPath).
		Scan(ctx)
	if err != nil {
		return
	}
	if len(repos) == 0 {
		return
	}

	return &repos[0], nil
}

func (s *DatasetStore) DeleteRepo(ctx context.Context, username, name string) (err error) {
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&Repository{}).
		Where("path = ?", fmt.Sprintf("%v/%v", username, name)).
		Where("repository_type = ?", DatasetRepo).
		Exec(ctx)
	return
}
