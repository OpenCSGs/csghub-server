package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type modelStoreImpl struct {
	db *DB
}

type ModelStore interface {
	ByRepoIDs(ctx context.Context, repoIDs []int64) (models []Model, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Model, error)
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (models []Model, total int, err error)
	UserLikesModels(ctx context.Context, userID int64, per, page int) (models []Model, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (models []Model, total int, err error)
	Count(ctx context.Context) (count int, err error)
	PublicCount(ctx context.Context) (count int, err error)
	Create(ctx context.Context, input Model) (*Model, error)
	Update(ctx context.Context, input Model) (*Model, error)
	FindByPath(ctx context.Context, namespace string, name string) (*Model, error)
	Delete(ctx context.Context, input Model) error
	ListByPath(ctx context.Context, paths []string) ([]Model, error)
	ByID(ctx context.Context, id int64) (*Model, error)
	CreateIfNotExist(ctx context.Context, input Model) (*Model, error)
	CreateAndUpdateRepoPath(ctx context.Context, input Model, path string) (*Model, error)
}

func NewModelStore() ModelStore {
	return &modelStoreImpl{
		db: defaultDB,
	}
}

func NewModelStoreWithDB(db *DB) ModelStore {
	return &modelStoreImpl{
		db: db,
	}
}

type Model struct {
	ID              int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID    int64       `bun:",notnull" json:"repository_id"`
	Repository      *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt   time.Time   `bun:",notnull" json:"last_updated_at"`
	BaseModel       string      `bun:"," json:"base_model"`
	ReportURL       string      `bun:"," json:"report_url"`
	MediumRiskCount int         `bun:"," json:"medium_risk_count"`
	HighRiskCount   int         `bun:"," json:"high_risk_count"`
	times
}

func (s *modelStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (models []Model, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&models).
		Relation("Repository").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Where("model.repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *modelStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Model, error) {
	var m Model
	err := s.db.Core.NewSelect().
		Model(&m).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("repo_id", repoID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find model by id, repository id: %d,error: %w", repoID, err)
	}

	return &m, nil
}

func (s *modelStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (models []Model, total int, err error) {
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
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("username", username).
		Set("public", onlyPublic),
	)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("username", username).
		Set("public", onlyPublic),
	)
	return
}

func (s *modelStoreImpl) UserLikesModels(ctx context.Context, userID int64, per, page int) (models []Model, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&models).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=? and deleted_at is NULL)", userID)

	query = query.Order("model.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("user_id", userID),
	)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("user_id", userID),
	)
	return
}

func (s *modelStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (models []Model, total int, err error) {
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
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("namespace", namespace).
		Set("public", onlyPublic),
	)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("namespace", namespace).
		Set("public", onlyPublic),
	)
	return
}

func (s *modelStoreImpl) Count(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.ModelRepo).
		Count(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *modelStoreImpl) PublicCount(ctx context.Context) (count int, err error) {
	count, err = s.db.Operator.Core.
		NewSelect().
		Model(&Repository{}).
		Where("repository_type = ?", types.DatasetRepo).
		Where("private = ?", false).
		Count(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *modelStoreImpl) Create(ctx context.Context, input Model) (*Model, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, fmt.Errorf("create model in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *modelStoreImpl) Update(ctx context.Context, input Model) (*Model, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	err = errorx.HandleDBError(err, nil)
	return &input, err
}

func (s *modelStoreImpl) FindByPath(ctx context.Context, namespace string, name string) (*Model, error) {
	resModel := new(Model)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resModel).
		Relation("Repository.User").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Relation("Repository.Metadata").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, name)).
		Limit(1).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", fmt.Sprintf("%s/%s", namespace, name)),
	)
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
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", fmt.Sprintf("%s/%s", namespace, name)),
	)
	return resModel, err
}

func (s *modelStoreImpl) Delete(ctx context.Context, input Model) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, nil)
		return fmt.Errorf("delete model in tx failed,error:%w", err)
	}
	return nil
}

func (s *modelStoreImpl) ListByPath(ctx context.Context, paths []string) ([]Model, error) {
	var models []Model
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Model{}).
		Relation("Repository").
		Where("repository.path IN (?)", bun.In(paths)).
		Scan(ctx, &models)
	err = errorx.HandleDBError(err, nil)
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

func (s *modelStoreImpl) ByID(ctx context.Context, id int64) (*Model, error) {
	var model Model
	err := s.db.Core.NewSelect().Model(&model).Relation("Repository").Where("model.id = ?", id).Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, err
	}
	return &model, err
}

func (s *modelStoreImpl) CreateIfNotExist(ctx context.Context, input Model) (*Model, error) {
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
		err = errorx.HandleDBError(err, nil)
		return nil, fmt.Errorf("create model in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *modelStoreImpl) CreateAndUpdateRepoPath(ctx context.Context, input Model, path string) (*Model, error) {
	err := s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repo Repository
		_, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err != nil {
			return fmt.Errorf("failed to create model: %w", err)
		}
		repo, err = updateRepoPath(ctx, tx, types.ModelRepo, path, input.RepositoryID)
		if err != nil {
			return fmt.Errorf("failed to update repository path: %w", err)
		}
		input.Repository = &repo
		return nil
	})
	return &input, err
}

func updateRepoPath(ctx context.Context, tx bun.Tx, repoType types.RepositoryType, repoPath string, repoID int64) (Repository, error) {
	var repo Repository
	err := tx.NewUpdate().
		Model(&Repository{}).
		Set("path = ?", repoPath).
		Set("git_path = ?", fmt.Sprintf("%ss_%s", repoType, repoPath)).
		Where("id = ?", repoID).
		Returning("*", &repo).
		Scan(ctx, &repo)
	if err != nil {
		return repo, fmt.Errorf("failed to update repository path: %w", err)
	}
	return repo, nil
}
