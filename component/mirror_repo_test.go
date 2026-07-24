package component

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// fakeMirrorRepoStore records transactional mirror repo creation input for tests.
type fakeMirrorRepoStore struct {
	inputs []database.CreateMirrorRepoRecordsInput
	result *database.Mirror
}

// CreateMirrorRepoRecords stores the input and returns the mirror as if the transaction committed.
func (s *fakeMirrorRepoStore) CreateMirrorRepoRecords(ctx context.Context, input database.CreateMirrorRepoRecordsInput) (*database.Mirror, error) {
	s.inputs = append(s.inputs, input)
	if s.result != nil {
		return s.result, nil
	}
	mirror := input.Mirror
	if mirror.RepositoryID == 0 && input.Repository != nil {
		mirror.RepositoryID = input.Repository.ID
	}
	return &mirror, nil
}

// fakeWorkhubJobClient records workhub enqueue arguments for adapter tests.
type fakeWorkhubJobClient struct {
	tx     *sql.Tx
	args   workhub.JobArgs
	opts   *workhub.InsertOpts
	called bool
}

var _ workhub.JobClient = (*fakeWorkhubJobClient)(nil)

// Insert records non-transactional enqueue arguments.
func (c *fakeWorkhubJobClient) Insert(ctx context.Context, args workhub.JobArgs, opts *workhub.InsertOpts) (int64, error) {
	c.args = args
	c.opts = opts
	c.called = true
	return 123, nil
}

// InsertTx records transactional enqueue arguments.
func (c *fakeWorkhubJobClient) InsertTx(ctx context.Context, tx *sql.Tx, args workhub.JobArgs, opts *workhub.InsertOpts) (int64, error) {
	c.tx = tx
	c.args = args
	c.opts = opts
	c.called = true
	return 123, nil
}

// JobCancelTx records no data because mirror repo creation tests only enqueue jobs.
func (c *fakeWorkhubJobClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	return nil
}

// useFakeMirrorJobClient wires the fake workhub client and its repo adapter for mirror component tests.
func useFakeMirrorJobClient(mc *testMirrorWithMocks) *fakeWorkhubJobClient {
	jobClient := &fakeWorkhubJobClient{}
	mc.mirrorJobClient = jobClient
	mc.mirrorRepoJobClient = workhub.NewMirrorRepoJobClient(jobClient, workhub.MirrorJobClientConfig{MaxRetryCount: mc.config.Mirror.MaxRetryCount})
	return jobClient
}

// expectMirrorRepoRequeue injects a mocked transactional requeue store for duplicate mirror sync tests.
func expectMirrorRepoRequeue(ctx context.Context, t *testing.T, mc *testMirrorWithMocks, repo *database.Repository, mirror *database.Mirror, username, accessToken *string, priority types.MirrorPriority, urgent bool) {
	taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
	mc.mirrorTaskJobStore = taskJobStore
	useFakeMirrorJobClient(mc)
	taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
		credentialsMatch := input.Username == nil && input.AccessToken == nil && username == nil && accessToken == nil
		if username != nil && accessToken != nil && input.Username != nil && input.AccessToken != nil {
			credentialsMatch = *input.Username == *username && *input.AccessToken == *accessToken
		}
		return credentialsMatch &&
			input.MirrorID == mirror.ID &&
			input.RepositoryID == repo.ID &&
			input.Priority == priority &&
			input.Urgent == urgent &&
			input.JobClient != nil &&
			input.JobCancelClient != nil
	})).Return(database.MirrorTask{ID: 99}, nil)
}

// TestMirrorComponent_SyncMirrorRoutesRequestedQueue verifies manual sync preserves priority and the requested queue.
func TestMirrorComponent_SyncMirrorRoutesRequestedQueue(t *testing.T) {
	for _, urgent := range []bool{false, true} {
		t.Run(fmt.Sprintf("urgent_%t", urgent), func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			repo := &database.Repository{ID: 123, Path: "ns/name", RepositoryType: types.ModelRepo}
			mirror := &database.Mirror{
				ID: 456, RepositoryID: repo.ID, Repository: repo, Priority: types.HighMirrorPriority,
			}

			mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{RoleMask: "admin"}, nil)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "name").Return(repo, nil)
			mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
			expectMirrorRepoRequeue(ctx, t, mc, repo, mirror, nil, nil, types.HighMirrorPriority, urgent)

			err := mc.SyncMirror(ctx, types.SyncMirrorReq{
				RepoType: types.ModelRepo, Namespace: "ns", Name: "name", CurrentUser: "admin", Urgent: urgent,
			})

			require.NoError(t, err)
		})
	}
}

