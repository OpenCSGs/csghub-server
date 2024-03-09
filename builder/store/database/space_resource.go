package database

import (
	"context"
	"fmt"
	"time"
)

type SpaceResourceStore struct {
	db *DB
}

func NewSpaceResourceStore() *SpaceResourceStore {
	return &SpaceResourceStore{db: defaultDB}
}

type SpaceResource struct {
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	Name   string `bun:",notnull" json:"name"`
	Cpu    int    `bun:",notnull" json:"cpu"`
	Gpu    int    `bun:",notnull" json:"gpu"`
	Memory int    `bun:",notnull" json:"memory"`
	Disk   int    `bun:",notnull" json:"disk"`
	times
}

func (s *SpaceResourceStore) Index(ctx context.Context) ([]SpaceResource, error) {
	var result []SpaceResource
	_, err := s.db.Operator.Core.
		NewSelect().
		Model(&result).
		Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SpaceResourceStore) Create(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create space resource in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *SpaceResourceStore) Update(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *SpaceResourceStore) Delete(ctx context.Context, input SpaceResource) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *SpaceResourceStore) FindByID(ctx context.Context, id int64) (*SpaceResource, error) {
	var res SpaceResource
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)

	return &res, err
}
