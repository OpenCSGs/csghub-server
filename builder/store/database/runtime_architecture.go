package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

type runtimeArchitecturesStoreImpl struct {
	db *DB
}

type RuntimeArchitecturesStore interface {
	ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]RuntimeArchitecture, error)
	Add(ctx context.Context, arch RuntimeArchitecture) error
	BatchAdd(ctx context.Context, archs []RuntimeArchitecture) error
	DeleteByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) error
	DeleteByRuntimeID(ctx context.Context, id int64) error
	FindByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) (*RuntimeArchitecture, error)
	ListByRArchName(ctx context.Context, archName string) ([]RuntimeArchitecture, error)
	ListByArchNameAndModel(ctx context.Context, archs []string, modelName string) ([]RuntimeArchitecture, error)
	CheckEngineByArchModelNameAndType(ctx context.Context, archs []string, modelName, format string, deployType int) (bool, error)
}

func NewRuntimeArchitecturesStore() RuntimeArchitecturesStore {
	return &runtimeArchitecturesStoreImpl{
		db: defaultDB,
	}
}

func NewRuntimeArchitecturesStoreWithDB(db *DB) RuntimeArchitecturesStore {
	return &runtimeArchitecturesStoreImpl{
		db: db,
	}
}

type RuntimeArchitecture struct {
	ID                 int64             `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64             `bun:",notnull" json:"runtime_framework_id"`
	RuntimeFramework   *RuntimeFramework `bun:"rel:belongs-to,join:runtime_framework_id=id" json:"runtime_framework"`
	ArchitectureName   string            `bun:",nullzero" json:"architecture_name"`
	// some engine has specific model names, like ms-swift,mindie
	ModelName string `bun:",nullzero" json:"model_name"`
}

func (ra *runtimeArchitecturesStoreImpl) ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("runtime_framework_id = ?", id).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

func (ra *runtimeArchitecturesStoreImpl) Add(ctx context.Context, arch RuntimeArchitecture) error {
	res, err := ra.db.Core.NewInsert().Model(&arch).Exec(ctx, &arch)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("creating runtime architecture in the db failed,error:%w", err)
	}
	return nil
}

func (ra *runtimeArchitecturesStoreImpl) DeleteByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) error {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewDelete().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting runtime architecture in the db failed, error:%w", err)
	}
	return nil
}

func (ra *runtimeArchitecturesStoreImpl) FindByRuntimeIDAndArchName(ctx context.Context, id int64, archName string) (*RuntimeArchitecture, error) {
	var arch RuntimeArchitecture
	_, err := ra.db.Core.NewSelect().Model(&arch).Where("runtime_framework_id = ? and architecture_name = ?", id, archName).Exec(ctx, &arch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting runtime architecture in the db failed, error:%w", err)
	}
	return &arch, nil
}

func (ra *runtimeArchitecturesStoreImpl) ListByRArchName(ctx context.Context, archName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("architecture_name = ?", archName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

func (ra *runtimeArchitecturesStoreImpl) ListByArchNameAndModel(ctx context.Context, archNames []string, modelName string) ([]RuntimeArchitecture, error) {
	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.NewSelect().Model(&result).Where("architecture_name in (?) or model_name=?", bun.In(archNames), modelName).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("error happened while getting runtime architecture, %w", err)
	}
	return result, nil
}

// ListByRArchsAndModelFormat
// add RuntimeFramework as relation
func (ra *runtimeArchitecturesStoreImpl) CheckEngineByArchModelNameAndType(ctx context.Context, archs []string, name, modelFormat string, deployType int) (bool, error) {

	var result []RuntimeArchitecture
	_, err := ra.db.Operator.Core.
		NewSelect().Model(&result).
		Relation("RuntimeFramework", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("model_format = ?", modelFormat).Where("type = ?", deployType)
		}).
		Where("architecture_name in (?) or model_name=?", bun.In(archs), name).Exec(ctx, &result)
	if err != nil {
		return false, fmt.Errorf("error happened while checking runtime architecture, %w", err)
	}
	return len(result) > 0, nil
}

// DeleteByRuntimeID
func (ra *runtimeArchitecturesStoreImpl) DeleteByRuntimeID(ctx context.Context, runtimeFrameworkID int64) error {
	_, err := ra.db.Core.NewDelete().Model((*RuntimeArchitecture)(nil)).Where("runtime_framework_id = ?", runtimeFrameworkID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error happened while deleting runtime architecture by runtime framework id %d, %w", runtimeFrameworkID, err)
	}
	return nil
}

// batchadd
func (ra *runtimeArchitecturesStoreImpl) BatchAdd(ctx context.Context, architectures []RuntimeArchitecture) error {
	_, err := ra.db.Core.NewInsert().Model(&architectures).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error happened while adding runtime architecture %v, %w", architectures, err)
	}
	return nil
}
