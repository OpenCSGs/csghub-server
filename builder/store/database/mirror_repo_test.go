package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// fakeMirrorJobClient records mirror job enqueue attempts for transaction tests.
type fakeMirrorJobClient struct {
	err    error
	inputs []database.MirrorJobInput
}

// InsertMirrorRepoJobTx records the input and optionally returns a configured error.
func (c *fakeMirrorJobClient) InsertMirrorRepoJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorJobInput) (int64, error) {
	c.inputs = append(c.inputs, input)
	if c.err != nil {
		return 0, c.err
	}
	return 123, nil
}

// assertQueuedMirrorTask verifies the task created for a mirror repo transaction.
func assertQueuedMirrorTask(t *testing.T, ctx context.Context, db *database.DB, mirror *database.Mirror, urgent bool) {
	t.Helper()

	require.NotZero(t, mirror.CurrentTaskID)
	task, err := database.NewMirrorTaskStoreWithDB(db).FindByID(ctx, mirror.CurrentTaskID)
	require.NoError(t, err)
	require.Equal(t, mirror.ID, task.MirrorID)
	require.Equal(t, types.MirrorQueued, task.Status)
	require.Equal(t, mirror.Priority, task.Priority)
	require.Equal(t, int64(123), task.RepoJobID)
	require.Equal(t, urgent, task.IsUrgent)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsForExistingRepo verifies existing repos can be bound to one mirror transactionally.
func TestMirrorRepoStore_CreateMirrorRepoRecordsForExistingRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repo, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "ns/name",
		GitPath:        "models_ns/name",
		Name:           "name",
		Nickname:       "name",
		Private:        true,
		DefaultBranch:  types.MainBranch,
		RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)

	jobClient := &fakeMirrorJobClient{}
	store := database.NewMirrorRepoStoreWithDB(db, jobClient)
	mirror, err := store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository: repo,
		Urgent:     true,
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/upstream/name.git",
			SourceRepoPath: "upstream/name",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.NoError(t, err)
	require.Equal(t, repo.ID, mirror.RepositoryID)
	require.Equal(t, types.SyncStatusPending, mirror.Repository.SyncStatus)
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, mirror.ID, jobClient.inputs[0].MirrorID)
	require.Equal(t, repo.ID, jobClient.inputs[0].RepositoryID)
	require.Equal(t, mirror.CurrentTaskID, jobClient.inputs[0].MirrorTaskID)
	require.Equal(t, types.ModelRepo, jobClient.inputs[0].RepoType)
	require.Equal(t, "https://github.com/upstream/name.git", jobClient.inputs[0].SourceURL)
	require.Equal(t, "ns/name", jobClient.inputs[0].RepoPath)
	require.Equal(t, types.ASAPMirrorPriority, jobClient.inputs[0].Priority)
	require.True(t, jobClient.inputs[0].Urgent)
	assertQueuedMirrorTask(t, ctx, db, mirror, true)

	reloadedRepo, err := database.NewRepoStoreWithDB(db).FindByPath(ctx, types.ModelRepo, "ns", "name")
	require.NoError(t, err)
	require.Equal(t, types.SyncStatusPending, reloadedRepo.SyncStatus)

	_, err = store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository: repo,
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/other/name.git",
			SourceRepoPath: "other/name",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.Error(t, err)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsUpdatesExistingRepoSourceFields verifies source columns are persisted from the repository row.
func TestMirrorRepoStore_CreateMirrorRepoRecordsUpdatesExistingRepoSourceFields(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repo, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "ns/source",
		GitPath:        "models_ns/source",
		Name:           "source",
		Nickname:       "source",
		Private:        true,
		DefaultBranch:  types.MainBranch,
		RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)

	repo.GithubPath = "upstream/source"
	store := database.NewMirrorRepoStoreWithDB(db, &fakeMirrorJobClient{})
	_, err = store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository: repo,
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/upstream/source.git",
			SourceRepoPath: "upstream/source",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.NoError(t, err)

	reloadedRepo, err := database.NewRepoStoreWithDB(db).FindByPath(ctx, types.ModelRepo, "ns", "source")
	require.NoError(t, err)
	require.Equal(t, "upstream/source", reloadedRepo.GithubPath)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsForNewRepo verifies new mirror repos keep their final repository address.