// TestMirrorComponent_SyncMirrorRequiresWritePermission verifies repository writers can sync while readers cannot.
func TestMirrorComponent_SyncMirrorRequiresWritePermission(t *testing.T) {
	for _, tc := range []struct {
		name       string
		canWrite   bool
		wantDenied bool
	}{
		{name: "allows writer", canWrite: true},
		{name: "rejects reader", wantDenied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			repo := &database.Repository{ID: 123, Path: "ns/name", RepositoryType: types.ModelRepo}
			mirror := &database.Mirror{
				ID: 456, RepositoryID: repo.ID, Repository: repo, Priority: types.LowMirrorPriority,
			}

			mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "member").Return(database.User{}, nil)
			mc.mocks.components.repo.EXPECT().
				CheckCurrentUserPermission(ctx, "member", "ns", membership.RoleWrite).
				Return(tc.canWrite, nil)
			if tc.canWrite {
				mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "name").Return(repo, nil)
				mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
				expectMirrorRepoRequeue(ctx, t, mc, repo, mirror, nil, nil, types.LowMirrorPriority, false)
			}

			err := mc.SyncMirror(ctx, types.SyncMirrorReq{
				RepoType: types.ModelRepo, Namespace: "ns", Name: "name", CurrentUser: "member",
			})

			if tc.wantDenied {
				require.ErrorIs(t, err, errorx.ErrForbidden)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMirrorComponent_MirrorFromSaas(t *testing.T) {
	t.Run("creates mirror records and repo job for existing repo without mirror", func(t *testing.T) {
		ctx := context.TODO()
		mc := initializeTestMirrorComponent(ctx, t)
		mc.config.MultiSync.SaasSyncDomain = "https://saas.test"
		fakeStore := &fakeMirrorRepoStore{result: &database.Mirror{
			ID:            456,
			RepositoryID:  123,
			CurrentTaskID: 789,
			CurrentTask:   &database.MirrorTask{ID: 789, Status: types.MirrorQueued},
		}}
		mc.mirrorRepoStore = fakeStore

		repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: types.ModelRepo, Source: types.OpenCSGSource}
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil)
		mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(nil, sql.ErrNoRows)
		mc.mocks.stores.SyncVersionMock().EXPECT().FindByRepoTypeAndPath(ctx, "ns/n", types.ModelRepo).Return(&database.SyncVersion{SourceID: types.SyncVersionSourceOpenCSG}, nil)

		result, err := mc.MirrorFromSaas(ctx, types.MirrorFromSaasReq{
			Namespace:   "CSG_ns",
			Name:        "n",
			RepoType:    types.ModelRepo,
			CurrentUser: "writer",
		})
		require.NoError(t, err)
		require.Equal(t, &types.MirrorFromSaasResponse{
			RepositoryID: 123,
			MirrorID:     456,
			TaskID:       789,
			Status:       types.MirrorQueued,
		}, result)
		require.Len(t, fakeStore.inputs, 1)
		input := fakeStore.inputs[0]
		require.False(t, input.CreateRepository)
		require.Equal(t, repo, input.Repository)
		require.Equal(t, types.ModelRepo, input.Repository.RepositoryType)
		require.Equal(t, "https://saas.test/models/ns/n.git", input.Mirror.SourceUrl)
		require.Equal(t, int64(123), input.Mirror.RepositoryID)
		require.Equal(t, repo, input.Mirror.Repository)
		require.Equal(t, "CSG_ns/n", input.Mirror.SourceRepoPath)
		require.Equal(t, types.MediumMirrorPriority, input.Mirror.Priority)
		require.False(t, input.Urgent)
	})

	t.Run("requeues existing mirror through workhub", func(t *testing.T) {
		ctx := context.TODO()
		mc := initializeTestMirrorComponent(ctx, t)
		taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
		mc.mirrorTaskJobStore = taskJobStore
		useFakeMirrorJobClient(mc)

		repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: types.ModelRepo, Source: types.OpenCSGSource}
		mirror := &database.Mirror{ID: 1, SourceUrl: "https://saas.test/models/ns/n.git", RepositoryID: 123, Repository: repo}
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil)
		mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(mirror, nil)
		taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
			return input.MirrorID == mirror.ID &&
				input.RepositoryID == repo.ID &&
				input.Priority == types.LowMirrorPriority &&
				!input.Urgent &&
				input.JobClient != nil &&
				input.JobCancelClient != nil
		})).Return(database.MirrorTask{ID: 99, Status: types.MirrorQueued}, nil)

		result, err := mc.MirrorFromSaas(ctx, types.MirrorFromSaasReq{
			Namespace:   "CSG_ns",
			Name:        "n",
			RepoType:    types.ModelRepo,
			CurrentUser: "writer",
		})
		require.NoError(t, err)
		require.Equal(t, &types.MirrorFromSaasResponse{
			RepositoryID: 123,
			MirrorID:     1,
			TaskID:       99,
			Status:       types.MirrorQueued,
		}, result)
	})

	t.Run("rejects users without write permission", func(t *testing.T) {
		ctx := context.TODO()
		mc := initializeTestMirrorComponent(ctx, t)
		repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: types.ModelRepo}
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)

		result, err := mc.MirrorFromSaas(ctx, types.MirrorFromSaasReq{
			Namespace:   "CSG_ns",
			Name:        "n",
			RepoType:    types.ModelRepo,
			CurrentUser: "reader",
		})
		require.ErrorIs(t, err, errorx.ErrForbidden)
		require.Nil(t, result)
	})
}

