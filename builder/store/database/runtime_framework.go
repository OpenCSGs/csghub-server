package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

type runtimeFrameworksStoreImpl struct {
	db *DB
}

type RuntimeFrameworksStore interface {
	List(ctx context.Context, deployType int) ([]RuntimeFramework, error)
	ListByArchsNameAndType(ctx context.Context, name, format string, archs []string, deployType int) ([]RuntimeFramework, error)
	FindByID(ctx context.Context, id int64) (*RuntimeFramework, error)
	Add(ctx context.Context, frame RuntimeFramework) (*RuntimeFramework, error)
	Update(ctx context.Context, frame RuntimeFramework) (*RuntimeFramework, error)
	Delete(ctx context.Context, frame RuntimeFramework) error
	RemoveRuntimeFrameworkAndArch(ctx context.Context, id int64) error
	FindEnabledByID(ctx context.Context, id int64) (*RuntimeFramework, error)
	FindEnabledByName(ctx context.Context, name string) (*RuntimeFramework, error)
	FindByFrameName(ctx context.Context, name string) ([]RuntimeFramework, error)
	FindByFrameImageAndComputeType(ctx context.Context, frameImage, ComputeType string) (*RuntimeFramework, error)
	ListAll(ctx context.Context) ([]RuntimeFramework, error)
	ListByIDs(ctx context.Context, ids []int64) ([]RuntimeFramework, error)
}

func NewRuntimeFrameworksStore() RuntimeFrameworksStore {
	return &runtimeFrameworksStoreImpl{
		db: defaultDB,
	}
}

func NewRuntimeFrameworksStoreWithDB(db *DB) RuntimeFrameworksStore {
	return &runtimeFrameworksStoreImpl{
		db: db,
	}
}

type RuntimeFramework struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	FrameName     string `bun:",notnull" json:"frame_name"`
	FrameVersion  string `bun:",notnull" json:"frame_version"`
	FrameImage    string `bun:",notnull" json:"frame_image"`
	ComputeType   string `bun:",notnull" json:"compute_type"` // cpu, gpu,npu,mlu
	DriverVersion string `bun:"," json:"driver_version"`      //12.1
	Description   string `bun:"," json:"description"`
	Enabled       int64  `bun:",notnull" json:"enabled"`
	ContainerPort int    `bun:",notnull" json:"container_port"`
	Type          int    `bun:",notnull" json:"type"` // 0-space, 1-inference, 2-finetune
	EngineArgs    string `bun:",nullzero" json:"engine_args"`
	ModelFormat   string `bun:",nullzero" json:"model_format"` // safetensors, gguf, or onnx
	times
}

func (rf *runtimeFrameworksStoreImpl) List(ctx context.Context, deployType int) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Where("type = ?", deployType).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rf *runtimeFrameworksStoreImpl) ListByRepoID(ctx context.Context, repoID int64, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	err := rf.db.Operator.Core.NewSelect().Model(&RepositoriesRuntimeFramework{}).Relation("RuntimeFramework").Where("repositories_runtime_framework.type = ? and repositories_runtime_framework.repo_id = ?", deployType, repoID).Scan(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, err
}

func (rf *runtimeFrameworksStoreImpl) FindByID(ctx context.Context, id int64) (*RuntimeFramework, error) {
	var res RuntimeFramework
	res.ID = id
	_, err := rf.db.Core.NewSelect().Model(&res).WherePK().Exec(ctx, &res)
	return &res, err
}

func (rf *runtimeFrameworksStoreImpl) Add(ctx context.Context, frame RuntimeFramework) (*RuntimeFramework, error) {
	res, err := rf.db.Core.NewInsert().Model(&frame).Exec(ctx, &frame)
	if err := assertAffectedOneRow(res, err); err != nil {
		slog.Error("create runtime framework in db failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create runtime framework in db failed,error:%w", err)
	}
	return &frame, nil
}

func (rf *runtimeFrameworksStoreImpl) Update(ctx context.Context, frame RuntimeFramework) (*RuntimeFramework, error) {
	_, err := rf.db.Core.NewUpdate().Model(&frame).WherePK().Exec(ctx)
	return &frame, err
}

func (rf *runtimeFrameworksStoreImpl) Delete(ctx context.Context, frame RuntimeFramework) error {
	_, err := rf.db.Core.NewDelete().Model(&frame).WherePK().Exec(ctx)
	return err
}

func (rf *runtimeFrameworksStoreImpl) FindEnabledByID(ctx context.Context, id int64) (*RuntimeFramework, error) {
	var res RuntimeFramework
	res.ID = id
	_, err := rf.db.Core.NewSelect().Model(&res).WherePK().Where("enabled = 1").Exec(ctx, &res)
	return &res, err
}

func (rf *runtimeFrameworksStoreImpl) FindEnabledByName(ctx context.Context, name string) (*RuntimeFramework, error) {
	var res RuntimeFramework
	_, err := rf.db.Core.NewSelect().Model(&res).Where("LOWER(frame_name) = LOWER(?)", name).Where("enabled = 1").Exec(ctx, &res)
	return &res, err
}

func (rf *runtimeFrameworksStoreImpl) ListAll(ctx context.Context) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rf *runtimeFrameworksStoreImpl) ListByIDs(ctx context.Context, ids []int64) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Operator.Core.NewSelect().Model(&result).Where("id in (?)", bun.In(ids)).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("query runtimes failed, %w", err)
	}
	return result, nil
}

// ListByArchsNameAndType
func (rf *runtimeFrameworksStoreImpl) ListByArchsNameAndType(ctx context.Context, name, format string, archs []string, deployType int) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	var ras []RuntimeArchitecture
	_, err := rf.db.Operator.Core.
		NewSelect().Model(&ras).
		Relation("RuntimeFramework", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("model_format = ?", format).Where("type = ?", deployType)
		}).
		Where("architecture_name in (?) or model_name=?", bun.In(archs), name).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	for _, ra := range ras {
		result = append(result, *ra.RuntimeFramework)
	}
	return result, nil
}

// FindByNameAndComputeType
func (rf *runtimeFrameworksStoreImpl) FindByNameAndComputeType(ctx context.Context, frameName, driverVersion, computeType string) (*RuntimeFramework, error) {
	var res RuntimeFramework
	_, err := rf.db.Core.NewSelect().Model(&res).Where("LOWER(frame_name) = LOWER(?)", frameName).Where("compute_type = ?", computeType).Where("driver_version = ?", driverVersion).Exec(ctx, &res)
	return &res, err
}

func (rf *runtimeFrameworksStoreImpl) RemoveRuntimeFrameworkAndArch(ctx context.Context, rfId int64) error {
	return rf.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// First, delete runtime architectures associated with the framework
		_, err := tx.NewDelete().Model(&RuntimeArchitecture{}).Where("runtime_framework_id = ?", rfId).Exec(ctx)
		if err != nil {
			return fmt.Errorf("error happened while removing runtime architecture, %w", err)
		}

		// Then, delete the runtime framework itself
		_, err = tx.NewDelete().Model(&RuntimeFramework{}).Where("id = ?", rfId).Exec(ctx)
		if err != nil {
			return fmt.Errorf("error happened while removing runtime framework, %w", err)
		}

		return nil
	})
}

func (rf *runtimeFrameworksStoreImpl) FindByFrameName(ctx context.Context, name string) ([]RuntimeFramework, error) {
	var result []RuntimeFramework
	_, err := rf.db.Core.NewSelect().Model(&result).Where("frame_name = ?", name).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