func TestMirrorRepoStore_CreateMirrorRepoRecordsForNewRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	jobClient := &fakeMirrorJobClient{}
	store := database.NewMirrorRepoStoreWithDB(db, jobClient)
	mirror, err := store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		CreateRepository: true,
		Repository: &database.Repository{
			UserID:         1,
			Path:           "ns/new",
			GitPath:        "models_ns/new",
			Name:           "new",
			Nickname:       "new",
			Private:        true,
			DefaultBranch:  types.MainBranch,
			RepositoryType: types.ModelRepo,
			GithubPath:     "upstream/new",
		},
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/upstream/new.git",
			SourceRepoPath: "upstream/new",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "ns/new", mirror.Repository.Path)
	require.Equal(t, "models_ns/new", mirror.Repository.GitPath)
	require.Equal(t, types.SyncStatusPending, mirror.Repository.SyncStatus)
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, mirror.ID, jobClient.inputs[0].MirrorID)
	require.Equal(t, mirror.RepositoryID, jobClient.inputs[0].RepositoryID)
	require.Equal(t, mirror.CurrentTaskID, jobClient.inputs[0].MirrorTaskID)
	require.Equal(t, types.ModelRepo, jobClient.inputs[0].RepoType)
	require.Equal(t, "ns/new", jobClient.inputs[0].RepoPath)
	assertQueuedMirrorTask(t, ctx, db, mirror, false)

	repo, err := database.NewRepoStoreWithDB(db).FindByPath(ctx, types.ModelRepo, "ns", "new")
	require.NoError(t, err)
	require.Equal(t, mirror.RepositoryID, repo.ID)
	require.Equal(t, "models_ns/new", repo.GitPath)
	require.Equal(t, "upstream/new", repo.GithubPath)
	require.Equal(t, types.SyncStatusPending, repo.SyncStatus)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsForNewCodeRepoSetsLastUpdatedAt preserves GitLab import code row semantics.
func TestMirrorRepoStore_CreateMirrorRepoRecordsForNewCodeRepoSetsLastUpdatedAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	jobClient := &fakeMirrorJobClient{}
	store := database.NewMirrorRepoStoreWithDB(db, jobClient)
	mirror, err := store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		CreateRepository: true,
		Repository: &database.Repository{
			UserID:         1,
			Path:           "ns/code",
			GitPath:        "codes_ns/code",
			Name:           "code",
			Nickname:       "code",
			Private:        true,
			DefaultBranch:  types.MainBranch,
			RepositoryType: types.CodeRepo,
		},
		Mirror: database.Mirror{
			SourceUrl:      "https://gitlab.com/upstream/code.git",
			SourceRepoPath: "upstream/code",
			Priority:       types.HighMirrorPriority,
		},
	})
	require.NoError(t, err)

	code, err := database.NewCodeStoreWithDB(db).ByRepoID(ctx, mirror.RepositoryID)
	require.NoError(t, err)
	require.False(t, code.LastUpdatedAt.IsZero())
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsForMCPServerRows verifies prebuilt MCP rows are inserted transactionally.
func TestMirrorRepoStore_CreateMirrorRepoRecordsForMCPServerRows(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorRepoStoreWithDB(db, &fakeMirrorJobClient{})
	mirror, err := store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		CreateRepository: true,
		Repository: &database.Repository{
			UserID:         1,
			Path:           "ns/mcp",
			GitPath:        "mcpservers_ns/mcp",
			Name:           "mcp",
			Nickname:       "mcp",
			Private:        true,
			DefaultBranch:  types.MainBranch,
			RepositoryType: types.MCPServerRepo,
		},
		MCPServer: &database.MCPServer{
			ToolsNum:      1,
			Configuration: `{"mode":"stdio"}`,
			Schema:        `{"tools":[{"name":"search"}]}`,
			AvatarURL:     "https://example.com/avatar.png",
		},
		MCPServerProperties: []database.MCPServerProperty{
			{
				Kind:        types.MCPPropTool,
				Name:        "search",
				Description: "Search things",
				Schema:      `{"type":"object"}`,
			},
		},
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/upstream/mcp.git",
			SourceRepoPath: "upstream/mcp",
			Priority:       types.HighMirrorPriority,
		},
	})
	require.NoError(t, err)

	mcpServer, err := database.NewMCPServerStoreWithDB(db).ByRepoID(ctx, mirror.RepositoryID)
	require.NoError(t, err)
	require.Equal(t, 1, mcpServer.ToolsNum)
	require.Equal(t, `{"mode":"stdio"}`, mcpServer.Configuration)
	require.Equal(t, `{"tools":[{"name":"search"}]}`, mcpServer.Schema)
	require.Equal(t, "https://example.com/avatar.png", mcpServer.AvatarURL)

	var properties []database.MCPServerProperty
	err = db.Core.NewSelect().
		Model(&properties).
		Where("mcp_server_id = ?", mcpServer.ID).
		Scan(ctx)
	require.NoError(t, err)
	require.Len(t, properties, 1)
	require.Equal(t, types.MCPPropTool, properties[0].Kind)
	require.Equal(t, "search", properties[0].Name)
	require.Equal(t, "Search things", properties[0].Description)
	require.Equal(t, `{"type":"object"}`, properties[0].Schema)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsRollsBackWhenJobInsertFails verifies job enqueue participates in the mirror transaction.
