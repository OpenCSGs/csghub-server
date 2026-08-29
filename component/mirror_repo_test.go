package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/multisync"
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

// TestMirrorComponent_MirrorFromSaas verifies SaaS sync creation, requeue, and permission behavior.
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

	for _, repoType := range []types.RepositoryType{types.MCPServerRepo, types.SkillRepo} {
		t.Run(fmt.Sprintf("requeues existing %s mirror without metadata refresh", repoType), func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
			mc.mirrorTaskJobStore = taskJobStore
			useFakeMirrorJobClient(mc)

			repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: repoType, Source: types.OpenCSGSource}
			mirror := &database.Mirror{
				ID: 1, SourceUrl: fmt.Sprintf("https://sync.opencsg.com/%ss/ns/n.git", repoType), RepositoryID: repo.ID, Repository: repo,
			}
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, repoType, "CSG_ns", "n").Return(repo, nil)
			mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "writer", repo).Return(&types.UserRepoPermission{CanWrite: true}, nil)
			mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
			taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
				return input.MirrorID == mirror.ID &&
					input.RepositoryID == repo.ID &&
					input.Priority == types.LowMirrorPriority &&
					!input.Urgent &&
					input.JobClient != nil &&
					input.JobCancelClient != nil &&
					input.Metadata == nil
			})).Return(database.MirrorTask{ID: 99, Status: types.MirrorQueued}, nil)

			result, err := mc.MirrorFromSaas(ctx, types.MirrorFromSaasReq{
				Namespace: "CSG_ns", Name: "n", RepoType: repoType, CurrentUser: "writer",
			})

			require.NoError(t, err)
			require.Equal(t, &types.MirrorFromSaasResponse{
				RepositoryID: repo.ID, MirrorID: mirror.ID, TaskID: 99, Status: types.MirrorQueued,
			}, result)
		})
	}

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
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)
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

// TestMirrorComponent_CreateMirrorRequeuesSameSource verifies repeated creation starts a fresh sync for the existing source.
func TestMirrorComponent_CreateMirrorRequeuesSameSource(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	repo := &database.Repository{ID: 123, Path: "ns/n", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{
		ID: 456, RepositoryID: repo.ID, SourceUrl: "https://github.com/upstream/repo.git",
	}
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
	username, accessToken := "source-user", "source-token"
	expectMirrorRepoRequeue(ctx, t, mc, repo, mirror, &username, &accessToken, types.LowMirrorPriority, true)

	got, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
		SourceUrl:   "https://source-user:source-token@github.com/upstream/repo",
		CurrentUser: "user",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		Urgent:      true,
	})

	require.NoError(t, err)
	require.Equal(t, mirror.ID, got.ID)
	require.Equal(t, username, got.Username)
	require.Equal(t, accessToken, got.AccessToken)
	require.Empty(t, fakeStore.inputs)
}

// TestMirrorComponent_CreateMirrorSkipsSourcePathForCodeAndSkill verifies that CreateMirror
// does not set source path fields for code and skill repos, since these are user-imported types.
func TestMirrorComponent_CreateMirrorSkipsSourcePathForCodeAndSkill(t *testing.T) {
	for _, repoType := range []types.RepositoryType{types.CodeRepo, types.SkillRepo} {
		t.Run(string(repoType), func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			fakeStore := &fakeMirrorRepoStore{}
			mc.mirrorRepoStore = fakeStore

			repo := &database.Repository{
				ID:             123,
				Path:           "ns/n",
				HTTPCloneURL:   "https://opencsg.com/repos/ns/n.git",
				RepositoryType: repoType,
			}
			sourceURL := "https://github.com/upstream/repo"
			if repoType == types.SkillRepo {
				sourceURL = "https://opencsg.com/upstream/repo"
				mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
					require.Equal(t, "https://hub.opencsg.com", endpoint)
					require.Empty(t, accessToken)
					return mc.mocks.multiSyncClient
				}
				mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, types.SyncVersion{
					RepoPath: "upstream/repo", RepoType: types.SkillRepo,
				}).Return(&types.Skill{Description: "fresh skill description", DefaultBranch: "develop"}, nil)
			}

			mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, repoType, "ns", "n").Return(repo, nil)
			mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)

			got, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
				SourceUrl:      sourceURL,
				CurrentUser:    "user",
				Namespace:      "ns",
				Name:           "n",
				RepoType:       repoType,
				SkipSourcePath: true,
			})
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Len(t, fakeStore.inputs, 1)
			input := fakeStore.inputs[0]
			require.Empty(t, input.Repository.GithubPath)
			require.Empty(t, input.Repository.HFPath)
			require.Empty(t, input.Repository.MSPath)
			if repoType == types.SkillRepo {
				require.NotNil(t, input.Metadata)
				require.Equal(t, "fresh skill description", input.Metadata.Repository.Description)
				require.Equal(t, "develop", input.Metadata.Repository.DefaultBranch)
			} else {
				require.Nil(t, input.Metadata)
			}
		})
	}
}

