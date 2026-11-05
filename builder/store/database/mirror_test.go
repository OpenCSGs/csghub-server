package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type fakeMirrorDeleteJobCancelClient struct {
	cancelled []int64
	err       error
}

func (c *fakeMirrorDeleteJobCancelClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	c.cancelled = append(c.cancelled, jobID)
	return c.err
}

// mirrorMetadataUpdateContextKey identifies the stale writer in concurrency tests.
type mirrorMetadataUpdateContextKey struct{}

// mirrorMetadataUpdateHook pauses a stale metadata write while task transactions commit.
type mirrorMetadataUpdateHook struct {
	ready  chan struct{}
	resume chan struct{}
}

// BeforeQuery exposes a deterministic interleaving without delaying task queries.
func (h *mirrorMetadataUpdateHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(mirrorMetadataUpdateContextKey{}) == h && strings.HasPrefix(event.Query, `UPDATE "mirrors"`) {
		close(h.ready)
		select {
		case <-h.resume:
		case <-ctx.Done():
		}
	}
	return ctx
}

// AfterQuery completes the Bun hook contract without changing query results.
func (h *mirrorMetadataUpdateHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestMirrorStore_StaleMetadataUpdatePreservesPublication replays a callback or
// settings update racing with real requeue/completion transactions and cancellation.
func TestMirrorStore_StaleMetadataUpdatePreservesPublication(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	for _, tc := range []struct {
		name          string
		finishFirst   bool
		cancelPublish bool
		taskless      bool
	}{
		{name: "new task running"},
		{name: "new task already completed", finishFirst: true},
		{name: "owner cancelled publication", cancelPublish: true},
		{name: "snapshot predates first task", taskless: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			repo, stale := newPublicationFixture(t, db, strings.ReplaceAll(tc.name, " ", "-"), true, types.SensitiveCheckPass)
			if tc.taskless {
				stale.CurrentTaskID = 0
				stale.RepositoryID = 0
			}
			stale.LastUpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
			store := database.NewMirrorStoreWithDB(db)
			repos := database.NewRepoStoreWithDB(db)
			tasks := database.NewMirrorTaskJobStoreWithDB(db)
			hook := &mirrorMetadataUpdateHook{ready: make(chan struct{}), resume: make(chan struct{})}
			db.BunDB.AddQueryHook(hook)
			resume := sync.OnceFunc(func() { close(hook.resume) })
			defer resume()
			finished := make(chan error, 1)
			go func() {
				finished <- store.Update(context.WithValue(ctx, mirrorMetadataUpdateContextKey{}, hook), stale)
			}()
			select {
			case <-hook.ready:
			case <-ctx.Done():
				t.Fatal("stale mirror update did not reach its write")
			}
			task, err := tasks.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
				MirrorID: stale.ID, RepositoryID: repo.ID, JobClient: &fakeMirrorRepoJobClient{},
			})
			require.NoError(t, err)
			task, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
			require.NoError(t, err)
			if tc.finishFirst {
				task, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorNoLfsToSync)
				require.NoError(t, err)
			}
			if tc.cancelPublish {
				_, err = repos.UpdateRepo(ctx, *repo, types.RepositoryUpdateOptions{Private: new(true)})
				require.NoError(t, err)
			}
			resume()
			require.NoError(t, <-finished)
			current, err := store.FindByID(ctx, stale.ID)
			require.NoError(t, err)
			require.Equal(t, task.ID, current.CurrentTaskID)
			require.Equal(t, repo.ID, current.RepositoryID)
			require.Equal(t, task.Status, current.Status)
			require.True(t, current.LastUpdatedAt.Equal(stale.LastUpdatedAt))
			require.NoError(t, repos.PublishPendingModelRepo(ctx, repo.ID))
			got, err := repos.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, !tc.finishFirst, got.Private)
			require.Equal(t, !tc.finishFirst && !tc.cancelPublish, got.PendingPublish)
			if !tc.finishFirst {
				_, err = tasks.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorNoLfsToSync)
				require.NoError(t, err)
			}
			require.NoError(t, repos.PublishPendingModelRepo(ctx, repo.ID))
			got, err = repos.FindById(ctx, repo.ID)
			require.NoError(t, err)
			require.Equal(t, tc.cancelPublish, got.Private)
			require.False(t, got.PendingPublish)
		})
	}
}

// TestMirrorStore_UpdateTasklessLegacyFailure preserves failure reporting and
// configuration changes for legacy mirrors that have not acquired a task yet.
func TestMirrorStore_UpdateTasklessLegacyFailure(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	mirror, err := store.Create(ctx, &database.Mirror{Status: types.MirrorQueued})
	require.NoError(t, err)
	mirror.Status = types.MirrorRepoSyncFailed
	mirror.LastMessage = "repository creation failed"
	mirror.SourceUrl = "https://huggingface.co/upstream/model"
	mirror.Username = "source-user"
	mirror.AccessToken = "source-token"
	require.NoError(t, store.Update(ctx, mirror))
	got, err := store.FindByID(ctx, mirror.ID)
	require.NoError(t, err)
	require.Equal(t, mirror.Status, got.Status)
	require.Equal(t, mirror.LastMessage, got.LastMessage)
	require.Equal(t, mirror.SourceUrl, got.SourceUrl)
	require.Equal(t, mirror.Username, got.Username)
	require.Equal(t, mirror.AccessToken, got.AccessToken)
}

func TestMirrorStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	_, err := store.Create(ctx, &database.Mirror{
		Interval:          "foo",
		RepositoryID:      123,
		PushMirrorCreated: true,
		Status:            types.MirrorLfsSyncFinished,
		Priority:          types.HighMirrorPriority,
	})
	require.Nil(t, err)

	mi := &database.Mirror{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)

	mi, err = store.FindByID(ctx, mi.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)

	mi, err = store.FindByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)
	payload, err := json.Marshal(mi)
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"interval"`)

	exist, err := store.IsExist(ctx, 123)
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.IsExist(ctx, 456)
	require.Nil(t, err)
	require.False(t, exist)

	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/n",
		Name:           "repo",
		Path:           "ns/n",
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "NS", "N")
	require.Nil(t, err)
	require.True(t, exist)

	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "ns", "n2")
	require.Nil(t, err)
	require.False(t, exist)

	mi.RepositoryID = repo.ID
	err = store.Update(ctx, mi)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, repo.ID, mi.RepositoryID)

	mi, err = store.FindByRepoPath(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)
	require.Equal(t, repo.ID, mi.RepositoryID)

	ms, err := store.PushedMirror(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))

	ms, err = store.NoPushMirror(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, len(ms))

	ms, err = store.Finished(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))

	ms, err = store.Unfinished(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, len(ms))

	mi.AccessToken = "abc"
	repo.Nickname = "fooo"
	err = store.UpdateMirrorAndRepository(ctx, mi, repo)
	require.Nil(t, err)
	mi = &database.Mirror{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "abc", mi.AccessToken)
	repo = &database.Repository{}
	err = db.Core.NewSelect().Model(repo).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "fooo", repo.Nickname)

	err = store.Delete(ctx, mi)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, mi.ID)
	require.NotNil(t, err)

}

// TestMirrorStore_DeleteWithTaskCancelTxCancelsJobsAndDeletesMirror verifies only the current task job is cancelled.
func TestMirrorStore_DeleteWithTaskCancelTxCancelsJobsAndDeletesMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
		SyncStatus:     types.SyncStatusInProgress,
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))

	mirror, err := store.Create(ctx, &database.Mirror{
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	tasks := []database.MirrorTask{
		{MirrorID: mirror.ID, Status: types.MirrorCanceled, Priority: types.ASAPMirrorPriority, RepoJobID: 11, LFSJobID: 12},
		{MirrorID: mirror.ID, Status: types.MirrorRepoSyncStart, Priority: types.HighMirrorPriority, RepoJobID: 21},
	}
	for i := range tasks {
		require.NoError(t, db.Core.NewInsert().Model(&tasks[i]).Scan(ctx, &tasks[i]))
	}
	_, err = db.Core.NewUpdate().
		Model((*database.Mirror)(nil)).
		Set("current_task_id = ?", tasks[1].ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.NoError(t, err)

	cancelClient := &fakeMirrorDeleteJobCancelClient{}
	lockHook := &mirrorLockOrderHook{}
	db.BunDB.AddQueryHook(lockHook)
	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, cancelClient)
	require.NoError(t, err)
	require.Equal(t, []string{"mirror", "repository", "mirror_task"}, lockHook.tables)
	require.Equal(t, []int64{21}, cancelClient.cancelled)

	_, err = store.FindByID(ctx, mirror.ID)
	require.Error(t, err)
	count, err := db.Core.NewSelect().Model((*database.MirrorTask)(nil)).Where("mirror_id = ?", mirror.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	var storedRepo database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx))
	require.Equal(t, types.SyncStatusCanceled, storedRepo.SyncStatus)
}

// TestMirrorStore_DeleteWithTaskCancelTxRollsBackWhenJobCancelFails verifies current-job cancellation remains atomic.
func TestMirrorStore_DeleteWithTaskCancelTxRollsBackWhenJobCancelFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
	mirror, err := store.Create(ctx, &database.Mirror{
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	task := &database.MirrorTask{
		MirrorID:  mirror.ID,
		Status:    types.MirrorRepoSyncStart,
		Priority:  types.ASAPMirrorPriority,
		RepoJobID: 11,
	}
	require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))
	_, err = db.Core.NewUpdate().
		Model((*database.Mirror)(nil)).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.NoError(t, err)

	cancelClient := &fakeMirrorDeleteJobCancelClient{err: errors.New("cancel failed")}
	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, cancelClient)
	require.Error(t, err)

	_, err = store.FindByID(ctx, mirror.ID)
	require.NoError(t, err)
	count, err := db.Core.NewSelect().Model((*database.MirrorTask)(nil)).Where("mirror_id = ?", mirror.ID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// TestMirrorStore_DeleteWithTaskCancelTxKeepsCompletedRepoStatus verifies completed synchronization remains completed.
func TestMirrorStore_DeleteWithTaskCancelTxKeepsCompletedRepoStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
		SyncStatus:     types.SyncStatusCompleted,
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
	mirror, err := store.Create(ctx, &database.Mirror{
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorLfsSyncFinished,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	task := &database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
		Priority: types.ASAPMirrorPriority,
	}
	require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))
	_, err = db.Core.NewUpdate().
		Model((*database.Mirror)(nil)).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.NoError(t, err)

	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorDeleteJobCancelClient{})
	require.NoError(t, err)

	var storedRepo database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx))
	require.Equal(t, types.SyncStatusCompleted, storedRepo.SyncStatus)
}

// TestMirrorStore_DeleteWithTaskCancelTxSucceedsWhenRepoDeleted verifies that a
// mirror can still be deleted by id when its repository row has already been
// hard-deleted. The mirror keeps a stale RepositoryID pointing at the gone
// repo, so the repo lookup inside the transaction must tolerate the missing
// row instead of rolling the whole deletion back.
func TestMirrorStore_DeleteWithTaskCancelTxSucceedsWhenRepoDeleted(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
		SyncStatus:     types.SyncStatusInProgress,
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
	// Simulate the repo being hard-deleted ahead of the mirror, as
	// repoStoreImpl.DeleteRepo does via ForceDelete.
	_, err := db.Core.NewDelete().Model(repo).WherePK().ForceDelete().Exec(ctx)
	require.NoError(t, err)

	mirror, err := store.Create(ctx, &database.Mirror{
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	tasks := []database.MirrorTask{
		{MirrorID: mirror.ID, Status: types.MirrorCanceled, Priority: types.ASAPMirrorPriority, RepoJobID: 31, LFSJobID: 32},
		{MirrorID: mirror.ID, Status: types.MirrorRepoSyncStart, Priority: types.HighMirrorPriority, RepoJobID: 41},
	}
	for i := range tasks {
		require.NoError(t, db.Core.NewInsert().Model(&tasks[i]).Scan(ctx, &tasks[i]))
	}
	_, err = db.Core.NewUpdate().
		Model((*database.Mirror)(nil)).
		Set("current_task_id = ?", tasks[1].ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.NoError(t, err)

	cancelClient := &fakeMirrorDeleteJobCancelClient{}
	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, cancelClient)
	require.NoError(t, err)
	require.Equal(t, []int64{41}, cancelClient.cancelled)

	// The mirror and its tasks must be gone even though the repo row was missing.
	_, err = store.FindByID(ctx, mirror.ID)
	require.Error(t, err)
	count, err := db.Core.NewSelect().Model((*database.MirrorTask)(nil)).Where("mirror_id = ?", mirror.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestMirrorStore_FindWithMapping(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", RepositoryType: types.ModelRepo, Path: "ns/repo1", HFPath: "hf/repo1"},
		{Name: "repo2", RepositoryType: types.DatasetRepo, Path: "ns/repo2", MSPath: "ms/repo2"},
		{Name: "repo3", RepositoryType: types.PromptRepo, Path: "ns/repo3"},
	}

	for _, repo := range repos {
		repo.GitPath = repo.Path
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
	}

	mi, err := store.FindWithMapping(ctx, types.ModelRepo, "ns", "repo1", types.CSGHubMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Name)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "HF", "REPO1", types.HFMapping)
	require.Nil(t, err)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "aaa", "repo1", types.HFMapping)
	require.NotNil(t, err)

	mi, err = store.FindWithMapping(ctx, types.DatasetRepo, "MS", "REPO2", types.ModelScopeMapping)
	require.Nil(t, err)
	require.Equal(t, "repo2", mi.Name)

	mi, err = store.FindWithMapping(ctx, types.ModelRepo, "HF", "REPO1", types.AutoMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Name)

	mi, err = store.FindWithMapping(ctx, types.PromptRepo, "NS", "REPO3", types.CSGHubMapping)
	require.Nil(t, err)
	require.Equal(t, "repo3", mi.Name)
}

func TestMirrorStore_ToSync(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	dt := time.Now().Add(1 * time.Hour)
	mirrors := []*database.Mirror{
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFailed, SourceUrl: "m1"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFinished, SourceUrl: "m2"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFailed, SourceUrl: "m3"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFinished, SourceUrl: "m4"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncStart, SourceUrl: "m5"},
		{NextExecutionTimestamp: dt.Add(-5 * time.Hour), Status: types.MirrorLfsSyncFinished, SourceUrl: "m7"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFatal, SourceUrl: "m8"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncStart, SourceUrl: "m9"},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	ms, err := store.ToSyncRepo(ctx)
	require.Nil(t, err)
	names := []string{}
	for _, m := range ms {
		names = append(names, m.SourceUrl)
	}
	require.ElementsMatch(t, []string{"m1", "m3", "m7"}, names)

	ms, err = store.ToSyncLfs(ctx)
	require.Nil(t, err)
	names = []string{}
	for _, m := range ms {
		names = append(names, m.SourceUrl)
	}
	require.ElementsMatch(t, []string{"m4", "m7"}, names)

}

func TestMirrorStore_IndexWithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	msStore := database.NewMirrorTaskStoreWithDB(db)

	mirrors := []*database.Mirror{
		{LocalRepoPath: "foo", SourceUrl: "bar"},
		{LocalRepoPath: "bar", SourceUrl: "foo"},
	}
	var mStatus types.MirrorTaskStatus
	for i, m := range mirrors {
		mr, err := store.Create(ctx, m)
		require.Nil(t, err)
		if i == 0 {
			mStatus = types.MirrorRepoSyncStart
		} else {
			mStatus = types.MirrorLfsSyncFailed
		}
		ct, err := msStore.Create(ctx, database.MirrorTask{
			MirrorID: mr.ID,
			Status:   mStatus,
		})
		require.Nil(t, err)
		m.CurrentTaskID = ct.ID
		err = msStore.SetMirrorCurrentTaskID(ctx, ct)
		require.Nil(t, err)
	}

	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, false)
	require.Nil(t, err)
	require.Equal(t, 2, count)
	// make sure in "DESC" order
	require.Equal(t, "bar", ms[0].LocalRepoPath)
	require.Equal(t, "foo", ms[1].LocalRepoPath)

	_, count, err = store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, true)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	status := types.MirrorRepoSyncStart
	ms, count, err = store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Status: &status}, false)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "foo", ms[0].LocalRepoPath)
}

// TestMirrorStore_IndexSyncWithPagination verifies static search includes usernames.
func TestMirrorStore_IndexSyncWithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	_, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/other.git", Username: "other-user"})
	require.NoError(t, err)
	wanted, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git", Username: "target-user"})
	require.NoError(t, err)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 1, Search: "TARGET-USER",
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, mirrors, 1)
	require.Equal(t, wanted.ID, mirrors[0].ID)
}

// TestMirrorStore_IndexSyncWithPaginationOrdersByCurrentTaskUpdatedAt verifies recent current tasks sort first and taskless mirrors last.
func TestMirrorStore_IndexSyncWithPaginationOrdersByCurrentTaskUpdatedAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	taskStore := database.NewMirrorTaskStoreWithDB(db)
	createMirrorWithTask := func(updatedAt time.Time) *database.Mirror {
		mirror, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git"})
		require.NoError(t, err)
		task, err := taskStore.Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: types.MirrorQueued})
		require.NoError(t, err)
		_, err = db.Core.NewUpdate().
			Model((*database.MirrorTask)(nil)).
			Set("updated_at = ?", updatedAt).
			Where("id = ?", task.ID).
			Exec(ctx)
		require.NoError(t, err)
		mirror.CurrentTaskID = task.ID
		require.NoError(t, taskStore.SetMirrorCurrentTaskID(ctx, task))
		return mirror
	}

	now := time.Now()
	newest := createMirrorWithTask(now)
	tiedUpdatedAt := now.Add(-time.Hour)
	tiedLowerID := createMirrorWithTask(tiedUpdatedAt)
	tiedHigherID := createMirrorWithTask(tiedUpdatedAt)
	taskless, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/taskless.git"})
	require.NoError(t, err)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{Page: 1, Per: 10})

	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Equal(t, []int64{newest.ID, tiedHigherID.ID, tiedLowerID.ID, taskless.ID}, mirrorIDs(mirrors))
}

// TestMirrorStore_IndexSyncWithPaginationFiltersTaskStatus verifies status conditions apply only to current tasks before pagination.
func TestMirrorStore_IndexSyncWithPaginationFiltersTaskStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	taskStore := database.NewMirrorTaskStoreWithDB(db)
	var wanted *database.Mirror
	for _, status := range []types.MirrorTaskStatus{types.MirrorQueued, types.MirrorRepoSyncFatal} {
		mirror, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git"})
		require.NoError(t, err)
		task, err := taskStore.Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: status})
		require.NoError(t, err)
		mirror.CurrentTaskID = task.ID
		require.NoError(t, taskStore.SetMirrorCurrentTaskID(ctx, task))
		if status == types.MirrorRepoSyncFatal {
			wanted = mirror
		}
	}

	tasklessWaiting, err := store.Create(ctx, &database.Mirror{
		SourceUrl: "https://example.com/taskless-waiting.git",
		Status:    types.MirrorQueued,
	})
	require.NoError(t, err)
	tasklessRunning, err := store.Create(ctx, &database.Mirror{
		SourceUrl: "https://example.com/taskless-running.git",
		Status:    types.MirrorRepoSyncStart,
	})
	require.NoError(t, err)

	allMirrors, allTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{Page: 1, Per: 10})
	require.NoError(t, err)
	require.Equal(t, 4, allTotal)
	require.Contains(t, mirrorIDs(allMirrors), tasklessWaiting.ID)
	require.Contains(t, mirrorIDs(allMirrors), tasklessRunning.ID)

	waiting, waitingTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{types.MirrorQueued, types.MirrorRepoSyncFinished},
	})
	require.NoError(t, err)
	require.Equal(t, 1, waitingTotal)
	require.NotContains(t, mirrorIDs(waiting), tasklessWaiting.ID)
	require.NotContains(t, mirrorIDs(waiting), tasklessRunning.ID)

	running, runningTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{
			types.MirrorRepoSyncStart,
			types.MirrorRepoSyncFailed,
			types.MirrorLfsSyncStart,
			types.MirrorLfsSyncFailed,
		},
	})
	require.NoError(t, err)
	require.Zero(t, runningTotal)
	require.NotContains(t, mirrorIDs(running), tasklessWaiting.ID)
	require.NotContains(t, mirrorIDs(running), tasklessRunning.ID)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{types.MirrorRepoSyncFatal},
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, mirrors, 1)
	require.Equal(t, wanted.ID, mirrors[0].ID)
}

// mirrorIDs returns mirror identifiers for list result assertions.
func mirrorIDs(mirrors []database.Mirror) []int64 {
	ids := make([]int64, 0, len(mirrors))
	for _, mirror := range mirrors {
		ids = append(ids, mirror.ID)
	}
	return ids
}

func TestMirrorStore_StatusCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{Status: types.MirrorRepoSyncFailed},
		{Status: types.MirrorRepoSyncFailed},
		{Status: types.MirrorRepoSyncFinished},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	cs, err := store.StatusCount(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, len(cs))
	require.ElementsMatch(t, []database.MirrorStatusCount{
		{types.MirrorRepoSyncFailed, 2},
		{types.MirrorRepoSyncFinished, 1},
	}, cs)

}

func TestMirrorStore_FindBySourceURLs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorRepoSyncFinished},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	cs, err := store.FindBySourceURLs(ctx, []string{"https://opencsg.com/models/repo2", "https://opencsg.com/models/repo3"})
	require.Nil(t, err)
	require.Equal(t, 3, len(cs))
}

func TestMirrorStore_BatchUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Username: "old-user-1", AccessToken: "old-token-1", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo2", Username: "old-user-2", AccessToken: "old-token-2", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo3", Username: "old-user-3", AccessToken: "old-token-3", Status: types.MirrorRepoSyncFailed, Priority: 1},
	}
	createdMirrors := []database.Mirror{}
	date := time.Now()
	for _, m := range mirrors {
		m, err := store.Create(ctx, m)
		require.Nil(t, err)
		m.Priority = 3
		m.RemoteUpdatedAt = date
		createdMirrors = append(createdMirrors, *m)
	}
	createdMirrors[0].Username = "new-user"
	createdMirrors[0].AccessToken = "new-token"
	createdMirrors[0].Status = types.MirrorQueued
	createdMirrors[0].RepositoryType = types.CodeRepo
	createdMirrors[0].RepositoryCreatorID = 42
	createdMirrors[1].Username = ""
	createdMirrors[1].AccessToken = ""
	err := store.BatchUpdate(ctx, createdMirrors)
	require.Nil(t, err)
	m1, err := store.FindByID(ctx, createdMirrors[0].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[0].Priority, m1.Priority)
	require.Equal(t, "new-user", m1.Username)
	require.Equal(t, "new-token", m1.AccessToken)
	require.Equal(t, types.MirrorQueued, m1.Status)
	require.Equal(t, types.CodeRepo, m1.RepositoryType)
	require.Equal(t, int64(42), m1.RepositoryCreatorID)
	m2, err := store.FindByID(ctx, createdMirrors[1].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[1].Priority, m2.Priority)
	require.Empty(t, m2.Username)
	require.Empty(t, m2.AccessToken)
	m3, err := store.FindByID(ctx, createdMirrors[2].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[2].Priority, m3.Priority)
	require.Equal(t, "old-user-3", m3.Username)
	require.Equal(t, "old-token-3", m3.AccessToken)
	require.Equal(t, types.MirrorRepoSyncFailed, m3.Status)
	require.Equal(t, types.UnknownRepo, m3.RepositoryType)
}

func TestMirrorStoreBatchUpdatePreservesConcurrentBindingAndCompletion(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	for _, initiallyBound := range []bool{false, true} {
		t.Run(fmt.Sprint(initiallyBound), func(t *testing.T) {
			mirror := &database.Mirror{SourceUrl: "https://github.com/acme/repo.git", Status: types.MirrorLfsSyncStart}
			if initiallyBound {
				mirror.RepositoryID = 10
			}
			mirror, err := store.Create(ctx, mirror)
			require.NoError(t, err)
			stale := *mirror
			stale.Status = types.MirrorQueued
			stale.RepositoryType = types.CodeRepo
			stale.RepositoryCreatorID = 42
			stale.Priority = types.HighMirrorPriority
			_, err = db.Core.NewUpdate().Model((*database.Mirror)(nil)).Set("repository_id = ?", 10).
				Set("status = ?", types.MirrorLfsSyncFinished).Set("repository_type = ?", types.ModelRepo).
				Set("repository_creator_id = ?", 7).Where("id = ?", mirror.ID).Exec(ctx)
			require.NoError(t, err)
			require.NoError(t, store.BatchUpdate(ctx, []database.Mirror{stale}))
			current, err := store.FindByID(ctx, mirror.ID)
			require.NoError(t, err)
			require.Equal(t, types.MirrorLfsSyncFinished, current.Status)
			require.Equal(t, types.ModelRepo, current.RepositoryType)
			require.Equal(t, int64(7), current.RepositoryCreatorID)
			require.Equal(t, stale.Priority, current.Priority)
		})
	}
}

func TestMirrorStore_BatchCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorRepoSyncFailed, Priority: 1},
	}
	err := store.BatchCreate(ctx, mirrors)
	require.Nil(t, err)
	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: ""}, false)
	require.Nil(t, err)
	require.Equal(t, 3, len(ms))
	require.Equal(t, 3, count)
}

// TestMirrorStoreToBeScheduledOrdersByPriority verifies lower numeric priority values are scheduled first.
func TestMirrorStoreToBeScheduledOrdersByPriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewMirrorStoreWithDB(db)

	err := store.BatchCreate(ctx, []database.Mirror{
		{SourceUrl: "https://example.com/low.git", Status: types.MirrorQueued, Priority: types.LowMirrorPriority},
		{SourceUrl: "https://example.com/asap.git", Status: types.MirrorQueued, Priority: types.ASAPMirrorPriority},
		{SourceUrl: "https://example.com/medium.git", Status: types.MirrorQueued, Priority: types.MediumMirrorPriority},
	})
	require.NoError(t, err)

	mirrors, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, mirrors, 3)
	require.Equal(t, []types.MirrorPriority{
		types.ASAPMirrorPriority,
		types.MediumMirrorPriority,
		types.LowMirrorPriority,
	}, []types.MirrorPriority{mirrors[0].Priority, mirrors[1].Priority, mirrors[2].Priority})
}

// func TestMirrorStore_ToBeScheduled_Status(t *testing.T) {
// 	db := tests.InitTestDB()
// 	defer db.Close()
// 	ctx := context.TODO()

// 	store := database.NewMirrorStoreWithDB(db)

// 	mirrors := []database.Mirror{
// 		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorFailed, Priority: 1},
// 		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorFinished, Priority: 1},
// 	}
// 	err := store.BatchCreate(ctx, mirrors)
// 	require.Nil(t, err)
// 	ms, err := store.ToBeScheduled(ctx)
// 	require.Nil(t, err)
// 	require.Equal(t, 2, len(ms))
// 	require.Equal(t, ms[0].SourceUrl, "https://opencsg.com/models/repo2")
// 	require.Equal(t, ms[1].SourceUrl, "https://opencsg.com/models/repo1")
// }

// func TestMirrorStore_ToBeScheduled_NextExecutionTimestamp(t *testing.T) {
// 	db := tests.InitTestDB()
// 	defer db.Close()
// 	ctx := context.TODO()

// 	store := database.NewMirrorStoreWithDB(db)

// 	mirrors := []database.Mirror{
// 		{SourceUrl: "https://opencsg.com/models/repo1", NextExecutionTimestamp: time.Now().Add(-time.Hour), Status: types.MirrorFinished, Priority: 3},
// 		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorFinished, Priority: 1},
// 		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorFinished, Priority: 1},
// 	}
// 	err := store.BatchCreate(ctx, mirrors)
// 	require.Nil(t, err)
// 	ms, err := store.ToBeScheduled(ctx)
// 	require.Nil(t, err)
// 	require.Equal(t, 1, len(ms))
// 	require.Equal(t, ms[0].SourceUrl, "https://opencsg.com/models/repo1")
// }

func TestMirrorStoreDeferredLegacyDoesNotStarveReadyMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	future := time.Now().Add(time.Hour)
	var waiting []*database.Mirror
	for i := 0; i < 100; i++ {
		m := &database.Mirror{SourceUrl: "https://example.com/wait.git", Status: types.MirrorQueued, Priority: types.HighMirrorPriority, RemoteUpdatedAt: future}
		_, err := store.Create(ctx, m)
		require.NoError(t, err)
		require.NoError(t, store.DeferScheduledMirror(ctx, m.ID, future))
		waiting = append(waiting, m)
	}
	ready := &database.Mirror{SourceUrl: "https://example.com/ready.git", Status: types.MirrorQueued, Priority: types.LowMirrorPriority}
	_, err := store.Create(ctx, ready)
	require.NoError(t, err)
	rows, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, ready.ID, rows[0].ID)
	before, err := store.FindByID(ctx, waiting[0].ID)
	require.NoError(t, err)
	require.Equal(t, types.MirrorQueued, before.Status)
	require.True(t, before.UpdatedAt.Equal(waiting[0].UpdatedAt))
	// Expired deferrals remain eligible, but must not fill the next batch ahead of fresh work.
	for _, m := range waiting {
		require.NoError(t, store.DeferScheduledMirror(ctx, m.ID, time.Now().Add(-time.Minute)))
	}
	rows, err = store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 100)
	require.Equal(t, ready.ID, rows[0].ID)
}

func TestMirrorStoreFindBySourceURLsMatchesGitSuffix(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	a := &database.Mirror{SourceUrl: "https://example.com/org/one"}
	b := &database.Mirror{SourceUrl: "https://example.com/org/two.git"}
	for _, m := range []*database.Mirror{a, b, {SourceUrl: "https://other.com/org/one"}, {SourceUrl: "https://example.com/other/one"}} {
		_, err := store.Create(ctx, m)
		require.NoError(t, err)
	}
	rows, err := store.FindBySourceURLs(ctx, []string{a.SourceUrl + ".git", "https://example.com/org/two"})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.ElementsMatch(t, []int64{a.ID, b.ID}, []int64{rows[0].ID, rows[1].ID})
}

func TestMirrorStoreDeferDoesNotModifyBoundMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	m := &database.Mirror{RepositoryID: 42, SourceUrl: "https://example.com/bound.git", RemoteUpdatedAt: time.Now().Add(time.Hour)}
	_, err := store.Create(ctx, m)
	require.NoError(t, err)
	require.NoError(t, store.DeferScheduledMirror(ctx, m.ID, time.Now().Add(time.Hour)))
	current, err := store.FindByID(ctx, m.ID)
	require.NoError(t, err)
	require.True(t, current.NextExecutionTimestamp.IsZero())
	rows, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestMirrorStoreRetriesOldestDeferralBeforePriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	older := &database.Mirror{SourceUrl: "https://example.com/older.git", Status: types.MirrorQueued, Priority: types.LowMirrorPriority}
	newer := &database.Mirror{SourceUrl: "https://example.com/newer.git", Status: types.MirrorQueued, Priority: types.HighMirrorPriority}
	for _, m := range []*database.Mirror{older, newer} {
		_, err := store.Create(ctx, m)
		require.NoError(t, err)
	}
	require.NoError(t, store.DeferScheduledMirror(ctx, older.ID, time.Now().Add(-time.Hour)))
	require.NoError(t, store.DeferScheduledMirror(ctx, newer.ID, time.Now().Add(-time.Minute)))
	rows, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, older.ID, rows[0].ID)
}

func TestMirrorStoreActiveBoundTasksDoNotFillSchedulingBatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewMirrorStoreWithDB(db)
	tasks := database.NewMirrorTaskJobStoreWithDB(db)
	var first database.MirrorTask
	for i := 0; i < 100; i++ {
		mirror, err := store.Create(ctx, &database.Mirror{RepositoryID: int64(i + 1), SourceUrl: "https://example.com/busy.git", Priority: types.HighMirrorPriority, RemoteUpdatedAt: time.Now().Add(time.Hour)})
		require.NoError(t, err)
		statuses := []types.MirrorTaskStatus{types.MirrorLfsSyncStart, types.MirrorRepoSyncFailed, types.MirrorLfsSyncFailed}
		task, err := tasks.Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: statuses[i%len(statuses)]})
		require.NoError(t, err)
		require.NoError(t, tasks.SetMirrorCurrentTaskID(ctx, task))
		if i == 0 {
			first = task
		}
	}
	ready, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/ready.git", Status: types.MirrorQueued, Priority: types.LowMirrorPriority})
	require.NoError(t, err)
	rows, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, ready.ID, rows[0].ID)
	_, err = db.Core.NewUpdate().Model((*database.MirrorTask)(nil)).Set("status = ?", types.MirrorLfsSyncFinished).Where("id = ?", first.ID).Exec(ctx)
	require.NoError(t, err)
	rows, err = store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, first.MirrorID, rows[0].ID)
}

// TestMirrorStoreQueueCounts executes the simplified query across current tasks and River states.
func TestMirrorStoreQueueCounts(t *testing.T) {
	ctx := context.Background()
	db, err := database.NewDB(ctx, database.DBConfig{Dialect: database.DialectSQLite, DSN: "file:mirror_queue_counts?mode=memory&cache=shared", DisableAuditHook: true})
	require.NoError(t, err)
	defer db.Close()
	for _, ddl := range []string{
		"CREATE TABLE mirrors (id BIGINT PRIMARY KEY, repository_id BIGINT, current_task_id BIGINT)",
		"CREATE TABLE mirror_tasks (id BIGINT PRIMARY KEY, mirror_id BIGINT, status TEXT, repo_job_id BIGINT, lfs_job_id BIGINT)",
		"CREATE TABLE river_job (id BIGINT PRIMARY KEY, state TEXT)",
	} {
		_, err = db.Core.ExecContext(ctx, ddl)
		require.NoError(t, err)
	}
	store := database.NewMirrorStoreWithDB(db)
	empty, err := store.QueueCounts(ctx)
	require.NoError(t, err)
	require.Equal(t, types.MirrorMetricsQueues{}, empty)
	next := int64(0)
	add := func(status, state string, repoID int64) int64 {
		next++
		id := next
		_, err := db.Core.ExecContext(ctx, "INSERT INTO mirrors VALUES (?, ?, ?)", id, repoID, id)
		require.NoError(t, err)
		_, err = db.Core.ExecContext(ctx, "INSERT INTO mirror_tasks VALUES (?, ?, ?, ?, ?)", id, id, status, id, id+10000)
		require.NoError(t, err)
		_, err = db.Core.ExecContext(ctx, "INSERT INTO river_job VALUES (?, ?), (?, ?)", id, state, id+10000, state)
		require.NoError(t, err)
		return id
	}
	for _, status := range []string{"queued", "running", "repo_failed", "repo_synced", "lfs_start", "failed", "finished", "cancelled", "repo_fatal", "fatal", "paused"} {
		for _, state := range []string{"available", "scheduled", "retryable", "running", "completed", "cancelled", "discarded"} {
			add(status, state, next+1)
		}
	}
	// Each stage has three current task statuses times three waiting River states.
	counts, err := store.QueueCounts(ctx)
	require.NoError(t, err)
	require.Equal(t, types.MirrorMetricsQueues{Repo: types.MirrorMetricsQueue{Waiting: 9}, LFS: types.MirrorMetricsQueue{Waiting: 9}}, counts)
	// Two mirrors of one repository count once; each stage remains independent.
	add("queued", "available", 1000)
	add("repo_failed", "retryable", 1000)
	add("repo_synced", "scheduled", 1000)
	// An earlier completed mirror cannot hide another waiting mirror of the same repository.
	add("queued", "completed", 2000)
	add("queued", "available", 2000)
	stale := add("queued", "available", 3000)
	_, err = db.Core.ExecContext(ctx, "UPDATE mirrors SET current_task_id = NULL WHERE id = ?", stale)
	require.NoError(t, err)
	orphan := add("queued", "available", 3001)
	_, err = db.Core.ExecContext(ctx, "DELETE FROM river_job WHERE id = ?", orphan)
	require.NoError(t, err)
	mismatch := add("queued", "available", 3002)
	_, err = db.Core.ExecContext(ctx, "UPDATE mirror_tasks SET mirror_id = -1 WHERE id = ?", mismatch)
	require.NoError(t, err)
	// The stage-specific job ID chooses the Repo state rather than the LFS state.
	repo := add("queued", "running", 3003)
	_, err = db.Core.ExecContext(ctx, "UPDATE river_job SET state = 'available' WHERE id = ?", repo+10000)
	require.NoError(t, err)
	lfs := add("repo_synced", "running", 3004)
	_, err = db.Core.ExecContext(ctx, "UPDATE river_job SET state = 'available' WHERE id = ?", lfs)
	require.NoError(t, err)
	counts, err = store.QueueCounts(ctx)
	require.NoError(t, err)
	require.Equal(t, types.MirrorMetricsQueues{Repo: types.MirrorMetricsQueue{Waiting: 11}, LFS: types.MirrorMetricsQueue{Waiting: 10}}, counts)
}

// TestMirrorStoreQueueCountsAggregatesOnly checks bounded results without JSON identity or diagnostics.
func TestMirrorStoreQueueCountsAggregatesOnly(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	defer db.Close()
	store := database.NewMirrorStoreWithDB(&database.DB{Operator: database.Operator{Core: db}, BunDB: db})
	mock.ExpectQuery(`SELECT CASE .*COUNT\(DISTINCT m.repository_id\).*m.current_task_id.*JOIN river_job.*GROUP BY stage`).
		WillReturnRows(sqlmock.NewRows([]string{"stage", "waiting"}).AddRow("repo", 30000).AddRow("lfs", 5))
	counts, err := store.QueueCounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, 30000, counts.Repo.Waiting)
	require.Equal(t, 5, counts.LFS.Waiting)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestMirrorStoreQueueCountsEmptyAndError distinguishes empty queues from a failed query.
func TestMirrorStoreQueueCountsEmptyAndError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	defer db.Close()
	store := database.NewMirrorStoreWithDB(&database.DB{Operator: database.Operator{Core: db}, BunDB: db})
	mock.ExpectQuery(`SELECT CASE .*GROUP BY stage`).WillReturnRows(sqlmock.NewRows([]string{"stage", "waiting"}))
	counts, err := store.QueueCounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, types.MirrorMetricsQueues{}, counts)
	mock.ExpectQuery(`SELECT CASE .*GROUP BY stage`).WillReturnError(errors.New("query timeout"))
	_, err = store.QueueCounts(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