// TestMirrorComponent_MirrorFromSaasStatus verifies public sync status is composed from persisted task state.
func TestMirrorComponent_MirrorFromSaasStatus(t *testing.T) {
	ctx := context.TODO()
	updatedAt := time.Now()
	repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: types.ModelRepo}

	tests := []struct {
		name      string
		task      *database.MirrorTask
		requested int64
		want      *types.MirrorSyncStatusResponse
	}{
		{
			name:      "reports running repo sync from task state",
			task:      &database.MirrorTask{ID: 7, MirrorID: 8, Status: types.MirrorRepoSyncStart, RepoJobID: 70},
			requested: 7,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 7,
				Status: types.MirrorRepoSyncStart, Phase: types.MirrorSyncPhaseRepo,
				UpdatedAt: updatedAt,
			},
		},
		{
			name:      "reports fatal repo task as terminal",
			task:      &database.MirrorTask{ID: 9, MirrorID: 8, Status: types.MirrorRepoSyncFatal, RepoJobID: 90},
			requested: 7,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 9,
				Status: types.MirrorRepoSyncFatal, Phase: types.MirrorSyncPhaseRepo,
				Terminal: true, Superseded: true,
				FailureReason: types.MirrorSyncFailureRepoSyncFailed,
				UpdatedAt:     updatedAt,
			},
		},
		{
			name:      "reports retryable repo failure from task state",
			task:      &database.MirrorTask{ID: 11, MirrorID: 8, Status: types.MirrorRepoSyncFailed, RepoJobID: 110},
			requested: 11,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 11,
				Status: types.MirrorRepoSyncFailed, Phase: types.MirrorSyncPhaseRepo,
				Retrying: true, UpdatedAt: updatedAt,
			},
		},
		{
			name:      "reports fatal LFS task without blocking Git data",
			task:      &database.MirrorTask{ID: 12, MirrorID: 8, Status: types.MirrorLfsSyncFatal, LFSJobID: 120},
			requested: 12,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 12,
				Status: types.MirrorLfsSyncFatal, Phase: types.MirrorSyncPhaseDone,
				RepoReady: true, Terminal: true,
				FailureReason: types.MirrorSyncFailureLFSSyncFailed,
				UpdatedAt:     updatedAt,
			},
		},
		{
			name:      "reports cancelled task as terminal",
			task:      &database.MirrorTask{ID: 13, MirrorID: 8, Status: types.MirrorCanceled},
			requested: 13,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 13,
				Status: types.MirrorCanceled, Phase: types.MirrorSyncPhaseDone,
				Terminal: true, FailureReason: types.MirrorSyncFailureCanceled,
				UpdatedAt: updatedAt,
			},
		},
		{
			name:      "reports completed LFS sync without querying workhub",
			task:      &database.MirrorTask{ID: 10, MirrorID: 8, Status: types.MirrorLfsSyncFinished, Progress: 100},
			requested: 10,
			want: &types.MirrorSyncStatusResponse{
				RepositoryID: 123, MirrorID: 8, TaskID: 10,
				Status: types.MirrorLfsSyncFinished, Phase: types.MirrorSyncPhaseDone,
				RepoReady: true, Terminal: true, Progress: 100,
				UpdatedAt: updatedAt,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := initializeTestMirrorComponent(ctx, t)
			tt.task.UpdatedAt = updatedAt
			mirror := &database.Mirror{ID: 8, RepositoryID: repo.ID, CurrentTaskID: tt.task.ID, CurrentTask: tt.task}
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
			mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
			mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

			got, err := mc.MirrorFromSaasStatus(ctx, types.MirrorFromSaasStatusReq{
				Namespace: "CSG_ns", Name: "n", RepoType: types.ModelRepo,
				CurrentUser: "reader", RequestedTaskID: tt.requested,
			})
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("rejects users without read permission", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "guest", repo).Return(&types.UserRepoPermission{}, nil)

		got, err := mc.MirrorFromSaasStatus(ctx, types.MirrorFromSaasStatusReq{
			Namespace: "CSG_ns", Name: "n", RepoType: types.ModelRepo, CurrentUser: "guest",
		})
		require.ErrorIs(t, err, errorx.ErrForbidden)
		require.Nil(t, got)
	})

	t.Run("rejects a nonterminal task without a job reference", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		task := &database.MirrorTask{ID: 14, MirrorID: 8, Status: types.MirrorRepoSyncStart}
		mirror := &database.Mirror{ID: 8, RepositoryID: repo.ID, CurrentTaskID: task.ID, CurrentTask: task}
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "reader", repo).Return(&types.UserRepoPermission{CanRead: true}, nil)
		mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

		got, err := mc.MirrorFromSaasStatus(ctx, types.MirrorFromSaasStatusReq{
			Namespace: "CSG_ns", Name: "n", RepoType: types.ModelRepo, CurrentUser: "reader",
		})
		require.ErrorIs(t, err, errorx.ErrMirrorTaskStateInvalid)
		require.Nil(t, got)
	})
}

