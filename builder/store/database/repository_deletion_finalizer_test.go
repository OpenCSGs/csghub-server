package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func createRepositoryFinalizerExtraTables(t *testing.T, db *DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE deploys (id INTEGER PRIMARY KEY, repo_id INTEGER)`,
		`CREATE TABLE llm_configs (id INTEGER PRIMARY KEY, repo_id INTEGER)`,
		`CREATE TABLE federation_adapter_sync_records (id INTEGER PRIMARY KEY, user_uuid TEXT, username TEXT, site_id TEXT, repo_type TEXT, source_url TEXT, source_path TEXT, fork_path TEXT, mirror_id INTEGER, repo_id INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	} {
		_, err := db.Core.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
}

func TestRepositoryDeletionFinalizerDeletesRelationsAndRepositoryIdempotently(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	createRepositoryFinalizerExtraTables(t, db)
	ctx := context.Background()
	repo := Repository{UserID: 1, Name: "repo", Path: "org/repo", GitPath: "models_org/repo", RepositoryType: types.ModelRepo}
	_, err := db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, "INSERT INTO user_likes (id, repo_id) VALUES (?, ?)", 1, repo.ID)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&Model{RepositoryID: repo.ID}).Exec(ctx)
	require.NoError(t, err)
	for _, statement := range []string{
		"INSERT INTO deploys (id, repo_id) VALUES (1, ?)",
		"INSERT INTO llm_configs (id, repo_id) VALUES (1, ?)",
	} {
		_, err = db.Core.ExecContext(ctx, statement, repo.ID)
		require.NoError(t, err)
	}

	finalizer := NewRepositoryDeletionFinalizerWithDB(db)
	require.NoError(t, finalizer.FinalizeRepositoryDeletion(ctx, repo.ID))
	require.NoError(t, finalizer.FinalizeRepositoryDeletion(ctx, repo.ID))

	count, err := db.Core.NewSelect().Table("user_likes").Where("repo_id = ?", repo.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	for _, table := range []string{"deploys", "llm_configs"} {
		linked, err := db.Core.NewSelect().Table(table).Where("repo_id = ?", repo.ID).Count(ctx)
		require.NoError(t, err)
		require.Zero(t, linked, table)
		count, err = db.Core.NewSelect().Table(table).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count, table)
	}
	count, err = db.Core.NewSelect().Model((*Repository)(nil)).Where("id = ?", repo.ID).WhereAllWithDeleted().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}