// TestMirrorComponent_CreateMirrorHonorsRequestPriority verifies that CreateMirror
// propagates an explicit priority (e.g. ASAP for code/skill imports) instead of
// always defaulting to the lowest priority.
func TestMirrorComponent_CreateMirrorHonorsRequestPriority(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	repo := &database.Repository{
		ID:             123,
		Path:           "ns/n",
		HTTPCloneURL:   "https://opencsg.com/codes/ns/n.git",
		RepositoryType: types.CodeRepo,
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.CodeRepo, "ns", "n").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)

	_, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
		SourceUrl:      "https://github.com/upstream/repo",
		CurrentUser:    "user",
		Namespace:      "ns",
		Name:           "n",
		RepoType:       types.CodeRepo,
		Priority:       types.ASAPMirrorPriority,
		SkipSourcePath: true,
	})
	require.NoError(t, err)
	require.Len(t, fakeStore.inputs, 1)
	require.Equal(t, types.ASAPMirrorPriority, fakeStore.inputs[0].Mirror.Priority)
}

// TestMirrorComponent_CreateMirrorRejectsDifferentSource verifies creation cannot replace an existing mirror source.
func TestMirrorComponent_CreateMirrorRejectsDifferentSource(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	repo := &database.Repository{ID: 123, Path: "ns/n", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{
		ID: 456, RepositoryID: repo.ID, SourceUrl: "https://github.com/existing/repo.git",
	}
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

	got, err := mc.CreateMirror(ctx, types.CreateMirrorReq{
		SourceUrl:   "https://github.com/new/repo",
		CurrentUser: "user",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
	})

	require.ErrorIs(t, err, errorx.ErrMirrorSourceConflict)
	require.Equal(t, repo.ID, got.RepositoryID)
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

// TestMirrorComponent_CreateMirrorRepoPreservesForkTargetCase verifies local mirror target identifiers are trimmed without changing case.
func TestMirrorComponent_CreateMirrorRepoPreservesForkTargetCase(t *testing.T) {
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
		ForkNamespace:     "  ALICE-TEAM ",
		ForkName:          " Qwen-Model  ",
	}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "ALICE-TEAM", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "ALICE-TEAM", "Qwen-Model").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ALICE-TEAM").Return(database.Namespace{
		Path: "Alice-Team",
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
	require.Equal(t, "Alice-Team/Qwen-Model", fakeStore.inputs[0].Repository.Path)
	require.Equal(t, "Qwen-Model", fakeStore.inputs[0].Repository.Name)
	require.Equal(t, "models_Alice-Team/Qwen-Model", fakeStore.inputs[0].Repository.GitPath)
	require.Equal(t, "github_model_Alice-Team_Qwen-Model", fakeStore.inputs[0].Mirror.LocalRepoPath)
}