func TestMirrorRepoStore_CreateMirrorRepoRecordsRollsBackWhenJobInsertFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repo, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "ns/rollback",
		GitPath:        "models_ns/rollback",
		Name:           "rollback",
		Nickname:       "rollback",
		Private:        true,
		DefaultBranch:  types.MainBranch,
		RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)

	jobClient := &fakeMirrorJobClient{err: errors.New("insert job failed")}
	store := database.NewMirrorRepoStoreWithDB(db, jobClient)
	_, err = store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		Repository: repo,
		Mirror: database.Mirror{
			SourceUrl:      "https://github.com/upstream/rollback.git",
			SourceRepoPath: "upstream/rollback",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.Error(t, err)
	require.Len(t, jobClient.inputs, 1)
	require.NotZero(t, jobClient.inputs[0].MirrorTaskID)

	var mirror database.Mirror
	err = db.Core.NewSelect().
		Model(&mirror).
		Where("repository_id = ?", repo.ID).
		Scan(ctx)
	require.ErrorIs(t, err, sql.ErrNoRows)

	_, err = database.NewMirrorTaskStoreWithDB(db).FindByID(ctx, jobClient.inputs[0].MirrorTaskID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// TestMirrorRepoStore_CreateMirrorRepoRecordsRollsBackNewRepoWhenJobInsertFails verifies new repo creation has no residual rows when job enqueue fails.
func TestMirrorRepoStore_CreateMirrorRepoRecordsRollsBackNewRepoWhenJobInsertFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sourceURL := "https://github.com/upstream/new-rollback.git"
	repoPath := "ns/new-rollback"
	jobClient := &fakeMirrorJobClient{err: errors.New("insert job failed")}
	store := database.NewMirrorRepoStoreWithDB(db, jobClient)

	_, err := store.CreateMirrorRepoRecords(ctx, database.CreateMirrorRepoRecordsInput{
		CreateRepository: true,
		Repository: &database.Repository{
			UserID:         1,
			Path:           repoPath,
			GitPath:        "models_ns/new-rollback",
			Name:           "new-rollback",
			Nickname:       "new-rollback",
			Private:        true,
			DefaultBranch:  types.MainBranch,
			RepositoryType: types.ModelRepo,
		},
		Mirror: database.Mirror{
			SourceUrl:      sourceURL,
			SourceRepoPath: "upstream/new-rollback",
			Priority:       types.ASAPMirrorPriority,
		},
	})
	require.Error(t, err)
	require.Len(t, jobClient.inputs, 1)

	repoCount, err := db.Core.NewSelect().
		Model((*database.Repository)(nil)).
		Where("path = ?", repoPath).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, repoCount)

	mirrorCount, err := db.Core.NewSelect().
		Model((*database.Mirror)(nil)).
		Where("source_url = ?", sourceURL).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, mirrorCount)

	taskCount, err := db.Core.NewSelect().
		Model((*database.MirrorTask)(nil)).
		Where("mirror_id IN (SELECT id FROM mirrors WHERE source_url = ?)", sourceURL).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, taskCount)

	scoreCount, err := db.Core.NewSelect().
		Model((*database.RecomRepoScore)(nil)).
		Where("repository_id IN (SELECT id FROM repositories WHERE path = ?)", repoPath).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, scoreCount)
}
