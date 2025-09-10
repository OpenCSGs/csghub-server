package database

import (
	"context"
	"time"
)

type lfsLockStoreImpl struct {
	db *DB
}

type LfsLockStore interface {
	FindByID(ctx context.Context, ID int64) (*LfsLock, error)
	FindByPath(ctx context.Context, RepoId int64, path string) (*LfsLock, error)
	FindByRepoID(ctx context.Context, RepoId int64, page, per int) ([]LfsLock, error)
	Create(ctx context.Context, lfsLock LfsLock) (*LfsLock, error)
	RemoveByID(ctx context.Context, ID int64) error
}

func NewLfsLockStore() LfsLockStore {
	return &lfsLockStoreImpl{
		db: defaultDB,
	}
}

func NewLfsLockStoreWithDB(db *DB) LfsLockStore {
	return &lfsLockStoreImpl{
		db: db,
	}
}

type LfsLock struct {
	ID           int64      `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64      `bun:",notnull" json:"repository_id"`
	Repository   Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	UserID       int64      `bun:",notnull" json:"user_id"`
	User         User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Path         string     `bun:",notnull" json:"path"`
	DeletedAt    time.Time  `bun:",soft_delete,nullzero"`
	times
}

func (s *lfsLockStoreImpl) FindByID(ctx context.Context, ID int64) (*LfsLock, error) {
	var lfsLock LfsLock
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsLock).
		Relation("User").
		Where("lfs_lock.id = ?", ID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsLock, nil
}

func (s *lfsLockStoreImpl) FindByPath(ctx context.Context, RepoId int64, path string) (*LfsLock, error) {
	var lfsLock LfsLock
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsLock).
		Relation("User").
		Where("path=? and repository_id=?", path, RepoId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsLock, nil
}

func (s *lfsLockStoreImpl) FindByRepoID(ctx context.Context, RepoId int64, page, per int) ([]LfsLock, error) {
	var lfsLocks []LfsLock
	query := s.db.Operator.Core.NewSelect().
		Model(&lfsLocks).
		Relation("User").
		Where("repository_id=?", RepoId)

	if page > 0 && per > 0 {
		query = query.Limit(per).Offset((page - 1) * per)
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return lfsLocks, nil
}

func (s *lfsLockStoreImpl) Create(ctx context.Context, lfsLock LfsLock) (*LfsLock, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(&lfsLock).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsLock, nil
}

func (s *lfsLockStoreImpl) RemoveByID(ctx context.Context, ID int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&LfsLock{}).
		Where("id = ?", ID).
		ForceDelete().
		Exec(ctx)

	return err
}
