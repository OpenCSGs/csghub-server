package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
)

type CodeStore struct {
	db *DB
}

func NewCodeStore() *CodeStore {
	return &CodeStore{db: defaultDB}
}

type Code struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func (s *CodeStore) ByRepoIDs(ctx context.Context, repoIDs []int64) (codes []Code, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&codes).
		Where("repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return
}

func (s *CodeStore) ByRepoID(ctx context.Context, repoID int64) (*Code, error) {
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

func (s *CodeStore) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (codes []Code, total int, err error) {
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

func (s *CodeStore) UserLikesCodes(ctx context.Context, userID int64, per, page int) (codes []Code, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&codes).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.id in (select repo_id from user_likes where user_id=?)", userID)

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

func (s *CodeStore) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (codes []Code, total int, err error) {
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

func (s *CodeStore) Create(ctx context.Context, input Code) (*Code, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create code in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create code in db failed,error:%w", err)
	}

	return &input, nil
}

func (s *CodeStore) Update(ctx context.Context, input Code) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *CodeStore) FindByPath(ctx context.Context, namespace string, repoPath string) (code *Code, err error) {
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

func (s *CodeStore) Delete(ctx context.Context, input Code) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete code in tx failed,error:%w", err)
	}
	return nil
}

func (s *CodeStore) ListByPath(ctx context.Context, paths []string) ([]Code, error) {
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
