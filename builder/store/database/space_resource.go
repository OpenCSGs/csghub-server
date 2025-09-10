package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
)

type spaceResourceStoreImpl struct {
	db *DB
}

type SpaceResourceStore interface {
	Index(ctx context.Context, clusterId string, per, page int) ([]SpaceResource, int, error)
	Create(ctx context.Context, input SpaceResource) (*SpaceResource, error)
	Update(ctx context.Context, input SpaceResource) (*SpaceResource, error)
	Delete(ctx context.Context, input SpaceResource) error
	FindByID(ctx context.Context, id int64) (*SpaceResource, error)
	FindByName(ctx context.Context, name string) (*SpaceResource, error)
	FindAll(ctx context.Context) ([]SpaceResource, error)
}

func NewSpaceResourceStore() SpaceResourceStore {
	return &spaceResourceStoreImpl{db: defaultDB}
}

func NewSpaceResourceStoreWithDB(db *DB) SpaceResourceStore {
	return &spaceResourceStoreImpl{db: db}
}

type SpaceResource struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	Name      string `bun:",notnull" json:"name"`
	Resources string `bun:",notnull" json:"resources"`
	ClusterID string `bun:",notnull" json:"cluster_id"`
	times
}

func (s *spaceResourceStoreImpl) Index(ctx context.Context, clusterId string, per, page int) ([]SpaceResource, int, error) {
	var result []SpaceResource
	query := s.db.Operator.Core.NewSelect().Model(&result).Where("cluster_id = ?", clusterId)
	query = query.Order("name asc").
		Limit(per).
		Offset((page - 1) * per)
	err := query.Scan(ctx, &result)
	err = errorx.HandleDBError(err, nil)
	if err != nil {
		return nil, 0, err
	}
	total, err := query.Count(ctx)
	err = errorx.HandleDBError(err, nil)
	return result, total, err
}

func (s *spaceResourceStoreImpl) Create(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create space resource in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *spaceResourceStoreImpl) Update(ctx context.Context, input SpaceResource) (*SpaceResource, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *spaceResourceStoreImpl) Delete(ctx context.Context, input SpaceResource) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *spaceResourceStoreImpl) FindByID(ctx context.Context, id int64) (*SpaceResource, error) {
	var res SpaceResource
	res.ID = id
	_, err := s.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)

	return &res, err
}

func (s *spaceResourceStoreImpl) FindByName(ctx context.Context, name string) (*SpaceResource, error) {
	var res SpaceResource
	err := s.db.Core.NewSelect().Model(&res).Where("name = ?", name).Scan(ctx)

	return &res, err
}

func (s *spaceResourceStoreImpl) FindAll(ctx context.Context) ([]SpaceResource, error) {
	var result []SpaceResource
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