// TestMirrorComponent_DeleteMirror verifies manual mirror deletion uses the transactional delete path.
func TestMirrorComponent_DeleteMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	jobClient := &fakeWorkhubJobClient{}
	syncCache := mockcache.NewMockCache(t)
	mc.mirrorJobClient = jobClient
	mc.syncCache = syncCache
	mc.config.Mirror.PartSize = 100
	repo := &database.Repository{ID: 123}
	mirror := &database.Mirror{ID: 321, RepositoryID: repo.ID}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
	mc.mocks.stores.MirrorMock().EXPECT().DeleteWithTaskCancelTx(ctx, mirror.ID, jobClient).Return(nil)
	syncCache.EXPECT().DeleteRepoSyncCache(ctx, repo.ID, "100").Return(nil)

	err := mc.DeleteMirror(ctx, types.DeleteMirrorReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.NoError(t, err)
}

// TestMirrorComponent_GetMirror verifies mirror configuration reads are owned by MirrorComponent.
func TestMirrorComponent_GetMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	dm := &database.Mirror{ID: 11, SourceUrl: "test", Repository: &database.Repository{Path: "test/abc", RepositoryType: types.ModelRepo}}
	m := &types.Mirror{ID: 11, SourceUrl: "test", LocalRepoPath: "models/test/abc"}
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(dm, nil)
	mm, err := mc.GetMirror(ctx, types.GetMirrorReq{
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, m, mm)
}

// TestMirrorComponent_UpdateMirror verifies mirror configuration writes normalize embedded source credentials.
func TestMirrorComponent_UpdateMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	m := database.Mirror{
		ID:              123,
		SourceUrl:       "https://example.com/source/repo.git",
		Username:        "user",
		AccessToken:     "ak",
		PushUsername:    "user",
		PushAccessToken: "foo",
		LocalRepoPath:   "model_ns_n",
		MirrorSourceID:  111,
	}
	mi := m
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(&mi, nil)
	mc.mocks.stores.AccessTokenMock().EXPECT().GetUserGitToken(ctx, "user").Return(&database.AccessToken{Token: "foo"}, nil)
	mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, &m).Return(nil)

	mm, err := mc.UpdateMirror(ctx, types.UpdateMirrorReq{
		Namespace:      "ns",
		CurrentUser:    "user",
		SourceUrl:      "https://user:ak@example.com/source/repo",
		RepoType:       types.ModelRepo,
		Name:           "n",
		MirrorSourceID: 111,
	})
	require.Nil(t, err)
	require.Equal(t, m, *mm)
}