// TestMirrorComponent_CreateMirrorRepoRejectsCaseVariantExistingTarget verifies repository identity remains case-insensitive.
func TestMirrorComponent_CreateMirrorRepoRejectsCaseVariantExistingTarget(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	createTargetRepo := true
	repo := &database.Repository{ID: 11, Path: "alice/MyName", Name: "MyName", RepositoryType: types.ModelRepo}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "alice", "myname").Return(repo, nil)

	got, err := mc.CreateMirrorRepo(ctx, types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "myname",
		CreateTargetRepo:  &createTargetRepo,
	})

	require.ErrorIs(t, err, errorx.ErrRepoAlreadyExist)
	require.Nil(t, got)
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
		ForkNamespace:     "ALICE",
		ForkName:          "FORKED",
		CreateTargetRepo:  &createTargetRepo,
	}
	repo := &database.Repository{ID: 11, Path: "alice/Forked", Name: "Forked", RepositoryType: types.DatasetRepo}

	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "ALICE", "FORKED").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "ALICE", membership.RoleWrite).Return(true, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.RepositoryID)
	require.Len(t, fakeStore.inputs, 1)
	require.Equal(t, repo, fakeStore.inputs[0].Repository)
	require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
	require.Empty(t, fakeStore.inputs[0].Mirror.Username)
	require.Equal(t, "github_dataset_alice_Forked", fakeStore.inputs[0].Mirror.LocalRepoPath)
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
			if repoType == types.SkillRepo {
				req.MirrorSourceID = 43
				version := types.SyncVersion{RepoPath: "upstream/repo", RepoType: types.SkillRepo}
				mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(43)).Return(&database.MirrorSource{
					ID:         43,
					InfoAPIUrl: "https://api.community.example.com/",
				}, nil)
				mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
					require.Equal(t, "https://api.community.example.com", endpoint)
					return mc.mocks.multiSyncClient
				}
				mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(&types.Skill{}, nil)
			} else {
				mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
					t.Fatal("non-MCP/skill requests and legacy MCP attributes must not fetch metadata")
					return nil
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

// TestMirrorComponent_CreateMirrorRepoFetchesMCPMetadata verifies OpenCSG MCP mirrors import metadata before transactional creation.
func TestMirrorComponent_CreateMirrorRepoFetchesMCPMetadata(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	createTargetRepo := true
	private := false
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "CSG_AIWizards",
		SourceName:        "zaturn",
		MirrorSourceID:    42,
		RepoType:          types.MCPServerRepo,
		Private:           &private,
		CreateTargetRepo:  &createTargetRepo,
		SourceGitCloneUrl: "https://git.example.opencsg.com/CSG_AIWizards/zaturn.git",
		AccessToken:       "source-token",
		Username:          "source-user",
		Description:       "web description",
		CurrentUser:       "admin",
		ForkNamespace:     "local",
		ForkName:          "zaturn-mirror",
	}
	version := types.SyncVersion{RepoPath: "CSG_AIWizards/zaturn", RepoType: types.MCPServerRepo}
	mcpMetadata := &types.MCPServer{
		Nickname:        "Zaturn MCP",
		Description:     "source description",
		Private:         true,
		DefaultBranch:   "develop",
		License:         "Apache-2.0",
		ToolsNum:        1,
		Configuration:   `{"command":"zaturn"}`,
		Schema:          `{"tools":[{"name":"search","description":"Search things","inputSchema":{"required":["query"],"type":"object"}}]}`,
		StarNum:         12,
		ProgramLanguage: "python",
		RunMode:         "stdio",
		InstallDepsCmds: "pip install -r requirements.txt",
		BuildCmds:       "python -m build",
		LaunchCmds:      "python server.py",
		AvatarURL:       "https://example.com/avatar.png",
		Readme:          "API README must be ignored",
	}

	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(42)).Return(&database.MirrorSource{
		ID:         42,
		InfoAPIUrl: "https://api.example.opencsg.com/",
	}, nil)
	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://api.example.opencsg.com", endpoint)
		require.Equal(t, "source-token", accessToken)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().MCPServerInfo(ctx, version).Return(mcpMetadata, nil)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.MCPServerRepo, "local", "zaturn-mirror").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "local").Return(database.Namespace{Path: "local"}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
		ID: 1, Username: "admin", Email: "admin@example.com", RoleMask: "admin",
	}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	input := fakeStore.inputs[0]
	require.Equal(t, "Zaturn MCP", input.Repository.Nickname)
	require.Equal(t, "web description", input.Repository.Description)
	require.Equal(t, "Apache-2.0", input.Repository.License)
	require.Equal(t, "develop", input.Repository.DefaultBranch)
	require.Empty(t, input.Repository.Readme)
	require.Equal(t, 12, input.Repository.StarCount)
	require.False(t, input.Repository.Private)
	require.NotNil(t, input.MCPServer)
	require.Equal(t, "python", input.MCPServer.ProgramLanguage)
	require.Equal(t, "stdio", input.MCPServer.RunMode)
	require.Equal(t, "python server.py", input.MCPServer.LaunchCmds)
	require.Equal(t, "https://example.com/avatar.png", input.MCPServer.AvatarURL)
	require.Len(t, input.MCPServerProperties, 1)
	require.Equal(t, "search", input.MCPServerProperties[0].Name)
	require.Contains(t, input.MCPServerProperties[0].Schema, `"required":["query"]`)
}

