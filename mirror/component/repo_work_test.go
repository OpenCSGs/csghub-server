package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeRepoTaskStore records repo worker task state transitions.
type fakeRepoTaskStore struct {
	task        *database.MirrorTask
	actions     []string
	lfsInputs   []database.MirrorLFSJobInput
	insertedLFS bool
}

// FindByID returns the configured task.
func (s *fakeRepoTaskStore) FindByID(ctx context.Context, id int64) (*database.MirrorTask, error) {
	return s.task, nil
}

// UpdateStatusAndRepoSyncStatus records a plain task status transition.
func (s *fakeRepoTaskStore) UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, action string) (database.MirrorTask, error) {
	s.actions = append(s.actions, action)
	switch action {
	case database.MirrorContinue:
		task.Status = types.MirrorRepoSyncStart
	case database.MirrorFail:
		task.Status = types.MirrorRepoSyncFailed
	}
	s.task = &task
	return task, nil
}

// CompleteRepoSyncAndInsertLFSJob records the repo completion transaction.
func (s *fakeRepoTaskStore) CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error) {
	s.actions = append(s.actions, database.MirrorSuccess)
	s.lfsInputs = append(s.lfsInputs, input.JobInput)
	s.insertedLFS = true
	input.Task.Status = types.MirrorRepoSyncFinished
	if input.Task.Mirror != nil && input.Task.Mirror.Repository != nil {
		input.Task.Mirror.Repository.DefaultBranch = input.DefaultBranch
	}
	s.task = &input.Task
	return input.Task, nil
}

// fakeRepoSyncer returns the configured mirror task after repo sync.
type fakeRepoSyncer struct {
	task *database.MirrorTask
	err  error
}

// SyncRepo returns the configured sync result.
func (s fakeRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error) {
	if s.err != nil {
		return s.task, s.err
	}
	if s.task != nil {
		return s.task, nil
	}
	return mt, nil
}

// failRepoSyncer fails the test if a skipped repo job reaches the syncer.
type failRepoSyncer struct {
	t *testing.T
}

// SyncRepo fails because stale or terminal jobs must stop before repo sync.
func (s failRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error) {
	s.t.Fatalf("SyncRepo should not be called")
	return nil, nil
}

// fakeMirrorLFSJobClient satisfies the LFS job client dependency for config tests.
type fakeMirrorLFSJobClient struct{}

// InsertMirrorLFSJobTx records no job because config tests never enqueue work.
func (fakeMirrorLFSJobClient) InsertMirrorLFSJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorLFSJobInput) (int64, error) {
	return 0, nil
}

func TestRepoWorker_WorkAlwaysEnqueuesLFSJobAfterRepoSync(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	syncedTask := *task
	syncedTask.Status = types.MirrorRepoSyncStart
	syncedTask.Progress = 100
	syncedTask.BeforeLastCommitID = "before"
	syncedTask.AfterLastCommitID = "after"
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{task: &syncedTask},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.NoError(t, err)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.True(t, store.insertedLFS)
	require.Len(t, store.lfsInputs, 1)
	require.Equal(t, task.ID, store.lfsInputs[0].MirrorTaskID)
}

func TestRepoWorker_WorkEnqueuesLFSJobWhenRepoHasLFS(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	syncedTask := *task
	syncedTask.Status = types.MirrorRepoSyncStart
	syncedTask.Progress = 0
	syncedTask.BeforeLastCommitID = "before"
	syncedTask.AfterLastCommitID = "after"
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{task: &syncedTask},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.NoError(t, err)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.True(t, store.insertedLFS)
	require.Len(t, store.lfsInputs, 1)
	require.Equal(t, task.ID, store.lfsInputs[0].MirrorTaskID)
	require.Equal(t, task.MirrorID, store.lfsInputs[0].MirrorID)
	require.Equal(t, task.Mirror.RepositoryID, store.lfsInputs[0].RepositoryID)
	require.Equal(t, task.Mirror.SourceUrl, store.lfsInputs[0].SourceURL)
}

func TestRepoWorker_WorkMarksOriginalTaskFailedWhenSyncReturnsNilTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: errors.New("sync failed")},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.ErrorContains(t, err, "sync failed")
	require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
	require.Equal(t, "sync failed", store.task.ErrorMessage)
}

