package database

import (
	"context"
	"fmt"
	"time"
)

type LfsMetaObjectStore struct {
	db *DB
}

func NewLfsMetaObjectStore() *LfsMetaObjectStore {
	return &LfsMetaObjectStore{
		db: defaultDB,
	}
}

type LfsMetaObject struct {
	ID           int64      `bun:",pk,autoincrement" json:"user_id"`
	Oid          string     `bun:",notnull" json:"oid"`
	Size         int64      `bun:",notnull" json:"size"`
	RepositoryID int64      `bun:",notnull" json:"repository_id"`
	Repository   Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	Existing     bool       `bun:",notnull" json:"existing"`
	times
}

func (s *LfsMetaObjectStore) FindByOID(ctx context.Context, RepoId int64, Oid string) (*LfsMetaObject, error) {
	var lfsMetaObject LfsMetaObject
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsMetaObject).
		Where("oid=? and repository_id=?", Oid, RepoId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsMetaObject, nil
}

func (s *LfsMetaObjectStore) FindByRepoID(ctx context.Context, repoID int64) ([]LfsMetaObject, error) {
	var lfsMetaObjects []LfsMetaObject
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsMetaObjects).
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return lfsMetaObjects, nil
}

func (s *LfsMetaObjectStore) Create(ctx context.Context, lfsObj LfsMetaObject) (*LfsMetaObject, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(&lfsObj).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsObj, nil
}

func (s *LfsMetaObjectStore) RemoveByOid(ctx context.Context, oid string, repoID int64) error {
	err := s.db.Operator.Core.NewDelete().
		Model(&LfsMetaObject{}).
		Where("oid = ? and repository_id= ?", oid, repoID).
		Scan(ctx)

	return err
}

func (s *LfsMetaObjectStore) UpdateOrCreate(ctx context.Context, input LfsMetaObject) (*LfsMetaObject, error) {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().
		Model(&input).
		Where("oid = ? and repository_id = ?", input.Oid, input.RepositoryID).
		Returning("*").
		Exec(ctx, &input)
	if err == nil {
		return &input, nil
	}

	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create lfs meta object in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *LfsMetaObjectStore) BulkUpdateOrCreate(ctx context.Context, input []LfsMetaObject) error {
	_, err := s.db.Core.NewInsert().
		Model(&input).
		On("CONFLICT (oid, repository_id) DO UPDATE").
		Set("size = EXCLUDED.size, updated_at = EXCLUDED.updated_at, existing = EXCLUDED.existing").
		Exec(ctx)
	return err
}
