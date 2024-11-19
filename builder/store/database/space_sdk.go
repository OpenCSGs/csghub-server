package database

import (
	"context"
	"fmt"
)

type spaceSdkStoreImpl struct {
	db *DB
}

type SpaceSdkStore interface {
	Index(ctx context.Context) ([]SpaceSdk, error)
	Create(ctx context.Context, input SpaceSdk) (*SpaceSdk, error)
	Update(ctx context.Context, input SpaceSdk) (*SpaceSdk, error)
	Delete(ctx context.Context, input SpaceSdk) error
	FindByID(ctx context.Context, id int64) (*SpaceSdk, error)
}

func NewSpaceSdkStore() SpaceSdkStore {
	return &spaceSdkStoreImpl{db: defaultDB}
}

type SpaceSdk struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Name    string `bun:",notnull" json:"name"`
	Version string `bun:",notnull" json:"version"`
	times
}

func (s *spaceSdkStoreImpl) Index(ctx context.Context) ([]SpaceSdk, error) {
	var result []SpaceSdk
	_, err := s.db.Operator.Core.
		NewSelect().
		Model(&result).
		Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *spaceSdkStoreImpl) Create(ctx context.Context, input SpaceSdk) (*SpaceSdk, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create space sdk in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *spaceSdkStoreImpl) Update(ctx context.Context, input SpaceSdk) (*SpaceSdk, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *spaceSdkStoreImpl) Delete(ctx context.Context, input SpaceSdk) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *spaceSdkStoreImpl) FindByID(ctx context.Context, id int64) (*SpaceSdk, error) {
	var res SpaceSdk
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)

	return &res, err
}
