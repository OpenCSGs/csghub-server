package database

import (
	"context"
	"fmt"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type ModelStore struct {
	db *model.DB
}

func NewModelStore(db *model.DB) *ModelStore {
	return &ModelStore{
		db: db,
	}
}

func (s *ModelStore) Index(ctx context.Context, per, page int) (models []Repository, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Where("repository_type = ?", ModelRepo).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) PublicRepos(ctx context.Context, per, page int) (models []Repository, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Where("repository_type = ?", ModelRepo).
		Where("private = ?", false).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) RepoByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (models []Repository, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Join("JOIN users AS u ON u.id = repository.user_id").
		Where("u.username = ?", username).
		Where("repository_type = ?", ModelRepo)

	if onlyPublic {
		query = query.Where("private = ?", false)
	}
	query = query.Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &models)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", ModelRepo).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) PublicRepoCount(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", DatasetRepo).
		Where("private = ?", false).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) CreateRepo(ctx context.Context, repo *Repository, userId int) (err error) {
	repo.UserID = userId
	err = s.db.Operator.Core.NewInsert().Model(repo).Scan(ctx)
	return
}

func (s *ModelStore) UpdateRepo(ctx context.Context, repo *Repository) (err error) {
	repo.UpdatedAt = time.Now()
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().Model(repo).WherePK().Exec(ctx))
	return
}

func (s *ModelStore) FindyByRepoPath(ctx context.Context, owner string, repoPath string) (model *Repository, err error) {
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

func (s *ModelStore) DeleteRepo(ctx context.Context, username, name string) (err error) {
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&Repository{}).
		Where("path = ?", fmt.Sprintf("%v/%v", username, name)).
		Where("repository_type = ?", ModelRepo).
		Exec(ctx)
	return
}
