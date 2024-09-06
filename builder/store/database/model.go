package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type ModelStore struct {
	db *DB
}

func NewModelStore() *ModelStore {
	return &ModelStore{
		db: defaultDB,
	}
}

type Model struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	BaseModel     string      `bun:"," json:"base_model"`
	times
}

func (s *ModelStore) ByRepoIDs(ctx context.Context, repoIDs []int64) (models []Model, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&models).
		Relation("Repository").
		Where("repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return
}

func (s *ModelStore) ByRepoID(ctx context.Context, repoID int64) (*Model, error) {
	var m Model
	err := s.db.Core.NewSelect().
		Model(&m).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find model by id, repository id: %d,error: %w", repoID, err)
	}

	return &m, nil
}

func (s *ModelStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (models []Model, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("model.created_at DESC").
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

func (s *ModelStore) UserLikesModels(ctx context.Context, userID int64, per, page int) (models []Model, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=?)", userID)

	query = query.Order("model.created_at DESC").
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

func (s *ModelStore) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (models []Model, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("model.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &models)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.ModelRepo).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) PublicCount(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.DatasetRepo).
		Where("private = ?", false).
		Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *ModelStore) Create(ctx context.Context, input Model) (*Model, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create model in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create model in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *ModelStore) Update(ctx context.Context, input Model) (*Model, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *ModelStore) FindByPath(ctx context.Context, namespace string, name string) (*Model, error) {
	resModel := new(Model)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resModel).
		Relation("Repository.User").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, name)).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find model,error: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resModel.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	return resModel, err
}

func (s *ModelStore) Delete(ctx context.Context, input Model) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete model in tx failed,error:%w", err)
	}
	return nil
}

func (s *ModelStore) ListByPath(ctx context.Context, paths []string) ([]Model, error) {
	var models []Model
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Model{}).
		Relation("Repository").
		Where("repository.path IN (?)", bun.In(paths)).
		Scan(ctx, &models)
	if err != nil {
		return nil, fmt.Errorf("failed to find models by path,error: %w", err)
	}

	var sortedModels []Model
	for _, path := range paths {
		for _, m := range models {
			if m.Repository.Path == path {
				sortedModels = append(sortedModels, m)
			}
		}
	}

	return sortedModels, nil
}

func (s *ModelStore) ByID(ctx context.Context, id int64) (*Model, error) {
	var model Model
	err := s.db.Core.NewSelect().Model(&model).Relation("Repository").Where("model.id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &model, err
}

func (s *ModelStore) CreateIfNotExist(ctx context.Context, input Model) (*Model, error) {
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
		slog.Error("create model in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create model in db failed,error:%w", err)
	}

	return &input, nil
}