// TestMirrorComponent_CreateMirror verifies existing repositories are bound to mirrors through the transactional store.
func TestMirrorComponent_CreateMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	repo := &database.Repository{
		ID:             123,
		Path:           "ns/n",
		HTTPCloneURL:   "https://opencsg.com/models/ns/n.git",
		RepositoryType: types.ModelRepo,
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(repo, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(321)).Return(&database.MirrorSource{
		SourceName: "github",
	}, nil)

	got, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
		SourceUrl:      "https://source-user:source-token@github.com/upstream/repo",
		CurrentUser:    "user",
		Namespace:      "ns",
		Name:           "n",
		RepoType:       types.ModelRepo,
		MirrorSourceID: 321,
		SourceRepoPath: "upstream/repo",
	})
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.RepositoryID)
	require.Len(t, fakeStore.inputs, 1)
	require.False(t, fakeStore.inputs[0].CreateRepository)
	require.Equal(t, repo, fakeStore.inputs[0].Repository)
	require.Equal(t, types.ModelRepo, fakeStore.inputs[0].Repository.RepositoryType)
	require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
	require.Equal(t, database.Mirror{
		SourceUrl:      "https://github.com/upstream/repo.git",
		MirrorSourceID: 321,
		Username:       "source-user",
		AccessToken:    "source-token",
		PushUrl:        "https://opencsg.com/models/ns/n.git",
		SourceRepoPath: "upstream/repo",
		LocalRepoPath:  "github_model_ns_n",
		RepositoryID:   repo.ID,
		Repository:     repo,
		Priority:       types.LowMirrorPriority,
	}, fakeStore.inputs[0].Mirror)
}

// TestMirrorComponent_CreateMirrorRepoRejectsEmptyCurrentUser verifies mirror creation requires an explicit current user.
func TestMirrorComponent_CreateMirrorRepoRejectsEmptyCurrentUser(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	got, err := mc.CreateMirrorRepo(ctx, types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
	})
	require.Error(t, err)
	require.Nil(t, got)
}

// TestMirrorComponent_CreateMirrorRepoNormalizesForkTarget verifies local mirror target identifiers are trimmed and lowercased.
func TestMirrorComponent_CreateMirrorRepoNormalizesForkTarget(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "  Alice-Team ",
		ForkName:          " Qwen-Model  ",
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice-team", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice-team", "qwen-model").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "alice-team").Return(database.Namespace{
		Path: "alice-team",
	}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
		RoleMask: "admin",
	}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	require.Equal(t, "alice-team/qwen-model", fakeStore.inputs[0].Repository.Path)
	require.Equal(t, "qwen-model", fakeStore.inputs[0].Repository.Name)
	require.Equal(t, "models_alice-team/qwen-model", fakeStore.inputs[0].Repository.GitPath)
	require.Equal(t, "github_model_alice-team_qwen-model", fakeStore.inputs[0].Mirror.LocalRepoPath)
}

// TestMirrorComponent_CreateMirrorRepoPersistsNormalizedSourceAndCredentials verifies normalized source data reaches the mirror record.
func TestMirrorComponent_CreateMirrorRepoPersistsNormalizedSourceAndCredentials(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://source-user:source-token@github.com/upstream/repo/",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		Priority:          types.ASAPMirrorPriority,
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{
		Path: "alice",
	}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
		RoleMask: "admin",
	}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	require.Equal(t, "https://github.com/upstream/repo.git", fakeStore.inputs[0].Mirror.SourceUrl)
	require.Equal(t, "source-user", fakeStore.inputs[0].Mirror.Username)
	require.Equal(t, "source-token", fakeStore.inputs[0].Mirror.AccessToken)
	require.Equal(t, types.ASAPMirrorPriority, fakeStore.inputs[0].Mirror.Priority)
	require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
}

// TestMirrorComponent_CreateMirrorRepoUsesExplicitVisibility verifies callers can override the default private visibility.
func TestMirrorComponent_CreateMirrorRepoUsesExplicitVisibility(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	private := false

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.CodeRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		Private:           &private,
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{
		Path: "alice",
	}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
		RoleMask: "admin",
	}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	require.False(t, fakeStore.inputs[0].Repository.Private)
}

// TestMirrorComponent_CreateMirrorRepoRejectsInvalidSourceURL verifies only HTTP(S) Git clone URLs with repository paths are accepted.
func TestMirrorComponent_CreateMirrorRepoRejectsInvalidSourceURL(t *testing.T) {
	cases := []string{
		"git@github.com:upstream/repo.git",
		"ftp://github.com/upstream/repo.git",
		"https://github.com",
		"https://github.com/upstream/repo.git?token=abc",
	}

	for _, sourceURL := range cases {
		t.Run(sourceURL, func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)

			got, err := mc.CreateMirrorRepo(ctx, types.CreateMirrorRepoReq{
				SourceNamespace:   "upstream",
				SourceName:        "repo",
				RepoType:          types.ModelRepo,
				CurrentUser:       "admin",
				SourceGitCloneUrl: sourceURL,
				ForkNamespace:     "alice",
				ForkName:          "forked",
			})
			require.Error(t, err)
			require.ErrorIs(t, err, errorx.ErrBadRequest)
			require.Nil(t, got)
			customErr, ok := errorx.GetFirstCustomError(err)
			require.True(t, ok)
			require.NotContains(t, customErr.(errorx.CustomError).Context(), "source url")
		})
	}
}

