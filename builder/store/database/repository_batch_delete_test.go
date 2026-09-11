package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"opencsg.com/csghub-server/common/types"
)

type recordingRepositoryDeletionJobClient struct {
	inputs []RepositoryDeletionJobInput
	err    error
}

const testOrganizationUUID = "44444444-4444-4444-4444-444444444444"

func insertTestOrganizationNamespace(t *testing.T, db *DB, path string) Namespace {
	t.Helper()
	namespace := Namespace{Path: path, NamespaceType: OrgNamespace, UUID: "namespace-uuid"}
	_, err := db.Core.NewInsert().Model(&namespace).Exec(context.Background())
	require.NoError(t, err)
	organization := Organization{Name: path, NamespaceID: namespace.ID, UUID: uuid.MustParse(testOrganizationUUID)}
	_, err = db.Core.NewInsert().Model(&organization).Exec(context.Background())
	require.NoError(t, err)
	return namespace
}

func insertTestUserNamespace(t *testing.T, db *DB, path, userUUID string) Namespace {
	t.Helper()
	user := User{Username: path, UUID: userUUID}
	_, err := db.Core.NewInsert().Model(&user).Exec(context.Background())
	require.NoError(t, err)
	namespace := Namespace{Path: path, UserID: user.ID, NamespaceType: UserNamespace, UUID: "namespace-uuid"}
	_, err = db.Core.NewInsert().Model(&namespace).Exec(context.Background())
	require.NoError(t, err)
	return namespace
}

func (c *recordingRepositoryDeletionJobClient) InsertRepositoryDeletionJobTx(_ context.Context, _ *sql.Tx, input RepositoryDeletionJobInput) (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	c.inputs = append(c.inputs, input)
	return int64(len(c.inputs)), nil
}

func newRepositoryBatchDeleteTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(context.Background(), DBConfig{Dialect: DialectSQLite, DSN: "file::memory:?cache=shared"})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for _, model := range []any{
		(*Repository)(nil), (*Model)(nil), (*Dataset)(nil), (*Code)(nil), (*Space)(nil),
		(*Prompt)(nil), (*MCPServer)(nil), (*Skill)(nil), (*RepositoriesRuntimeFramework)(nil),
		(*PendingDeletion)(nil), (*Namespace)(nil), (*User)(nil), (*Organization)(nil),
	} {
		_, err = db.Core.NewCreateTable().Model(model).IfNotExists().Exec(context.Background())
		require.NoError(t, err)
	}
	for _, statement := range []string{
		`CREATE TABLE user_likes (id INTEGER PRIMARY KEY, repo_id INTEGER)`,
		`CREATE TABLE lfs_meta_objects (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE mirrors (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE mirror_tasks (id INTEGER PRIMARY KEY, mirror_id INTEGER)`,
		`CREATE TABLE files (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repository_tags (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repository_downloads (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE metadata (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repository_statistics (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE lfs_files (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE lfs_locks (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repository_files (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repository_file_checks (id INTEGER PRIMARY KEY, repo_file_id INTEGER)`,
		`CREATE TABLE collection_repositories (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE repo_relations (id INTEGER PRIMARY KEY, from_repo_id INTEGER, to_repo_id INTEGER)`,
		`CREATE TABLE dataviewers (id INTEGER PRIMARY KEY, repo_id INTEGER)`,
		`CREATE TABLE dataviewer_jobs (id INTEGER PRIMARY KEY, repo_id INTEGER)`,
		`CREATE TABLE model_trees (id INTEGER PRIMARY KEY, source_repo_id INTEGER, target_repo_id INTEGER)`,
		`CREATE TABLE mcp_scan_results (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE xnet_migration_tasks (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE recom_op_weights (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
		`CREATE TABLE recom_repo_scores (id INTEGER PRIMARY KEY, repository_id INTEGER)`,
	} {
		_, err = db.Core.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
	return db
}

func TestDeleteRepositoriesByIDsUsesOwnerEntityUUID(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()

	user := User{Username: "person", UUID: "user-entity-uuid"}
	_, err := db.Core.NewInsert().Model(&user).Exec(ctx)
	require.NoError(t, err)
	userNamespace := Namespace{Path: "person", UserID: user.ID, NamespaceType: UserNamespace, UUID: "user-namespace-uuid"}
	_, err = db.Core.NewInsert().Model(&userNamespace).Exec(ctx)
	require.NoError(t, err)

	legacyNamespace := Namespace{Path: "legacy", NamespaceType: OrgNamespace, UUID: "legacy-namespace-uuid"}
	hierarchyNamespace := Namespace{Path: "root/department", NamespaceType: OrgNamespace, UUID: "hierarchy-namespace-uuid"}
	namespaces := []Namespace{legacyNamespace, hierarchyNamespace}
	_, err = db.Core.NewInsert().Model(&namespaces).Exec(ctx)
	require.NoError(t, err)
	legacyNamespaceID := namespaces[0].ID
	hierarchyNamespaceID := namespaces[1].ID
	legacyUUID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	hierarchyUUID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	organizations := []Organization{
		{Name: "legacy", NamespaceID: legacyNamespaceID, UUID: legacyUUID},
		{Name: "root/department", NamespaceID: hierarchyNamespaceID, UUID: hierarchyUUID, IsHierarchical: true},
	}
	_, err = db.Core.NewInsert().Model(&organizations).Exec(ctx)
	require.NoError(t, err)

	// A deleted namespace with the same path must not win merely because rows are
	// keyed by path. Its deleted organization is tied to that namespace by ID.
	tombstoneNamespace := Namespace{Path: "legacy", NamespaceType: OrgNamespace, UUID: "tombstone-namespace-uuid", DeletedAt: time.Now()}
	_, err = db.Core.NewInsert().Model(&tombstoneNamespace).Exec(ctx)
	require.NoError(t, err)
	tombstoneOrg := Organization{
		Name: "legacy", NamespaceID: tombstoneNamespace.ID,
		UUID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), DeletedAt: time.Now(),
	}
	_, err = db.Core.NewInsert().Model(&tombstoneOrg).Exec(ctx)
	require.NoError(t, err)

	repositories := []Repository{
		{UserID: user.ID, Name: "personal", Path: "person/personal", GitPath: "models_person/personal", RepositoryType: types.ModelRepo},
		{UserID: user.ID, Name: "legacy", Path: "legacy/repo", GitPath: "models_legacy/repo", RepositoryType: types.ModelRepo},
		{UserID: user.ID, Name: "hierarchy", Path: "root/department/repo", GitPath: "models_root/department/repo", RepositoryType: types.ModelRepo},
	}
	_, err = db.Core.NewInsert().Model(&repositories).Exec(ctx)
	require.NoError(t, err)
	jobClient := &recordingRepositoryDeletionJobClient{}

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := deleteRepositoriesByIDs(ctx, tx, []int64{repositories[0].ID, repositories[1].ID, repositories[2].ID}, jobClient)
		return err
	})
	require.NoError(t, err)
	require.Len(t, jobClient.inputs, 3)
	require.Equal(t, UserNamespace, jobClient.inputs[0].OwnerType)
	require.Equal(t, user.UUID, jobClient.inputs[0].OwnerUUID)
	require.Equal(t, OrgNamespace, jobClient.inputs[1].OwnerType)
	require.Equal(t, legacyUUID.String(), jobClient.inputs[1].OwnerUUID)
	require.Equal(t, OrgNamespace, jobClient.inputs[2].OwnerType)
	require.Equal(t, hierarchyUUID.String(), jobClient.inputs[2].OwnerUUID)
}

func TestFindRepositoryIDsByNamespaces(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()

	repos := []Repository{
		{UserID: 1, Name: "one", Path: "team/repo", GitPath: "models_team/repo", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "two", Path: "team-child/repo", GitPath: "models_team-child/repo", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "three", Path: "percent%org/repo", GitPath: "models_percent%org/repo", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "four", Path: "percentXorg/repo", GitPath: "models_percentXorg/repo", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "five", Path: "under_score/repo", GitPath: "models_under_score/repo", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "six", Path: "underXscore/repo", GitPath: "models_underXscore/repo", RepositoryType: types.ModelRepo},
	}
	_, err := db.Core.NewInsert().Model(&repos).Exec(ctx)
	require.NoError(t, err)

	ids, err := findRepositoryIDsByNamespaces(ctx, db.Core, []string{"team", "percent%org", "under_score"})
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{repos[0].ID, repos[2].ID, repos[4].ID}, ids)

	ids, err = findRepositoryIDsByNamespaces(ctx, db.Core, nil)
	require.NoError(t, err)
	require.Empty(t, ids)
}

func TestDeleteRepositoriesByIDs(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	jobClient := &recordingRepositoryDeletionJobClient{}
	insertTestOrganizationNamespace(t, db, "org")
	var err error
	_, err = db.Core.NewInsert().Model(&Namespace{Path: "other", NamespaceType: UserNamespace, UUID: "user-uuid"}).Exec(ctx)
	require.NoError(t, err)

	repos := []Repository{
		{UserID: 1, Name: "model", Path: "org/model", GitPath: "models_org/model", RepositoryType: types.ModelRepo},
		{UserID: 1, Name: "dataset", Path: "org/dataset", GitPath: "datasets_org/dataset", RepositoryType: types.DatasetRepo, Hashed: true},
		{UserID: 1, Name: "code", Path: "org/code", GitPath: "codes_org/code", RepositoryType: types.CodeRepo},
		{UserID: 1, Name: "space", Path: "org/space", GitPath: "spaces_org/space", RepositoryType: types.SpaceRepo},
		{UserID: 1, Name: "prompt", Path: "org/prompt", GitPath: "prompts_org/prompt", RepositoryType: types.PromptRepo},
		{UserID: 1, Name: "mcp", Path: "org/mcp", GitPath: "mcpservers_org/mcp", RepositoryType: types.MCPServerRepo},
		{UserID: 1, Name: "skill", Path: "org/skill", GitPath: "skills_org/skill", RepositoryType: types.SkillRepo},
		{UserID: 1, Name: "keep", Path: "other/keep", GitPath: "models_other/keep", RepositoryType: types.ModelRepo},
	}
	_, err = db.Core.NewInsert().Model(&repos).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Model{{RepositoryID: repos[0].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx,
		`INSERT INTO datasets (repository_id, last_updated_at, dataset_type, status) VALUES (?, ?, ?, ?)`,
		repos[1].ID, "2026-08-27T00:00:00Z", types.DatasetTypeNormal, types.DatasetStatusNormal)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Code{{RepositoryID: repos[2].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Space{{RepositoryID: repos[3].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Prompt{{RepositoryID: repos[4].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]MCPServer{{RepositoryID: repos[5].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Skill{{RepositoryID: repos[6].ID}}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]Model{{RepositoryID: repos[7].ID}}).Exec(ctx)
	require.NoError(t, err)

	selectedIDs := []int64{repos[0].ID, repos[1].ID, repos[2].ID, repos[3].ID, repos[4].ID, repos[5].ID, repos[6].ID}
	for _, relation := range []struct {
		table  string
		column string
	}{
		{table: "user_likes", column: "repo_id"},
		{table: "lfs_meta_objects", column: "repository_id"},
		{table: "files", column: "repository_id"},
		{table: "repository_tags", column: "repository_id"},
		{table: "repository_downloads", column: "repository_id"},
		{table: "metadata", column: "repository_id"},
		{table: "repository_statistics", column: "repository_id"},
		{table: "lfs_files", column: "repository_id"},
	} {
		_, err = db.Core.ExecContext(ctx, "INSERT INTO "+relation.table+" (id, "+relation.column+") VALUES (?, ?), (?, ?)", 1, repos[0].ID, 2, repos[7].ID)
		require.NoError(t, err)
	}
	_, err = db.Core.ExecContext(ctx, "INSERT INTO mirrors (id, repository_id) VALUES (?, ?), (?, ?)", 1, repos[0].ID, 2, repos[7].ID)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, "INSERT INTO mirror_tasks (id, mirror_id) VALUES (?, ?), (?, ?)", 1, 1, 2, 2)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&[]RepositoriesRuntimeFramework{
		{RuntimeFrameworkID: 1, RepoID: repos[0].ID, Type: 0},
		{RuntimeFrameworkID: 1, RepoID: repos[7].ID, Type: 0},
	}).Exec(ctx)
	require.NoError(t, err)
	for _, tableColumn := range []struct{ table, column string }{
		{table: "lfs_locks", column: "repository_id"},
		{table: "collection_repositories", column: "repository_id"},
		{table: "dataviewers", column: "repo_id"},
		{table: "dataviewer_jobs", column: "repo_id"},
		{table: "mcp_scan_results", column: "repository_id"},
		{table: "xnet_migration_tasks", column: "repository_id"},
		{table: "recom_op_weights", column: "repository_id"},
		{table: "recom_repo_scores", column: "repository_id"},
	} {
		_, err = db.Core.ExecContext(ctx, "INSERT INTO "+tableColumn.table+" (id, "+tableColumn.column+") VALUES (?, ?), (?, ?)", 1, repos[0].ID, 2, repos[7].ID)
		require.NoError(t, err)
	}
	_, err = db.Core.ExecContext(ctx, "INSERT INTO repository_files (id, repository_id) VALUES (1, ?), (2, ?)", repos[0].ID, repos[7].ID)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, "INSERT INTO repository_file_checks (id, repo_file_id) VALUES (1, 1), (2, 2)")
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, "INSERT INTO repo_relations (id, from_repo_id, to_repo_id) VALUES (1, ?, ?), (2, ?, ?), (3, ?, ?)", repos[0].ID, repos[7].ID, repos[7].ID, repos[1].ID, repos[7].ID, repos[7].ID)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, "INSERT INTO model_trees (id, source_repo_id, target_repo_id) VALUES (1, ?, ?), (2, ?, ?), (3, ?, ?)", repos[0].ID, repos[7].ID, repos[7].ID, repos[1].ID, repos[7].ID, repos[7].ID)
	require.NoError(t, err)

	var deleted []DeletedRepository
	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		deleted, err = deleteRepositoriesByIDs(ctx, tx, selectedIDs, jobClient)
		return err
	})
	require.NoError(t, err)
	require.Len(t, deleted, len(selectedIDs))

	var remaining []Repository
	require.NoError(t, db.Core.NewSelect().Model(&remaining).Scan(ctx))
	require.Len(t, remaining, 1)
	require.Equal(t, repos[7].ID, remaining[0].ID)
	for table, expected := range map[string]int{"models": 2, "datasets": 1, "codes": 1, "spaces": 1, "prompts": 1, "mcp_servers": 1, "skills": 1} {
		count, err := db.Core.NewSelect().Table(table).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, expected, count, table)
	}
	for _, relation := range []string{"user_likes", "lfs_meta_objects", "mirrors", "mirror_tasks", "files", "repository_tags", "repository_downloads", "metadata", "repository_statistics", "lfs_files"} {
		count, err := db.Core.NewSelect().Table(relation).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count, relation)
	}
	for _, relation := range []string{"lfs_locks", "repository_files", "repository_file_checks", "collection_repositories", "dataviewers", "dataviewer_jobs", "mcp_scan_results", "xnet_migration_tasks", "recom_op_weights", "recom_repo_scores"} {
		count, err := db.Core.NewSelect().Table(relation).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count, relation)
	}
	for _, relation := range []string{"repo_relations", "model_trees"} {
		count, err := db.Core.NewSelect().Table(relation).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, count, relation)
	}
	var runtimeFrameworks []RepositoriesRuntimeFramework
	require.NoError(t, db.Core.NewSelect().Model(&runtimeFrameworks).Scan(ctx))
	require.Len(t, runtimeFrameworks, 2)

	var pending []PendingDeletion
	require.NoError(t, db.Core.NewSelect().Model(&pending).Order("id ASC").Scan(ctx))
	require.Empty(t, pending)
	require.Len(t, jobClient.inputs, len(selectedIDs))
	for index, input := range jobClient.inputs {
		require.Equal(t, repos[index].ID, input.RepositoryID)
		require.Equal(t, repos[index].RepositoryType, input.RepositoryType)
		require.Equal(t, repos[index].Path, input.Path)
		require.Equal(t, repos[index].GitalyPath(), input.GitalyPath)
		require.Equal(t, OrgNamespace, input.OwnerType)
		require.Equal(t, testOrganizationUUID, input.OwnerUUID)
	}
	var withDeleted []Repository
	require.NoError(t, db.Core.NewSelect().Model(&withDeleted).WhereAllWithDeleted().Scan(ctx))
	require.Len(t, withDeleted, len(repos))

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		deleted, err = deleteRepositoriesByIDs(ctx, tx, nil, jobClient)
		return err
	})
	require.NoError(t, err)
	require.Empty(t, deleted)
}

func TestRepoStoreDeleteRepoSoftDeletesAndEnqueues(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	jobClient := &recordingRepositoryDeletionJobClient{}
	store := NewRepoStoreWithDBAndDeletionJobClient(db, jobClient)
	insertTestUserNamespace(t, db, "owner", "owner-uuid")
	var err error
	repo, err := store.CreateRepo(ctx, Repository{
		UserID: 1, Name: "model", Path: "owner/model", GitPath: "models_owner/model", RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)

	require.NoError(t, store.DeleteRepo(ctx, *repo))
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, repo.ID, jobClient.inputs[0].RepositoryID)
	var deleted Repository
	require.NoError(t, db.Core.NewSelect().Model(&deleted).WhereAllWithDeleted().Where("id = ?", repo.ID).Scan(ctx))
	require.False(t, deleted.DeletedAt.IsZero())
}

func TestDeleteRepositoriesByIDsRollback(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	repo := Repository{UserID: 1, Name: "repo", Path: "org/repo", GitPath: "models_org/repo", RepositoryType: types.ModelRepo}
	insertTestOrganizationNamespace(t, db, "org")
	var err error
	_, err = db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.NoError(t, err)

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := deleteRepositoriesByIDs(ctx, tx, []int64{repo.ID}, &recordingRepositoryDeletionJobClient{})
		require.NoError(t, err)
		return errors.New("forced failure")
	})
	require.EqualError(t, err, "forced failure")
	require.NoError(t, db.Core.NewSelect().Model(&Repository{}).Where("id = ?", repo.ID).Scan(ctx))
	count, err := db.Core.NewSelect().Model((*PendingDeletion)(nil)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestDeleteRepositoriesByIDsEnqueueFailureRollsBack(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	insertTestOrganizationNamespace(t, db, "org")
	var err error
	repo := Repository{UserID: 1, Name: "repo", Path: "org/repo", GitPath: "models_org/repo", RepositoryType: types.ModelRepo}
	_, err = db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.NoError(t, err)

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := deleteRepositoriesByIDs(ctx, tx, []int64{repo.ID, repo.ID}, &recordingRepositoryDeletionJobClient{err: errors.New("river unavailable")})
		return err
	})
	require.EqualError(t, err, "enqueue repository 1 deletion: river unavailable")
	require.NoError(t, db.Core.NewSelect().Model(&Repository{}).Where("id = ?", repo.ID).Scan(ctx))
}

func TestDeleteRepositoriesByIDsOwnerLookupFailureRollsBack(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	// The namespace exists, but no organization is associated through
	// organizations.namespace_id. Namespace.UUID must never be used as fallback.
	_, err := db.Core.NewInsert().Model(&Namespace{
		Path: "orphan", NamespaceType: OrgNamespace, UUID: "wrong-namespace-uuid",
	}).Exec(ctx)
	require.NoError(t, err)
	repository := Repository{
		UserID: 1, Name: "repo", Path: "orphan/repo",
		GitPath: "models_orphan/repo", RepositoryType: types.ModelRepo,
	}
	_, err = db.Core.NewInsert().Model(&repository).Exec(ctx)
	require.NoError(t, err)
	jobClient := &recordingRepositoryDeletionJobClient{}

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := deleteRepositoriesByIDs(ctx, tx, []int64{repository.ID}, jobClient)
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Empty(t, jobClient.inputs)
	var active Repository
	require.NoError(t, db.Core.NewSelect().Model(&active).Where("id = ?", repository.ID).Scan(ctx))
	require.True(t, active.DeletedAt.IsZero())
}

func TestDeleteRepositoriesByIDsDeduplicatesIDs(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	insertTestOrganizationNamespace(t, db, "org")
	var err error
	repo := Repository{UserID: 1, Name: "repo", Path: "org/repo", GitPath: "models_org/repo", RepositoryType: types.ModelRepo}
	_, err = db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.NoError(t, err)
	jobClient := &recordingRepositoryDeletionJobClient{}

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		deleted, err := deleteRepositoriesByIDs(ctx, tx, []int64{repo.ID, repo.ID}, jobClient)
		require.Len(t, deleted, 1)
		return err
	})
	require.NoError(t, err)
	require.Len(t, jobClient.inputs, 1)
}

func TestDeleteRepositoriesByIDsAffectedRowsMismatchRollsBack(t *testing.T) {
	db := newRepositoryBatchDeleteTestDB(t)
	ctx := context.Background()
	repo := Repository{UserID: 1, Name: "repo", Path: "org/repo", GitPath: "models_org/repo", RepositoryType: types.ModelRepo}
	insertTestOrganizationNamespace(t, db, "org")
	var err error
	_, err = db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&Model{RepositoryID: repo.ID}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.ExecContext(ctx, `CREATE TRIGGER ignore_repository_delete BEFORE UPDATE OF deleted_at ON repositories BEGIN SELECT RAISE(IGNORE); END`)
	require.NoError(t, err)

	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := deleteRepositoriesByIDs(ctx, tx, []int64{repo.ID}, &recordingRepositoryDeletionJobClient{})
		return err
	})
	require.EqualError(t, err, "soft-delete repositories: expected 1 affected rows, got 0")
	require.NoError(t, db.Core.NewSelect().Model(&Repository{}).Where("id = ?", repo.ID).Scan(ctx))
	require.NoError(t, db.Core.NewSelect().Model(&Model{}).Where("repository_id = ?", repo.ID).Scan(ctx))
	count, err := db.Core.NewSelect().Model((*PendingDeletion)(nil)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestDeleteRepositoriesByIDsLocksSelectedRepositories(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	db.RegisterModel((*RepositoryTag)(nil))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "repositories" AS "repository" WHERE .* FOR UPDATE`).
		WillReturnError(errors.New("stop after locked select"))
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, err = deleteRepositoriesByIDs(context.Background(), tx, []int64{1, 2}, &recordingRepositoryDeletionJobClient{})
	require.ErrorContains(t, err, "load repositories before deletion: stop after locked select")
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	mock.ExpectClose()
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
