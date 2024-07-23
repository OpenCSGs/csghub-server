package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

type RuntimeFrameworksStore struct {
	db *DB
}

func NewRuntimeFrameworksStore() *RuntimeFrameworksStore {
	return &RuntimeFrameworksStore{
		db: defaultDB,
	}
}

type RuntimeFramework struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	FrameName     string `bun:",notnull" json:"frame_name"`
	FrameVersion  string `bun:",notnull" json:"frame_version"`
	FrameImage    string `bun:",notnull" json:"frame_image"`
	FrameCpuImage string `bun:",notnull" json:"frame_cpu_image"`
	Enabled       int64  `bun:",notnull" json:"enabled"`
	ContainerPort int    `bun:",notnull" json:"container_port"`
	Type          int    `bun:",notnull" json:"type"` // 0-space, 1-inference, 2-finetune
	times
}

func (rf *RuntimeFrameworksStore) List(ctx context.Context, deployType int) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Where("type = ?", deployType).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rf *RuntimeFrameworksStore) ListByRepoID(ctx context.Context, repoID int64, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	err := rf.db.Operator.Core.NewSelect().Model(&RepositoriesRuntimeFramework{}).Relation("RuntimeFramework").Where("repositories_runtime_framework.type = ? and repositories_runtime_framework.repo_id = ?", deployType, repoID).Scan(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, err
}

func (rf *RuntimeFrameworksStore) FindByID(ctx context.Context, id int64) (*RuntimeFramework, error) {
	var res RuntimeFramework
	res.ID = id
	_, err := rf.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)
	return &res, err
}

func (rf *RuntimeFrameworksStore) Add(ctx context.Context, frame RuntimeFramework) error {
	res, err := rf.db.Core.NewInsert().Model(&frame).Exec(ctx, &frame)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create runtime framework in db failed", slog.String("error", err.Error()))
		return fmt.Errorf("create runtime framework in db failed,error:%w", err)
	}
	return nil
}

func (rf *RuntimeFrameworksStore) Update(ctx context.Context, frame RuntimeFramework) (*RuntimeFramework, error) {
	_, err := rf.db.Core.NewUpdate().Model(&frame).WherePK().Exec(ctx)
	return &frame, err
}

func (rf *RuntimeFrameworksStore) Delete(ctx context.Context, frame RuntimeFramework) error {
	_, err := rf.db.Core.NewDelete().Model(&frame).WherePK().Exec(ctx)
	return err
}

func (rf *RuntimeFrameworksStore) FindEnabledByID(ctx context.Context, id int64) (*RuntimeFramework, error) {
	var res RuntimeFramework
	res.ID = id
	_, err := rf.db.Core.NewSelect().Model(&res).WherePK().Where("enabled = 1").Exec(ctx, &res)
	return &res, err
}

func (rf *RuntimeFrameworksStore) FindEnabledByName(ctx context.Context, name string) (*RuntimeFramework, error) {
	var res RuntimeFramework
	_, err := rf.db.Core.NewSelect().Model(&res).Where("frame_name = ?", name).Where("enabled = 1").Exec(ctx, &res)
	return &res, err
}

func (rf *RuntimeFrameworksStore) ListAll(ctx context.Context) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rf *RuntimeFrameworksStore) ListByIDs(ctx context.Context, ids []int64) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Where("id in (?)", bun.In(ids)).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("query runtimes failed, %w", err)
	}
	return result, nil
}
