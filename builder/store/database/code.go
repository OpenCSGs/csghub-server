package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

type codeStoreImpl struct {
	db *DB
}

type CodeStore interface {
	ByRepoIDs(ctx context.Context, repoIDs []int64) (codes []Code, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Code, error)
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (codes []Code, total int, err error)
	UserLikesCodes(ctx context.Context, userID int64, per, page int) (codes []Code, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (codes []Code, total int, err error)
	Create(ctx context.Context, input Code) (*Code, error)
	Update(ctx context.Context, input Code) (err error)
	FindByPath(ctx context.Context, namespace string, repoPath string) (code *Code, err error)
	Delete(ctx context.Context, input Code) error
	ListByPath(ctx context.Context, paths []string) ([]Code, error)
	CreateIfNotExist(ctx context.Context, input Code) (*Code, error)
}

func NewCodeStore() CodeStore {
	return &codeStoreImpl{db: defaultDB}
}

func NewCodeStoreWithDB(db *DB) CodeStore {
	return &codeStoreImpl{db: db}
}

type Code struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func (s *codeStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (codes []Code, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&codes).
		Relation("Repository").
		Relation("Repository.Mirror").
		Relation("Repository.Mirror.CurrentTask").
		Where("code.repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return
}

func (s *codeStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Code, error) {
	var code Code
	err := s.db.Operator.Core.NewSelect().
		Model(&code).
		Where("repository_id = ?", repoID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to select code, error: %w", err)
	}

	return &code, nil
}

func (s *codeStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (codes []Code, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&codes).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("code.created_at DESC").
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

func (s *codeStoreImpl) UserLikesCodes(ctx context.Context, userID int64, per, page int) (codes []Code, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&codes).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=? and deleted_at is NULL)", userID)

	query = query.Order("code.created_at DESC").
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

func (s *codeStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (codes []Code, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&codes).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("code.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &codes)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *codeStoreImpl) Create(ctx context.Context, input Code) (*Code, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create code in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create code in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *codeStoreImpl) Update(ctx context.Context, input Code) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *codeStoreImpl) FindByPath(ctx context.Context, namespace string, repoPath string) (code *Code, err error) {
	resCode := new(Code)
	err = s.db.Operator.Core.
		NewSelect().
		Model(resCode).
		Relation("Repository.User").
		Where("repository.path =?", fmt.Sprintf("%s/%s", namespace, repoPath)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find code: %w", err)
	}
	err = s.db.Operator.Core.NewSelect().
		Model(resCode.Repository).
		WherePK().
		Relation("Tags", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("repository_tag.count > 0")
		}).
		Scan(ctx)
	return resCode, err
}

func (s *codeStoreImpl) Delete(ctx context.Context, input Code) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete code in tx failed,error:%w", err)
	}
	return nil
}

func (s *codeStoreImpl) ListByPath(ctx context.Context, paths []string) ([]Code, error) {
	var codes []Code
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Code{}).
		Relation("Repository").
		Where("path IN (?)", bun.In(paths)).
		Scan(ctx, &codes)
	if err != nil {
		return nil, fmt.Errorf("failed to find models by path,error: %w", err)
	}
	return codes, nil
}

func (s *codeStoreImpl) CreateIfNotExist(ctx context.Context, input Code) (*Code, error) {
	err := s.db.Core.NewSelect().
		Model(&input).
		Where("repository_id = ?", input.RepositoryID).
		Relation("Repository").
		Scan(ctx)
	if err == nil {
		return &input, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return &input, err
	}

	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create code in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create code in db failed,error:%w", err)
	}

	return &input, nil
}