// TestMirrorComponent_CreateMirrorRepoFetchesSkillMetadata verifies third-party skill mirrors use the configured source API endpoint.
func TestMirrorComponent_CreateMirrorRepoFetchesSkillMetadata(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "test/abc/skills",
		SourceName:        "reviewer",
		MirrorSourceID:    44,
		RepoType:          types.SkillRepo,
		SourceGitCloneUrl: "https://github.com/skills/reviewer.git",
		CurrentUser:       "admin",
		ForkNamespace:     "local",
		ForkName:          "reviewer",
	}
	version := types.SyncVersion{RepoPath: "skills/reviewer", RepoType: types.SkillRepo}
	skillMetadata := &types.Skill{
		Nickname:      "Code Reviewer",
		Description:   "Reviews code changes",
		DefaultBranch: "master",
		License:       "MIT",
		Readme:        "API README must be ignored",
	}

	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(44)).Return(&database.MirrorSource{
		ID:         44,
		InfoAPIUrl: "https://community.example.com/",
	}, nil)
	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://community.example.com", endpoint)
		require.Empty(t, accessToken)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(skillMetadata, nil)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.SkillRepo, "local", "reviewer").Return(nil, sql.ErrNoRows)
	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "local").Return(database.Namespace{Path: "local"}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
		ID: 1, Username: "admin", Email: "admin@example.com", RoleMask: "admin",
	}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	input := fakeStore.inputs[0]
	require.Equal(t, types.SkillRepo, input.Repository.RepositoryType)
	require.Equal(t, "Code Reviewer", input.Repository.Nickname)
	require.Equal(t, "Reviews code changes", input.Repository.Description)
	require.Equal(t, "master", input.Repository.DefaultBranch)
	require.Equal(t, "MIT", input.Repository.License)
	require.Empty(t, input.Repository.Readme)
	require.Nil(t, input.MCPServer)
}

// TestMirrorComponent_CreateMirrorRepoRejectsUnsupportedMetadataSources verifies MCP and skill metadata APIs are never inferred from arbitrary Git hosts.
func TestMirrorComponent_CreateMirrorRepoRejectsUnsupportedMetadataSources(t *testing.T) {
	tests := []struct {
		name             string
		repoType         types.RepositoryType
		sourceURL        string
		mirrorSourceID   int64
		mirrorSource     *database.MirrorSource
		mirrorSourceErr  error
		wantErrorMessage string
	}{
		{
			name:             "GitHub skill without mirror source",
			repoType:         types.SkillRepo,
			sourceURL:        "https://github.com/upstream/reviewer.git",
			wantErrorMessage: "mirror_source_id with a configured info_api_url is required",
		},
		{
			name:             "GitLab MCP without mirror source",
			repoType:         types.MCPServerRepo,
			sourceURL:        "https://gitlab.com/upstream/server.git",
			wantErrorMessage: "mirror_source_id with a configured info_api_url is required",
		},
		{
			name:             "missing mirror source",
			repoType:         types.SkillRepo,
			sourceURL:        "https://github.com/upstream/reviewer.git",
			mirrorSourceID:   51,
			mirrorSourceErr:  sql.ErrNoRows,
			wantErrorMessage: "mirror source 51 does not exist",
		},
		{
			name:           "mirror source without info API URL",
			repoType:       types.SkillRepo,
			sourceURL:      "https://github.com/upstream/reviewer.git",
			mirrorSourceID: 52,
			mirrorSource: &database.MirrorSource{
				ID: 52,
			},
			wantErrorMessage: "does not configure an info_api_url",
		},
		{
			name:           "mirror source with invalid info API URL",
			repoType:       types.MCPServerRepo,
			sourceURL:      "https://gitlab.com/upstream/server.git",
			mirrorSourceID: 53,
			mirrorSource: &database.MirrorSource{
				ID:         53,
				InfoAPIUrl: "ftp://community.example.com",
			},
			wantErrorMessage: "invalid info_api_url for mirror source 53",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			fakeStore := &fakeMirrorRepoStore{}
			mc.mirrorRepoStore = fakeStore
			req := types.CreateMirrorRepoReq{
				SourceNamespace:   "upstream",
				SourceName:        "repo",
				MirrorSourceID:    tt.mirrorSourceID,
				RepoType:          tt.repoType,
				SourceGitCloneUrl: tt.sourceURL,
				CurrentUser:       "admin",
				ForkNamespace:     "local",
				ForkName:          "repo",
			}

			mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, tt.repoType, "local", "repo").Return(nil, sql.ErrNoRows)
			if tt.mirrorSourceID != 0 {
				mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, tt.mirrorSourceID).Return(tt.mirrorSource, tt.mirrorSourceErr)
			}
			mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
				t.Fatal("unsupported metadata sources must be rejected before creating a client")
				return nil
			}

			got, err := mc.CreateMirrorRepo(ctx, req)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErrorMessage)
			require.True(t, errors.Is(err, errorx.ErrBadRequest))
			require.Nil(t, got)
			require.Empty(t, fakeStore.inputs)
		})
	}
}

