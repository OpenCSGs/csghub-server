package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepositoryDeletionMirrorTaskStoreFindsTaskForSoftDeletedRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	repo, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: "ns/deleting", GitPath: "models_ns/deleting"}))
	require.NoError(t, err)
	mirror, err := database.NewMirrorStoreWithDB(db).Create(ctx, &database.Mirror{
		SourceUrl: "https://example.com/repo.git", RepositoryID: repo.ID,
		Status: types.MirrorRepoSyncStart, Priority: types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	task := &database.MirrorTask{MirrorID: mirror.ID, Status: types.MirrorRepoSyncStart, Priority: types.ASAPMirrorPriority}
	require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))
	_, err = db.Core.NewUpdate().Model(mirror).Set("current_task_id = ?", task.ID).WherePK().Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewDelete().Model(repo).WherePK().Exec(ctx)
	require.NoError(t, err)

	store := database.NewRepositoryDeletionMirrorTaskStoreWithDB(db)
	taskID, err := store.FindCurrentMirrorTaskID(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, task.ID, taskID)

	taskID, err = store.FindCurrentMirrorTaskID(ctx, repo.ID+999)
	require.NoError(t, err)
	require.Zero(t, taskID)
}
