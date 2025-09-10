package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type imageBuilderWorkStoreImpl struct {
	db *DB
}

type ImageBuilderWorkStore interface {
	Create(ctx context.Context, imageBuilder *ImageBuilderWork) (*ImageBuilderWork, error)
	CreateOrUpdateByBuildID(ctx context.Context, imageBuilder *ImageBuilderWork) (*ImageBuilderWork, error)
	FindByWorkName(ctx context.Context, workName string) (*ImageBuilderWork, error)
	QueryStatusByBuildID(ctx context.Context, buildId string) (*ImageBuilderWork, error)
	QueryByBuildID(ctx context.Context, buildId string) (*ImageBuilderWork, error)
	UpdateByWorkName(ctx context.Context, work *ImageBuilderWork) (*ImageBuilderWork, error)
	FindByImagePath(ctx context.Context, imagePath string) (*ImageBuilderWork, error)
}

func NewImageBuilderStore() ImageBuilderWorkStore {
	return &imageBuilderWorkStoreImpl{
		db: defaultDB,
	}
}

func NewImageBuilderStoreWithDB(db *DB) ImageBuilderWorkStore {
	return &imageBuilderWorkStoreImpl{
		db: db,
	}
}

func (i *imageBuilderWorkStoreImpl) CreateOrUpdateByBuildID(ctx context.Context, work *ImageBuilderWork) (*ImageBuilderWork, error) {
	ibw := ImageBuilderWork{}
	err := i.db.Operator.Core.NewSelect().Model(&ibw).Where("build_id = ?", work.BuildId).Scan(ctx, &ibw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return i.Create(ctx, work)
	}
	work.ID = ibw.ID
	_, err = i.db.Core.NewUpdate().Model(work).WherePK().Exec(ctx)

	return work, err
}

func (i *imageBuilderWorkStoreImpl) Create(ctx context.Context, work *ImageBuilderWork) (*ImageBuilderWork, error) {
	res, err := i.db.Core.NewInsert().Model(work).Exec(ctx, work)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to save imageBuilder in db, error:%w", err)
	}

	return work, nil
}

func (i *imageBuilderWorkStoreImpl) FindByWorkName(ctx context.Context, workName string) (*ImageBuilderWork, error) {
	ibw := ImageBuilderWork{}
	err := i.db.Operator.Core.NewSelect().Model(&ibw).Where("work_name = ?", workName).Scan(ctx, &ibw)
	if err != nil {
		return nil, err
	}
	return &ibw, nil
}

func (i *imageBuilderWorkStoreImpl) UpdateByWorkName(ctx context.Context, work *ImageBuilderWork) (*ImageBuilderWork, error) {
	w, err := i.FindByWorkName(ctx, work.WorkName)
	if err != nil {
		return nil, err
	}

	work.ID = w.ID
	_, err = i.db.Core.NewUpdate().Model(work).WherePK().Exec(ctx)
	return work, err
}

func (i *imageBuilderWorkStoreImpl) QueryStatusByBuildID(ctx context.Context, buildId string) (*ImageBuilderWork, error) {
	ibw := ImageBuilderWork{}
	err := i.db.Operator.Core.NewSelect().Model(&ibw).Where("build_id = ?", buildId).Scan(ctx, &ibw)
	if err != nil {
		return nil, err
	}
	return &ibw, nil
}

func (i *imageBuilderWorkStoreImpl) QueryByBuildID(ctx context.Context, buildId string) (*ImageBuilderWork, error) {
	ibw := ImageBuilderWork{}
	err := i.db.Operator.Core.NewSelect().Model(&ibw).Where("build_id = ?", buildId).Scan(ctx, &ibw)
	if err != nil {
		return nil, err
	}
	return &ibw, nil
}

func (i *imageBuilderWorkStoreImpl) FindByImagePath(ctx context.Context, imagePath string) (*ImageBuilderWork, error) {
	ibw := ImageBuilderWork{}
	err := i.db.Operator.Core.NewSelect().Model(&ibw).Where("image_path = ?", imagePath).Scan(ctx, &ibw)
	if err != nil {
		return nil, err
	}
	return &ibw, nil
}

type ImageBuilderWork struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`

	WorkName   string `bun:"work_name,notnull,unique" json:"work_name"`
	WorkStatus string `bun:"work_status,notnull" json:"work_status"`
	Message    string `bun:"message" json:"message"`
	PodName    string `bun:"pod_name" json:"pod_name"`
	ClusterID  string `bun:"cluster_id" json:"cluster_id"`
	Namespace  string `bun:"namespace,notnull" json:"namespace"`
	ImagePath  string `bun:"image_path,notnull" json:"image_path"`
	BuildId    string `bun:"build_id,notnull,unique" json:"build_id"`

	InitContainerStatus string `bun:"init_container_status,notnull" json:"init_container_status"`
	InitContainerLog    string `bun:"init_container_log" json:"init_container_log"`
	MainContainerLog    string `bun:"main_container_log" json:"main_container_log"`

	times
}
