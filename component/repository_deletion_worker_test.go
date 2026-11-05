package component

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

type deletionStep struct {
	steps *[]string
	err   error
}

func (s deletionStep) DeleteRepo(context.Context, string) error {
	*s.steps = append(*s.steps, "git")
	return s.err
}

type deletionResources struct {
	steps *[]string
	err   error
}

func (r deletionResources) DeleteRepositoryResources(context.Context, workhub.RepositoryDeletionArgs) error {
	*r.steps = append(*r.steps, "resources")
	return r.err
}

type deletionFinalizer struct {
	steps *[]string
	err   error
}

type deletionMirrorTaskFinder struct {
	steps  *[]string
	taskID int64
	err    error
}

func (f deletionMirrorTaskFinder) FindCurrentMirrorTaskID(context.Context, int64) (int64, error) {
	*f.steps = append(*f.steps, "mirror-find")
	return f.taskID, f.err
}

type deletionMirrorCanceler struct {
	steps   *[]string
	taskIDs []int64
	err     error
}

func (c *deletionMirrorCanceler) CancelMirror(_ context.Context, taskID int64) error {
	*c.steps = append(*c.steps, "mirror-cancel")
	c.taskIDs = append(c.taskIDs, taskID)
	return c.err
}

func (f deletionFinalizer) FinalizeRepositoryDeletion(context.Context, int64) error {
	*f.steps = append(*f.steps, "database")
	return f.err
}

func TestRepositoryDeletionWorkerCleansResourcesBeforeDatabase(t *testing.T) {
	steps := []string{}
	mirrorCanceler := &deletionMirrorCanceler{steps: &steps}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps},
		configuredDeletionAuthorizationStore(t, 42, 1),
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps, taskID: 88}, mirrorCanceler,
	)
	args := workhub.RepositoryDeletionArgs{
		RepositoryID: 42, RepositoryType: types.ModelRepo, GitalyPath: "models_org/repo.git",
		OwnerType: database.OrgNamespace, OwnerUUID: "org-uuid",
	}

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: args})
	require.NoError(t, err)
	require.Equal(t, []string{"mirror-find", "mirror-cancel", "resources", "git", "database"}, steps)
	require.Equal(t, []int64{88}, mirrorCanceler.taskIDs)

}

func TestRepositoryDeletionWorkerTreatsMissingGitRepositoryAsSuccess(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps, err: status.Error(codes.NotFound, "missing")},
		deletionResources{steps: &steps},
		configuredDeletionAuthorizationStore(t, 7, 1),
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{
		RepositoryID: 7, OwnerType: database.UserNamespace, OwnerUUID: "user-uuid",
	}})
	require.NoError(t, err)
	require.Equal(t, []string{"mirror-find", "resources", "git", "database"}, steps)
}

func TestRepositoryDeletionWorkerStopsBeforeDatabaseOnExternalFailure(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps, err: errors.New("storage unavailable")}, mockdb.NewMockRepositoryAuthorizationStore(t), deletionFinalizer{steps: &steps},
		deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{RepositoryID: 9}})
	require.ErrorContains(t, err, "delete repository resources")
	require.Equal(t, []string{"mirror-find", "resources"}, steps)
}

func TestRepositoryDeletionWorkerRetriesAfterAuthorizationCleanupWhenFinalizerFails(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps},
		configuredDeletionAuthorizationStore(t, 42, 2),
		deletionFinalizer{steps: &steps, err: errors.New("database unavailable")},
		deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)
	job := &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{
		RepositoryID: 42, OwnerType: database.UserNamespace, OwnerUUID: "user-uuid",
	}}

	require.ErrorContains(t, worker.Work(context.Background(), job), "finalize repository database deletion")
	worker.finalizer = deletionFinalizer{steps: &steps}
	require.NoError(t, worker.Work(context.Background(), job))
	require.Equal(t, []string{"mirror-find", "resources", "git", "database", "mirror-find", "resources", "git", "database"}, steps)
}

func TestRepositoryDeletionWorkerStopsBeforeResourcesWhenMirrorCancelFails(t *testing.T) {
	steps := []string{}
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps},
		mockdb.NewMockRepositoryAuthorizationStore(t),
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps, taskID: 99},
		&deletionMirrorCanceler{steps: &steps, err: errors.New("mirror unavailable")},
	)

	err := worker.Work(context.Background(), &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{RepositoryID: 9}})
	require.ErrorContains(t, err, "cancel repository mirror task")
	require.Equal(t, []string{"mirror-find", "mirror-cancel"}, steps)
}

// TestRepositoryDeletionWorkerStopsBeforeDatabaseWhenAuthorizationRecordCleanupFails verifies record failures are retryable.
func TestRepositoryDeletionWorkerStopsBeforeDatabaseWhenAuthorizationRecordCleanupFails(t *testing.T) {
	ctx := context.Background()
	steps := []string{}
	store := mockdb.NewMockRepositoryAuthorizationStore(t)
	store.EXPECT().DeleteByRepository(ctx, int64(42)).Return(errors.New("database unavailable")).Once()
	worker := NewRepositoryDeletionWorker(
		deletionStep{steps: &steps}, deletionResources{steps: &steps}, store,
		deletionFinalizer{steps: &steps}, deletionMirrorTaskFinder{steps: &steps}, &deletionMirrorCanceler{steps: &steps},
	)

	err := worker.Work(ctx, &river.Job[workhub.RepositoryDeletionArgs]{Args: workhub.RepositoryDeletionArgs{
		RepositoryID: 42, OwnerType: database.UserNamespace, OwnerUUID: "user-uuid",
	}})
	require.ErrorContains(t, err, "delete repository authorization records")
	require.Equal(t, []string{"mirror-find", "resources", "git"}, steps)
}

// configuredDeletionAuthorizationStore configures the direct authorization cleanup calls for a worker test.
func configuredDeletionAuthorizationStore(t *testing.T, repositoryID int64, calls int) *mockdb.MockRepositoryAuthorizationStore {
	t.Helper()
	store := mockdb.NewMockRepositoryAuthorizationStore(t)
	store.EXPECT().DeleteByRepository(mock.Anything, repositoryID).Return(nil).Times(calls)
	return store
}

// TestCleanRepositoryAuthorizations verifies only database grants are cleaned.
func TestCleanRepositoryAuthorizations(t *testing.T) {
	ctx := context.Background()
	store := mockdb.NewMockRepositoryAuthorizationStore(t)
	store.EXPECT().DeleteByRepository(ctx, int64(42)).Return(nil).Once()
	require.NoError(t, cleanRepositoryAuthorizations(ctx, store, 42))
}

// TestCleanRepositoryAuthorizationsReturnsRecordDeletionError verifies database cleanup errors are returned.
func TestCleanRepositoryAuthorizationsReturnsRecordDeletionError(t *testing.T) {
	ctx := context.Background()
	store := mockdb.NewMockRepositoryAuthorizationStore(t)
	store.EXPECT().DeleteByRepository(ctx, int64(42)).Return(errors.New("database unavailable")).Once()

	err := cleanRepositoryAuthorizations(ctx, store, 42)
	require.ErrorContains(t, err, "delete repository authorization records")
}