// TestMirrorComponent_CreateMirrorRepoReturnsMetadataError verifies identified OpenCSG metadata failures abort creation before database writes.
func TestMirrorComponent_CreateMirrorRepoReturnsMetadataError(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "skills",
		SourceName:        "broken",
		MirrorSourceID:    54,
		RepoType:          types.SkillRepo,
		SourceGitCloneUrl: "https://opencsg.com/skills/broken.git",
		CurrentUser:       "admin",
		ForkNamespace:     "local",
		ForkName:          "broken",
	}
	version := types.SyncVersion{RepoPath: "skills/broken", RepoType: types.SkillRepo}

	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(54)).Return(&database.MirrorSource{ID: 54}, nil)
	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://hub.opencsg.com", endpoint)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(nil, fmt.Errorf("upstream unavailable"))
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.SkillRepo, "local", "broken").Return(nil, sql.ErrNoRows)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.ErrorContains(t, err, "failed to fetch skill mirror metadata")
	require.Nil(t, got)
	require.Empty(t, fakeStore.inputs)
}

// TestMirrorComponent_CreateMirrorRepoSkipSourcePath verifies that when SkipSourcePath is true,
// the source path fields (HFPath/MSPath/GithubPath) are not set on the repository.
func TestMirrorComponent_CreateMirrorRepoSkipSourcePath(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.CodeRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
		SkipSourcePath:    true,
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
	require.Empty(t, fakeStore.inputs[0].Repository.GithubPath)
	require.Empty(t, fakeStore.inputs[0].Repository.HFPath)
	require.Empty(t, fakeStore.inputs[0].Repository.MSPath)
}