// TestNormalizeMirrorPriority verifies omitted and supported priorities are normalized consistently.
func TestNormalizeMirrorPriority(t *testing.T) {
	tests := []struct {
		name         string
		priority     types.MirrorPriority
		wantPriority types.MirrorPriority
		wantErr      bool
	}{
		{name: "omitted", wantPriority: types.LowMirrorPriority},
		{name: "asap", priority: types.ASAPMirrorPriority, wantPriority: types.ASAPMirrorPriority},
		{name: "high", priority: types.HighMirrorPriority, wantPriority: types.HighMirrorPriority},
		{name: "medium", priority: types.MediumMirrorPriority, wantPriority: types.MediumMirrorPriority},
		{name: "low", priority: types.LowMirrorPriority, wantPriority: types.LowMirrorPriority},
		{name: "below range", priority: -1, wantErr: true},
		{name: "above range", priority: 5, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority, err := normalizeMirrorPriority(tt.priority)
			if tt.wantErr {
				require.ErrorIs(t, err, errorx.ErrBadRequest)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantPriority, priority)
		})
	}
}

// TestNormalizeMirrorSource verifies source URLs and credentials are canonicalized together.
func TestNormalizeMirrorSource(t *testing.T) {
	tests := []struct {
		name         string
		sourceURL    string
		username     string
		accessToken  string
		wantURL      string
		wantUsername string
		wantToken    string
		wantErr      error
	}{
		{name: "anonymous", sourceURL: " https://example.com/ns/repo/ ", wantURL: "https://example.com/ns/repo.git"},
		{name: "URL credentials", sourceURL: "https://url-user:url-token@example.com/ns/repo", wantURL: "https://example.com/ns/repo.git", wantUsername: "url-user", wantToken: "url-token"},
		{name: "encoded URL credentials", sourceURL: "https://url%2Duser:tok%40en%3Avalue@example.com/ns/repo.git", wantURL: "https://example.com/ns/repo.git", wantUsername: "url-user", wantToken: "tok@en:value"},
		{name: "explicit credentials", sourceURL: "https://example.com/ns/repo", username: "user", accessToken: "token", wantURL: "https://example.com/ns/repo.git", wantUsername: "user", wantToken: "token"},
		{name: "explicit credentials with HTTP URL", sourceURL: "http://example.com/ns/repo.git", username: "user", accessToken: "token", wantURL: "http://example.com/ns/repo.git", wantUsername: "user", wantToken: "token"},
		{name: "explicit credentials without URL", username: "user", accessToken: "token", wantErr: errorx.ErrBadRequest},
		{name: "SSH URL", sourceURL: "ssh://example.com/ns/repo.git", wantErr: errorx.ErrBadRequest},
		{name: "URL without host", sourceURL: "https:/ns/repo.git", wantErr: errorx.ErrBadRequest},
		{name: "URL without path", sourceURL: "https://example.com", wantErr: errorx.ErrBadRequest},
		{name: "URL with query", sourceURL: "https://example.com/ns/repo?token=value", wantErr: errorx.ErrBadRequest},
		{name: "URL with fragment", sourceURL: "https://example.com/ns/repo#main", wantErr: errorx.ErrBadRequest},
		{name: "explicit username only", sourceURL: "https://example.com/ns/repo.git", username: "user", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
		{name: "explicit token only", sourceURL: "https://example.com/ns/repo.git", accessToken: "token", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
		{name: "duplicate credentials", sourceURL: "https://url-user:url-token@example.com/ns/repo.git", username: "user", accessToken: "token", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
		{name: "URL username only", sourceURL: "https://url-user@example.com/ns/repo.git", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
		{name: "URL token only", sourceURL: "https://:url-token@example.com/ns/repo.git", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
		{name: "URL empty token", sourceURL: "https://url-user:@example.com/ns/repo.git", wantErr: errorx.ErrMirrorSourceRepoAuthInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceURL, username, accessToken, err := normalizeMirrorSource(
				tt.sourceURL, tt.username, tt.accessToken,
			)
			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantURL, sourceURL)
			require.Equal(t, tt.wantUsername, username)
			require.Equal(t, tt.wantToken, accessToken)
		})
	}
}

// TestMirrorWriteEntrypointsValidateCredentials verifies all mirror writes reject incomplete credentials early.
func TestMirrorWriteEntrypointsValidateCredentials(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateMirrorRepo", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		_, err := mc.CreateMirrorRepo(ctx, types.CreateMirrorRepoReq{
			SourceNamespace:   "source",
			SourceName:        "repo",
			RepoType:          types.ModelRepo,
			SourceGitCloneUrl: "https://example.com/source/repo.git",
			Username:          "user",
			CurrentUser:       "owner",
			ForkNamespace:     "owner",
			ForkName:          "repo",
		})
		require.ErrorContains(t, err, "username and access token must be provided together")
		require.ErrorIs(t, err, errorx.ErrMirrorSourceRepoAuthInvalid)
	})

	t.Run("CreateMirror", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		_, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
			SourceUrl: "https://example.com/source/repo.git",
			Username:  "user",
		})
		require.ErrorContains(t, err, "username and access token must be provided together")
		require.ErrorIs(t, err, errorx.ErrMirrorSourceRepoAuthInvalid)
	})

	t.Run("UpdateMirror", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		_, err := mc.UpdateMirror(ctx, types.UpdateMirrorReq{
			SourceUrl:   "https://example.com/source/repo.git",
			AccessToken: "token",
		})
		require.ErrorContains(t, err, "username and access token must be provided together")
		require.ErrorIs(t, err, errorx.ErrMirrorSourceRepoAuthInvalid)
	})

	t.Run("BatchCreate", func(t *testing.T) {
		mc := initializeTestMirrorComponent(ctx, t)
		err := mc.BatchCreate(ctx, types.BatchCreateMirrorReq{Mirrors: []types.MirrorReq{{
			SourceURL:   "https://example.com/source/repo.git",
			AccessToken: "token",
		}}})
		require.ErrorContains(t, err, "username and access token must be provided together")
		require.ErrorIs(t, err, errorx.ErrMirrorSourceRepoAuthInvalid)
	})
}

