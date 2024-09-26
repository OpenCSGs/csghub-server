package database

import (
	"context"
	"fmt"
)

type RepositoriesRuntimeFrameworkStore struct {
	db *DB
}

func NewRepositoriesRuntimeFramework() *RepositoriesRuntimeFrameworkStore {
	return &RepositoriesRuntimeFrameworkStore{
		db: defaultDB,
	}
}

type RepositoriesRuntimeFramework struct {
	ID                 int64             `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64             `bun:",notnull" json:"runtime_framework_id"`
	RuntimeFramework   *RuntimeFramework `bun:"rel:belongs-to,join:runtime_framework_id=id" json:"runtime_framework"`
	RepoID             int64             `bun:",notnull" json:"repo_id"`
	Type               int               `bun:",notnull" json:"type"` // 0-space, 1-inference, 2-finetune
}

func (m *RepositoriesRuntimeFrameworkStore) ListByRuntimeFrameworkID(ctx context.Context, runtimeFrameworkID int64, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.
		NewSelect().
		Model(&result).
		Where("type = ? and runtime_framework_id = ?", deployType, runtimeFrameworkID).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *RepositoriesRuntimeFrameworkStore) Add(ctx context.Context, runtimeFrameworkID, repoID int64, deployType int) error {
	relation := RepositoriesRuntimeFramework{
		RuntimeFrameworkID: runtimeFrameworkID,
		RepoID:             repoID,
		Type:               deployType,
	}
	_, err := m.db.Operator.Core.NewInsert().Model(&relation).Exec(ctx)
	return err
}

func (m *RepositoriesRuntimeFrameworkStore) Delete(ctx context.Context, runtimeFrameworkID, repoID int64, deployType int) error {
	res, err := m.db.BunDB.Exec("delete from repositories_runtime_frameworks where type = ? and repo_id = ? and runtime_framework_id = ?", deployType, repoID, runtimeFrameworkID)
	if err != nil {
		return err
	}
	err = assertAffectedOneRow(res, err)
	return err
}

func (m *RepositoriesRuntimeFrameworkStore) DeleteByRepoID(ctx context.Context, repoID int64) error {
	_, err := m.db.Operator.Core.NewDelete().Model((*RepositoriesRuntimeFramework)(nil)).Where("repo_id = ?", repoID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete repo runtime failed, %w", err)
	}
	return nil
}

func (m *RepositoriesRuntimeFrameworkStore) GetByIDsAndType(ctx context.Context, runtimeFrameworkID, repoID int64, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.NewSelect().Model(&result).Where("type = ? and repo_id=? and runtime_framework_id = ?", deployType, repoID, runtimeFrameworkID).Exec(ctx, &result)
	return result, err
}

func (m *RepositoriesRuntimeFrameworkStore) ListRepoIDsByType(ctx context.Context, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.NewSelect().Model(&result).Where("type = ?", deployType).Exec(ctx, &result)
	return result, err
}

func (m *RepositoriesRuntimeFrameworkStore) GetByRepoIDsAndType(ctx context.Context, repoID int64, deployType int) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.NewSelect().Model(&result).Where("type = ? and repo_id=?", deployType, repoID).Exec(ctx, &result)
	return result, err
}

func (m *RepositoriesRuntimeFrameworkStore) GetByRepoIDs(ctx context.Context, repoID int64) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.NewSelect().Model(&result).Where("repo_id=?", repoID).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("get runtime by repoid failed, %w", err)
	}
	return result, nil
}