// TestMirrorComponent_CreateMirrorRepoRefreshesMCPMetadataOnRequeue verifies repeated MCP syncs refresh API metadata with the new task.
func TestMirrorComponent_CreateMirrorRepoRefreshesMCPMetadataOnRequeue(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "server",
		RepoType:          types.MCPServerRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://opencsg.com/upstream/server.git",
		ForkNamespace:     "local",
		ForkName:          "server",
	}
	repo := &database.Repository{
		ID: 11, Path: "local/server", Name: "server", Nickname: "Old MCP", Description: "old description",
		License: "old-license", Readme: "old readme", DefaultBranch: "main", StarCount: 1, RepositoryType: types.MCPServerRepo,
	}
	mirror := &database.Mirror{ID: 3, RepositoryID: repo.ID, SourceUrl: req.SourceGitCloneUrl}
	version := types.SyncVersion{RepoPath: "upstream/server", RepoType: types.MCPServerRepo}
	metadata := &types.MCPServer{
		Nickname: "Fresh MCP", Description: "fresh description", License: "MIT", DefaultBranch: "develop",
		Readme: "ignored", StarNum: 9, ToolsNum: 1, Configuration: `{"command":"fresh"}`,
		Schema:          `{"tools":[{"name":"search","description":"Fresh search","inputSchema":{"type":"object"}}]}`,
		ProgramLanguage: "go", RunMode: "stdio", LaunchCmds: "go run .",
	}

	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://hub.opencsg.com", endpoint)
		require.Empty(t, accessToken)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().MCPServerInfo(ctx, version).Return(metadata, nil)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "local", "server").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

	taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
	mc.mirrorTaskJobStore = taskJobStore
	useFakeMirrorJobClient(mc)
	taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
		if input.Metadata == nil || input.Metadata.MCPServer == nil {
			return false
		}
		updatedRepo := input.Metadata.Repository
		return updatedRepo.ID == repo.ID &&
			updatedRepo.Nickname == "Fresh MCP" &&
			updatedRepo.Description == "fresh description" &&
			updatedRepo.License == "MIT" &&
			updatedRepo.Readme == "old readme" &&
			updatedRepo.DefaultBranch == "develop" &&
			updatedRepo.StarCount == 9 &&
			input.Metadata.MCPServer.Configuration == `{"command":"fresh"}` &&
			input.Metadata.MCPServer.ProgramLanguage == "go" &&
			len(input.Metadata.MCPServerProperties) == 1 &&
			input.Metadata.MCPServerProperties[0].Name == "search"
	})).Return(database.MirrorTask{ID: 99}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, mirror.ID, got.ID)
}

// TestMirrorComponent_CreateMirrorRepoRefreshesSkillMetadataOnRequeue verifies repeated skill syncs refresh API metadata with the new task.
func TestMirrorComponent_CreateMirrorRepoRefreshesSkillMetadataOnRequeue(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "reviewer",
		RepoType:          types.SkillRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://opencsg.com/upstream/reviewer.git",
		Description:       "stale request description",
		License:           "stale-request-license",
		DefaultBranch:     "stale-request-branch",
		ForkNamespace:     "local",
		ForkName:          "reviewer",
	}
	repo := &database.Repository{
		ID: 12, Path: "local/reviewer", Name: "reviewer", Nickname: "Old Skill", Description: "old description",
		License: "old-license", Readme: "old readme", DefaultBranch: "main", RepositoryType: types.SkillRepo,
	}
	mirror := &database.Mirror{ID: 4, RepositoryID: repo.ID, SourceUrl: req.SourceGitCloneUrl}
	version := types.SyncVersion{RepoPath: "upstream/reviewer", RepoType: types.SkillRepo}
	updatedAt := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	metadata := &types.Skill{
		Nickname: "Fresh Skill", Description: "fresh description", License: "Apache-2.0",
		DefaultBranch: "develop", UpdatedAt: updatedAt,
	}

	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://hub.opencsg.com", endpoint)
		require.Empty(t, accessToken)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(metadata, nil)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "local", "reviewer").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

	taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
	mc.mirrorTaskJobStore = taskJobStore
	useFakeMirrorJobClient(mc)
	taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
		if input.Metadata == nil || input.Metadata.SkillLastUpdatedAt == nil {
			return false
		}
		updatedRepo := input.Metadata.Repository
		return updatedRepo.ID == repo.ID &&
			updatedRepo.Nickname == "Fresh Skill" &&
			updatedRepo.Description == "fresh description" &&
			updatedRepo.License == "Apache-2.0" &&
			updatedRepo.Readme == "old readme" &&
			updatedRepo.DefaultBranch == "develop" &&
			input.Metadata.SkillLastUpdatedAt.Equal(updatedAt)
	})).Return(database.MirrorTask{ID: 100}, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, mirror.ID, got.ID)
}