// TestMirrorComponent_CreateMirrorRepoRequeuesSameTargetAndSource verifies repeat sync is scoped to the target repo mirror.
func TestMirrorComponent_CreateMirrorRepoRequeuesSameTargetAndSource(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://new-user:new-token@github.com/upstream/repo",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		Priority:          types.ASAPMirrorPriority,
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{ID: 3, RepositoryID: repo.ID, SourceUrl: "https://github.com/upstream/repo.git"}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
	username, accessToken := "new-user", "new-token"
	expectMirrorRepoRequeue(ctx, t, mc, repo, mirror, &username, &accessToken, types.ASAPMirrorPriority, false)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, mirror.ID, got.ID)
	require.Equal(t, username, got.Username)
	require.Equal(t, accessToken, got.AccessToken)
}

// TestMirrorComponent_CreateMirrorRepoRequeuePreservesCredentials verifies omitted credentials do not clear stored values.
func TestMirrorComponent_CreateMirrorRepoRequeuePreservesCredentials(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo",
		ForkNamespace:     "alice",
		ForkName:          "forked",
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{
		ID: 3, RepositoryID: repo.ID, SourceUrl: "https://github.com/upstream/repo.git",
		Username: "old-user", AccessToken: "old-token",
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
	expectMirrorRepoRequeue(ctx, t, mc, repo, mirror, nil, nil, types.LowMirrorPriority, false)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "old-user", got.Username)
	require.Equal(t, "old-token", got.AccessToken)
}

// TestMirrorComponent_CreateMirrorRepoAddsSourceToExistingTargetWithoutMirror verifies an existing repo without a mirror can be bound to one source URL.
func TestMirrorComponent_CreateMirrorRepoAddsSourceToExistingTargetWithoutMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	createTargetRepo := false

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.DatasetRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		CreateTargetRepo:  &createTargetRepo,
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.DatasetRepo}

	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.RepositoryID)
	require.Len(t, fakeStore.inputs, 1)
	require.Equal(t, repo, fakeStore.inputs[0].Repository)
	require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
	require.Empty(t, fakeStore.inputs[0].Mirror.Username)
	require.Equal(t, "github_dataset_alice_forked", fakeStore.inputs[0].Mirror.LocalRepoPath)
}

// TestMirrorComponent_CreateMirrorRepoRejectsExistingTargetWhenRequested verifies callers can keep import-style no-overwrite semantics.
func TestMirrorComponent_CreateMirrorRepoRejectsExistingTargetWhenRequested(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	createTargetRepo := true

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.CodeRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		CreateTargetRepo:  &createTargetRepo,
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.CodeRepo}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, errorx.ErrRepoAlreadyExist)
	require.Nil(t, got)
}

// TestMirrorComponent_CreateMirrorRepoRejectsMissingTargetWhenCreationIsDisabled verifies callers can require an existing target.
func TestMirrorComponent_CreateMirrorRepoRejectsMissingTargetWhenCreationIsDisabled(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	createTargetRepo := false
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.CodeRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		CreateTargetRepo:  &createTargetRepo,
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(nil, sql.ErrNoRows)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.ErrorIs(t, err, errorx.ErrRepoNotFound)
	require.Nil(t, got)
}