func TestRepoWorker_WorkReturnsCanceledWithoutFailingTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: context.Canceled},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
}

// TestRepoWorker_WorkSkipsCanceledTaskWithoutStateWrite verifies canceled tasks are terminal for repo jobs.
func TestRepoWorker_WorkSkipsCanceledTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.NoError(t, err)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

// TestRepoWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite verifies old repo jobs cannot update replaced tasks.
func TestRepoWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	task.Mirror.CurrentTaskID = task.ID + 1
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.NoError(t, err)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

func TestRepoWorker_WorkSnoozesWhenContextDeadlineStopsSync(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: context.DeadlineExceeded},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Duration(0), snoozeErr.Duration)
}

func TestNewRepoWorkClientRejectsMissingDependencies(t *testing.T) {
	ctx := context.TODO()

	_, err := NewRepoWorkClient(ctx, "", RepoWorkDeps{})
	require.ErrorContains(t, err, "mirror task store is required")

	_, err = NewRepoWorkClient(ctx, "", RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
	})
	require.ErrorContains(t, err, "repo syncer is required")

	_, err = NewRepoWorkClient(ctx, "", RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
	})
	require.ErrorContains(t, err, "LFS job client is required")
}

// TestNewRepoRiverConfigUsesConfiguredMaxWorkers verifies repo clients only consume the repo queue.
func TestNewRepoRiverConfigUsesConfiguredMaxWorkers(t *testing.T) {
	config := newRepoRiverConfig(RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
		LFSJobClient:    fakeMirrorLFSJobClient{},
		MaxWorkers:      7,
	})

	require.Equal(t, 7, config.Queues[workhub.MirrorRepoQueue].MaxWorkers)
	require.Len(t, config.Queues, 1)
	_, consumesLFS := config.Queues[workhub.MirrorLFSQueue]
	require.False(t, consumesLFS)
}

func TestNewRepoRiverConfigDefaultsMaxWorkers(t *testing.T) {
	config := newRepoRiverConfig(RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
		LFSJobClient:    fakeMirrorLFSJobClient{},
	})

	require.Equal(t, 1, config.Queues[workhub.MirrorRepoQueue].MaxWorkers)
}

// TestRepoWorkerTimeout verifies the real repo worker uses the shared workhub timeout contract.
func TestRepoWorkerTimeout(t *testing.T) {
	require.Equal(t, workhub.MirrorRepoJobTimeout, (&repoWorker{}).Timeout(&river.Job[workhub.RepoArgs]{}))
}

func TestShouldSkipRepoJobReportsReason(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	args := repoArgsFromTask(task)
	args.RepositoryID = 99

	skip, reason := shouldSkipRepoJob(task, args)

	require.True(t, skip)
	require.Equal(t, "repository_id_mismatch", reason)
}

func repoWorkerTask(status types.MirrorTaskStatus) *database.MirrorTask {
	repo := &database.Repository{
		ID:             11,
		Path:           "ns/repo",
		RepositoryType: types.ModelRepo,
		DefaultBranch:  types.MainBranch,
	}
	mirror := &database.Mirror{
		ID:            7,
		RepositoryID:  repo.ID,
		Repository:    repo,
		SourceUrl:     "https://github.com/upstream/repo.git",
		CurrentTaskID: 3,
		Priority:      types.ASAPMirrorPriority,
	}
	return &database.MirrorTask{
		ID:       3,
		MirrorID: mirror.ID,
		Mirror:   mirror,
		Status:   status,
		Priority: types.ASAPMirrorPriority,
	}
}

func repoArgsFromTask(task *database.MirrorTask) workhub.RepoArgs {
	return workhub.RepoArgs{
		MirrorID:     task.MirrorID,
		RepositoryID: task.Mirror.RepositoryID,
		MirrorTaskID: task.ID,
	}
}

func riverJob[T river.JobArgs](args T) *river.Job[T] {
	return &river.Job[T]{
		JobRow: &rivertype.JobRow{
			ID:          1,
			Attempt:     1,
			MaxAttempts: 3,
			Priority:    1,
		},
		Args: args,
	}
}