// TestMirrorComponent_CreateMirrorRepoRefreshesSkillMetadataWhenBindingExistingTarget verifies first task creation also refreshes an existing target.
func TestMirrorComponent_CreateMirrorRepoRefreshesSkillMetadataWhenBindingExistingTarget(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore
	createTargetRepo := false
	req := types.CreateMirrorRepoReq{
		SourceNamespace: "upstream", SourceName: "writer", RepoType: types.SkillRepo,
		CurrentUser: "admin", SourceGitCloneUrl: "https://opencsg.com/upstream/writer.git",
		ForkNamespace: "local", ForkName: "writer", CreateTargetRepo: &createTargetRepo,
	}
	repo := &database.Repository{
		ID: 13, Path: "local/writer", Name: "writer", Nickname: "Old Writer", Description: "old description",
		License: "old-license", Readme: "old readme", DefaultBranch: "main", RepositoryType: types.SkillRepo,
	}
	version := types.SyncVersion{RepoPath: "upstream/writer", RepoType: types.SkillRepo}
	updatedAt := time.Date(2026, time.August, 18, 9, 0, 0, 0, time.UTC)

	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://hub.opencsg.com", endpoint)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(&types.Skill{
		Nickname: "Fresh Writer", Description: "fresh description", License: "MIT", DefaultBranch: "develop", UpdatedAt: updatedAt,
	}, nil)
	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "local", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "local", "writer").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, sql.ErrNoRows)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, fakeStore.inputs, 1)
	input := fakeStore.inputs[0]
	require.False(t, input.CreateRepository)
	require.NotNil(t, input.Metadata)
	require.Equal(t, "Fresh Writer", input.Metadata.Repository.Nickname)
	require.Equal(t, "fresh description", input.Metadata.Repository.Description)
	require.Equal(t, "MIT", input.Metadata.Repository.License)
	require.Equal(t, "old readme", input.Metadata.Repository.Readme)
	require.Equal(t, "develop", input.Metadata.Repository.DefaultBranch)
	require.NotNil(t, input.Metadata.SkillLastUpdatedAt)
	require.True(t, input.Metadata.SkillLastUpdatedAt.Equal(updatedAt))
}

// TestMirrorComponent_SyncMirrorRefreshesSkillMetadata verifies manual sync refreshes skill API metadata before task creation.
func TestMirrorComponent_SyncMirrorRefreshesSkillMetadata(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	repo := &database.Repository{
		ID: 21, Path: "local/reviewer", Name: "reviewer", Nickname: "Old Skill", Description: "old description",
		License: "old-license", Readme: "old readme", DefaultBranch: "main", RepositoryType: types.SkillRepo,
	}
	mirror := &database.Mirror{
		ID: 31, RepositoryID: repo.ID, Repository: repo, SourceUrl: "https://opencsg.com/upstream/reviewer.git",
		SourceRepoPath: "upstream/reviewer", Priority: types.HighMirrorPriority,
	}
	version := types.SyncVersion{RepoPath: "upstream/reviewer", RepoType: types.SkillRepo}
	updatedAt := time.Date(2026, time.August, 18, 10, 0, 0, 0, time.UTC)

	mc.mirrorMetadataClientFactory = func(endpoint, accessToken string) multisync.Client {
		require.Equal(t, "https://hub.opencsg.com", endpoint)
		return mc.mocks.multiSyncClient
	}
	mc.mocks.multiSyncClient.EXPECT().SkillInfo(ctx, version).Return(&types.Skill{
		Nickname: "Fresh Skill", Description: "fresh description", License: "MIT", DefaultBranch: "develop", UpdatedAt: updatedAt,
	}, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{RoleMask: "admin"}, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.SkillRepo, "local", "reviewer").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)

	taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
	mc.mirrorTaskJobStore = taskJobStore
	useFakeMirrorJobClient(mc)
	taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
		return input.Metadata != nil &&
			input.Metadata.Repository.Nickname == "Fresh Skill" &&
			input.Metadata.Repository.Readme == "old readme" &&
			input.Metadata.SkillLastUpdatedAt != nil &&
			input.Metadata.SkillLastUpdatedAt.Equal(updatedAt)
	})).Return(database.MirrorTask{ID: 101}, nil)

	err := mc.SyncMirror(ctx, types.SyncMirrorReq{
		RepoType: types.SkillRepo, Namespace: "local", Name: "reviewer", CurrentUser: "admin",
	})
	require.NoError(t, err)
}
