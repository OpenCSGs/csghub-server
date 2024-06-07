package database

import (
	"context"
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
}

func (m *RepositoriesRuntimeFrameworkStore) ListByRuntimeFrameworkID(ctx context.Context, runtimeFrameworkID int64) ([]RepositoriesRuntimeFramework, error) {
	var result []RepositoriesRuntimeFramework
	_, err := m.db.Operator.Core.
		NewSelect().
		Model(&result).
		Where("runtime_framework_id = ?", runtimeFrameworkID).Exec(ctx, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *RepositoriesRuntimeFrameworkStore) Add(ctx context.Context, runtimeFrameworkID, repoID int64) error {
	relation := RepositoriesRuntimeFramework{
		RuntimeFrameworkID: runtimeFrameworkID,
		RepoID:             repoID,
	}
	_, err := m.db.Operator.Core.NewInsert().Model(&relation).Exec(ctx)
	return err
}

func (m *RepositoriesRuntimeFrameworkStore) Delete(ctx context.Context, runtimeFrameworkID, repoID int64) error {
	res, err := m.db.BunDB.Exec("delete from repositories_runtime_frameworks where runtime_framework_id = ? and repo_id = ?", runtimeFrameworkID, repoID)
	if err != nil {
		return err
	}
	err = assertAffectedOneRow(res, err)
	return err
}