// TestMirrorComponent_CreateMirrorRepoRejectsExistingTargetWithDifferentSource keeps one repo bound to at most one mirror source URL.
func TestMirrorComponent_CreateMirrorRepoRejectsExistingTargetWithDifferentSource(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{ID: 3, RepositoryID: repo.ID, SourceUrl: "https://github.com/other/repo.git"}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, errorx.ErrMirrorSourceConflict)
	require.Equal(t, repo.ID, got.RepositoryID)
}

// TestMirrorComponent_CreateMirrorRepoRejectsMissingWritePermission verifies write access is checked before mirror creation.
func TestMirrorComponent_CreateMirrorRepoRejectsMissingWritePermission(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(false, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.Error(t, err)
	require.Nil(t, got)
}

// TestMirrorComponent_CreateMirrorRepoCreatesAllMirrorRepoTypes verifies new mirror targets use the transactional store for every type supported by CreateMirrorRepo.
func TestMirrorComponent_CreateMirrorRepoCreatesAllMirrorRepoTypes(t *testing.T) {
	cases := []types.RepositoryType{
		types.ModelRepo,
		types.DatasetRepo,
		types.CodeRepo,
		types.MCPServerRepo,
		types.SkillRepo,
	}

	for _, repoType := range cases {
		t.Run(string(repoType), func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			fakeStore := &fakeMirrorRepoStore{}
			mc.mirrorRepoStore = fakeStore
			createTargetRepo := true

			req := types.CreateMirrorRepoReq{
				SourceNamespace:   "upstream",
				SourceName:        "repo",
				RepoType:          repoType,
				CurrentUser:       "admin",
				SourceGitCloneUrl: "https://github.com/upstream/repo.git",
				ForkName:          "forked",
				CreateTargetRepo:  &createTargetRepo,
			}
			if repoType == types.MCPServerRepo {
				req.MCPServerAttributes = types.MCPServerAttributes{
					Tools: []types.MCPTool{
						{
							Name:        "search",
							Description: "Search things",
							InputSchema: types.MCPToolSchema{
								Required: []string{"query"},
								Type:     "object",
							},
						},
					},
					Configuration: types.MCPSchema{Type: "object"},
					AvatarURL:     "https://example.com/avatar.png",
				}
			}

			mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().FindBySourceNamespace(context.Background(), "upstream").Return(&database.MirrorNamespaceMapping{
				TargetNamespace: "mapped",
			}, nil)
			mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "mapped", membership.RoleWrite).Return(true, nil)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, repoType, "mapped", "forked").Return(nil, sql.ErrNoRows)
			mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "mapped").Return(database.Namespace{
				Path: "mapped",
			}, nil)
			mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
				ID:       1,
				Username: "admin",
				Email:    "admin@example.com",
				RoleMask: "admin",
			}, nil)

			got, err := mc.CreateMirrorRepo(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, fakeStore.inputs, 1)
			require.Equal(t, repoType, fakeStore.inputs[0].Repository.RepositoryType)
			require.Equal(t, types.LowMirrorPriority, fakeStore.inputs[0].Mirror.Priority)
			require.Equal(t, "mapped/forked", fakeStore.inputs[0].Repository.Path)
			require.Equal(t, string(repoType)+"s_mapped/forked", fakeStore.inputs[0].Repository.GitPath)
			require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
			if repoType == types.MCPServerRepo {
				require.NotNil(t, fakeStore.inputs[0].MCPServer)
				require.Equal(t, 1, fakeStore.inputs[0].MCPServer.ToolsNum)
				require.Equal(t, "https://example.com/avatar.png", fakeStore.inputs[0].MCPServer.AvatarURL)
				require.Contains(t, fakeStore.inputs[0].MCPServer.Configuration, `"type":"object"`)
				require.Contains(t, fakeStore.inputs[0].MCPServer.Schema, `"name":"search"`)
				require.Len(t, fakeStore.inputs[0].MCPServerProperties, 1)
				require.Equal(t, types.MCPPropTool, fakeStore.inputs[0].MCPServerProperties[0].Kind)
				require.Equal(t, "search", fakeStore.inputs[0].MCPServerProperties[0].Name)
				require.Equal(t, "Search things", fakeStore.inputs[0].MCPServerProperties[0].Description)
				require.Contains(t, fakeStore.inputs[0].MCPServerProperties[0].Schema, `"required":["query"]`)
			}
		})
	}
}
