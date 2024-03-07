package database

import (
	"context"
	"fmt"
	"time"
)

type SpaceSdkStore struct {
	db *DB
}

func NewSpaceSdkStore() *SpaceSdkStore {
	return &SpaceSdkStore{db: defaultDB}
}

type SpaceSdk struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Name    string `bun:",notnull" json:"name"`
	Version string `bun:",notnull" json:"version"`
	times
}

func (s *SpaceSdkStore) Index(ctx context.Context) ([]SpaceSdk, error) {
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

func (s *SpaceSdkStore) Create(ctx context.Context, input SpaceSdk) (*SpaceSdk, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create space sdk in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *SpaceSdkStore) Update(ctx context.Context, input SpaceSdk) (*SpaceSdk, error) {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *SpaceSdkStore) Delete(ctx context.Context, input SpaceSdk) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *SpaceSdkStore) FindByID(ctx context.Context, id int64) (*SpaceSdk, error) {
	var res SpaceSdk
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)

	return &res, err
}
