package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type spaceStoreImpl struct {
	db *DB
}

type SpaceStore interface {
	// BeginTx(ctx context.Context) (bun.Tx, error)
	// CreateTx(ctx context.Context, tx bun.Tx, input Space) (*Space, error)
	Create(ctx context.Context, input Space) (*Space, error)
	Update(ctx context.Context, input Space) (err error)
	FindByPath(ctx context.Context, namespace, name string) (*Space, error)
	Delete(ctx context.Context, input Space) error
	ByID(ctx context.Context, id int64) (*Space, error)
	// ByRepoIDs get spaces by repoIDs, only basice info, no related repo
	ByRepoIDs(ctx context.Context, repoIDs []int64) (spaces []Space, err error)
	ByRepoID(ctx context.Context, repoID int64) (*Space, error)
	ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (spaces []Space, total int, err error)
	ByUserLikes(ctx context.Context, userID int64, per, page int) (spaces []Space, total int, err error)
	ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (spaces []Space, total int, err error)
	ListByPath(ctx context.Context, paths []string) ([]Space, error)
}

func NewSpaceStore() SpaceStore {
	return &spaceStoreImpl{
		db: defaultDB,
	}
}

func NewSpaceStoreWithDB(db *DB) SpaceStore {
	return &spaceStoreImpl{
		db: db,
	}
}

// func (s *spaceStoreImpl) BeginTx(ctx context.Context) (bun.Tx, error) {
// 	return s.db.Core.BeginTx(ctx, nil)
// }

// func (s *spaceStoreImpl) CreateTx(ctx context.Context, tx bun.Tx, input Space) (*Space, error) {
// 	res, err := tx.NewInsert().Model(&input).Exec(ctx)
// 	if err := assertAffectedOneRow(res, err); err != nil {
// 		slog.Error("create space in tx failed", slog.String("error", err.Error()))
// 		return nil, fmt.Errorf("create space in tx failed,error:%w", err)
// 	}

// 	input.ID, _ = res.LastInsertId()
// 	return &input, nil
// }

func (s *spaceStoreImpl) Create(ctx context.Context, input Space) (*Space, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create space in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create space in db failed,error:%w", err)
	}

	input.ID, _ = res.LastInsertId()
	return &input, nil
}

func (s *spaceStoreImpl) Update(ctx context.Context, input Space) (err error) {
	_, err = s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	return
}

func (s *spaceStoreImpl) FindByPath(ctx context.Context, namespace, name string) (*Space, error) {
	resSpace := new(Space)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resSpace).
		Relation("Repository.User").
		Where("repository.path = ? and repository.repository_type = ?", fmt.Sprintf("%s/%s", namespace, name), types.SpaceRepo).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find space: %w", err)
	}

	return resSpace, err
}

func (s *spaceStoreImpl) Delete(ctx context.Context, input Space) error {
	res, err := s.db.Operator.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("delete space in tx failed,error:%w", err)
	}
	return nil
}

func (s *spaceStoreImpl) ByID(ctx context.Context, id int64) (*Space, error) {
	var space Space
	err := s.db.Core.NewSelect().Model(&space).Relation("Repository").Where("space.id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &space, err
}

// ByRepoIDs get spaces by repoIDs, only basic info, no related repo
func (s *spaceStoreImpl) ByRepoIDs(ctx context.Context, repoIDs []int64) (spaces []Space, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&spaces).
		Where("repository_id in (?)", bun.In(repoIDs)).
		Scan(ctx)

	return

}

func (s *spaceStoreImpl) ByRepoID(ctx context.Context, repoID int64) (*Space, error) {
	var space Space
	err := s.db.Core.NewSelect().Model(&space).Where("repository_id = ?", repoID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find space by id, repository id: %d,error: %w", repoID, err)
	}
	return &space, err
}

func (s *spaceStoreImpl) ByUsername(ctx context.Context, username string, per, page int, onlyPublic bool) (spaces []Space, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&spaces).
		Relation("Repository.Tags").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", username))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("space.created_at DESC").
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

func (s *spaceStoreImpl) ByUserLikes(ctx context.Context, userID int64, per, page int) (spaces []Space, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&spaces).
		Relation("Repository.Tags").
		Where("repository.id in (select repo_id from user_likes where user_id=?)", userID)

	query = query.Order("space.created_at DESC").
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

func (s *spaceStoreImpl) ByOrgPath(ctx context.Context, namespace string, per, page int, onlyPublic bool) (spaces []Space, total int, err error) {
	query := s.db.Operator.Core.
		NewSelect().
		Model(&spaces).
		Relation("Repository.Tags").
		Relation("Repository.User").
		Where("repository.path like ?", fmt.Sprintf("%s/%%", namespace))

	if onlyPublic {
		query = query.Where("repository.private = ?", false)
	}
	query = query.Order("space.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx, &spaces)
	if err != nil {
		return
	}
	total, err = query.Count(ctx)
	if err != nil {
		return
	}
	return
}

func (s *spaceStoreImpl) ListByPath(ctx context.Context, paths []string) ([]Space, error) {
	var spaces []Space
	err := s.db.Operator.Core.
		NewSelect().
		Model(&Space{}).
		Relation("Repository").
		Where("path IN (?)", bun.In(paths)).
		Scan(ctx, &spaces)
	if err != nil {
		return nil, fmt.Errorf("failed to find space by path,error: %w", err)
	}

	var sortedSpaces []Space
	for _, path := range paths {
		for _, ds := range spaces {
			if ds.Repository.Path == path {
				sortedSpaces = append(sortedSpaces, ds)
			}
		}
	}

	spaces = nil
	return sortedSpaces, nil
}
