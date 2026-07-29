package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeManagerJobClient satisfies workhub.JobClient for manager unit tests.
type fakeManagerJobClient struct{}

var _ workhub.JobClient = fakeManagerJobClient{}

// Insert is unused by manager cancellation tests.
func (fakeManagerJobClient) Insert(ctx context.Context, args workhub.JobArgs, opts *workhub.InsertOpts) (int64, error) {
	return 0, nil
}

// InsertTx is unused by manager cancellation tests.
func (fakeManagerJobClient) InsertTx(ctx context.Context, tx *sql.Tx, args workhub.JobArgs, opts *workhub.InsertOpts) (int64, error) {
	return 0, nil
}

// JobCancelTx is passed through to the task store and is not called directly by manager tests.
func (fakeManagerJobClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	return nil
}

// TestManagerComponentCancelUsesTaskJobStore verifies cancel passes the workhub job client into the transactional store API.
func TestManagerComponentCancelUsesTaskJobStore(t *testing.T) {
	ctx := context.TODO()
	taskID := int64(123)
	store := mockdb.NewMockMirrorTaskJobStore(t)
	syncCache := mockcache.NewMockCache(t)
	jobClient := fakeManagerJobClient{}
	manager := &managerComponentImpl{
		mirrorTaskStore: store,
		jobClient:       jobClient,
		syncCache:       syncCache,
		partSize:        "100",
	}
	store.EXPECT().
		FindByID(ctx, taskID).
		Return(&database.MirrorTask{
			ID: taskID,
			Mirror: &database.Mirror{
				RepositoryID: 456,
			},
		}, nil)
	store.EXPECT().
		CancelMirrorTaskByIDWithJobCancel(ctx, taskID, mock.MatchedBy(func(got any) bool {
			_, ok := got.(fakeManagerJobClient)
			return ok
		})).
		Return(true, nil)
	syncCache.EXPECT().DeleteRepoSyncCache(ctx, int64(456), "100").Return(nil)

	ok, err := manager.Cancel(ctx, taskID)

	require.NoError(t, err)
	require.True(t, ok)
}

// TestManagerComponentCancelReportsMissingTask verifies a false store result becomes a not-found style error.
func TestManagerComponentCancelReportsMissingTask(t *testing.T) {
	ctx := context.TODO()
	taskID := int64(456)
	store := mockdb.NewMockMirrorTaskJobStore(t)
	manager := &managerComponentImpl{
		mirrorTaskStore: store,
		jobClient:       fakeManagerJobClient{},
	}
	store.EXPECT().
		FindByID(ctx, taskID).
		Return(&database.MirrorTask{
			ID: taskID,
			Mirror: &database.Mirror{
				RepositoryID: 456,
			},
		}, nil)
	store.EXPECT().
		CancelMirrorTaskByIDWithJobCancel(ctx, taskID, mock.Anything).
		Return(false, nil)

	ok, err := manager.Cancel(ctx, taskID)

	require.False(t, ok)
	require.ErrorContains(t, err, "no task found")
}

// TestManagerComponentCancelWrapsStoreError verifies store failures are surfaced to the caller.
func TestManagerComponentCancelWrapsStoreError(t *testing.T) {
	ctx := context.TODO()
	taskID := int64(789)
	store := mockdb.NewMockMirrorTaskJobStore(t)
	manager := &managerComponentImpl{
		mirrorTaskStore: store,
		jobClient:       fakeManagerJobClient{},
	}
	storeErr := errors.New("cancel failed")
	store.EXPECT().
		FindByID(ctx, taskID).
		Return(&database.MirrorTask{
			ID: taskID,
			Mirror: &database.Mirror{
				RepositoryID: 456,
			},
		}, nil)
	store.EXPECT().
		CancelMirrorTaskByIDWithJobCancel(ctx, taskID, mock.Anything).
		Return(false, storeErr)

	ok, err := manager.Cancel(ctx, taskID)

	require.False(t, ok)
	require.ErrorIs(t, err, storeErr)
}

// TestManagerComponentListTasksGroupsRunningAndWaitingTasks verifies the mirror task list response only exposes active and pending work.
func TestManagerComponentListTasksGroupsRunningAndWaitingTasks(t *testing.T) {
	ctx := context.TODO()
	store := mockdb.NewMockMirrorTaskJobStore(t)
	manager := &managerComponentImpl{
		mirrorTaskStore: store,
		jobClient:       fakeManagerJobClient{},
	}
	runningStatuses := []types.MirrorTaskStatus{types.MirrorRepoSyncStart, types.MirrorLfsSyncStart}
	waitingStatuses := []types.MirrorTaskStatus{
		types.MirrorQueued,
		types.MirrorRepoSyncFinished,
		types.MirrorRepoSyncFailed,
		types.MirrorLfsSyncFailed,
	}
	repo := &database.Repository{Path: "ns/name", RepositoryType: types.ModelRepo}
	store.EXPECT().
		ListByStatusWithPriority(ctx, runningStatuses, 10, 1).
		Return([]database.MirrorTask{{
			ID:       1,
			MirrorID: 2,
			Priority: types.HighMirrorPriority,
			Mirror: &database.Mirror{
				SourceUrl:  "https://example.com/a.git",
				Repository: repo,
			},
		}}, nil)
	store.EXPECT().
		ListByStatusWithPriority(ctx, waitingStatuses, 10, 1).
		Return([]database.MirrorTask{{
			ID:       3,
			MirrorID: 4,
			Priority: types.MediumMirrorPriority,
			Mirror: &database.Mirror{
				SourceUrl:  "https://example.com/b.git",
				Repository: repo,
			},
		}}, nil)

	resp, err := manager.ListTasks(ctx, 10, 1)

	require.NoError(t, err)
	require.Len(t, resp.RunningTasks, 1)
	require.Equal(t, int64(1), resp.RunningTasks[0].TaskID)
	require.Equal(t, "models/ns/name", resp.RunningTasks[0].RepoPath)
	require.Len(t, resp.WaitingTasks, 1)
	require.Equal(t, int64(3), resp.WaitingTasks[0].TaskID)
}
