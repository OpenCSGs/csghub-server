package component

import (
	"context"
	"database/sql"
	"testing"

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
}

// CreateMirrorRepoRecords stores the input and returns the mirror as if the transaction committed.
func (s *fakeMirrorRepoStore) CreateMirrorRepoRecords(ctx context.Context, input database.CreateMirrorRepoRecordsInput) (*database.Mirror, error) {
	s.inputs = append(s.inputs, input)
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
	mc.mirrorRepoJobClient = workhub.NewMirrorRepoJobClient(jobClient)
	return jobClient
}

// expectMirrorRepoRequeue injects a mocked transactional requeue store for duplicate mirror sync tests.
func expectMirrorRepoRequeue(ctx context.Context, t *testing.T, mc *testMirrorWithMocks, repo *database.Repository, mirror *database.Mirror) {
	taskJobStore := mockdb.NewMockMirrorTaskJobStore(t)
	mc.mirrorTaskJobStore = taskJobStore
	useFakeMirrorJobClient(mc)
	taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
		return input.MirrorID == mirror.ID &&
			input.RepositoryID == repo.ID &&
			input.Priority == types.ASAPMirrorPriority &&
			input.JobClient != nil &&
			input.JobCancelClient != nil
	})).Return(database.MirrorTask{ID: 99}, nil)
}

func TestMirrorComponent_MirrorFromSaas(t *testing.T) {
	t.Run("creates mirror records and repo job for existing repo without mirror", func(t *testing.T) {
		ctx := context.TODO()
		mc := initializeTestMirrorComponent(ctx, t)
		mc.config.MultiSync.SaasSyncDomain = "https://saas.test"
		fakeStore := &fakeMirrorRepoStore{}
		mc.mirrorRepoStore = fakeStore

		repo := &database.Repository{ID: 123, Path: "CSG_ns/n", RepositoryType: types.ModelRepo, Source: types.OpenCSGSource}
		mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "CSG_ns", "n").Return(repo, nil)
		mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(nil, sql.ErrNoRows)
		mc.mocks.stores.SyncVersionMock().EXPECT().FindByRepoTypeAndPath(ctx, "ns/n", types.ModelRepo).Return(&database.SyncVersion{SourceID: types.SyncVersionSourceOpenCSG}, nil)

		err := mc.MirrorFromSaas(ctx, "CSG_ns", "n", types.ModelRepo)
		require.NoError(t, err)
		require.Len(t, fakeStore.inputs, 1)
		input := fakeStore.inputs[0]
		require.False(t, input.CreateRepository)
		require.Equal(t, repo, input.Repository)
		require.Equal(t, types.ModelRepo, input.Repository.RepositoryType)
		require.Equal(t, "https://saas.test/models/ns/n.git", input.Mirror.SourceUrl)
		require.Equal(t, int64(123), input.Mirror.RepositoryID)
		require.Equal(t, repo, input.Mirror.Repository)
		require.Equal(t, "CSG_ns/n", input.Mirror.SourceRepoPath)
		require.Equal(t, types.ASAPMirrorPriority, input.Mirror.Priority)
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
		mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, int64(123)).Return(mirror, nil)
		taskJobStore.EXPECT().RequeueMirrorRepoTask(ctx, mock.MatchedBy(func(input database.RequeueMirrorRepoTaskInput) bool {
			return input.MirrorID == mirror.ID &&
				input.RepositoryID == repo.ID &&
				input.Priority == types.ASAPMirrorPriority &&
				input.JobClient != nil &&
				input.JobCancelClient != nil
		})).Return(database.MirrorTask{}, nil)

		err := mc.MirrorFromSaas(ctx, "CSG_ns", "n", types.ModelRepo)
		require.NoError(t, err)
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

// TestMirrorComponent_UpdateMirror verifies mirror configuration writes are owned by MirrorComponent.
func TestMirrorComponent_UpdateMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "ns", membership.RoleAdmin).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(&database.Repository{
		ID: 123,
	}, nil)
	m := database.Mirror{
		ID:              123,
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
		Username:       "user",
		AccessToken:    "ak",
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
		SourceUrl:      "https://github.com/upstream/repo.git",
		Username:       "source-user",
		CurrentUser:    "user",
		AccessToken:    "source-token",
		Namespace:      "ns",
		Name:           "n",
		RepoType:       types.ModelRepo,
		MirrorSourceID: 321,
		SourceRepoPath: "upstream/repo",
		Interval:       "24h",
	})
	require.NoError(t, err)
	require.Equal(t, repo.ID, got.RepositoryID)
	require.Len(t, fakeStore.inputs, 1)
	require.False(t, fakeStore.inputs[0].CreateRepository)
	require.Equal(t, repo, fakeStore.inputs[0].Repository)
	require.Equal(t, types.ModelRepo, fakeStore.inputs[0].Repository.RepositoryType)
	require.Equal(t, "upstream/repo", fakeStore.inputs[0].Repository.GithubPath)
	require.Equal(t, database.Mirror{
		Interval:       "24h",
		SourceUrl:      "https://github.com/upstream/repo.git",
		MirrorSourceID: 321,
		Username:       "source-user",
		AccessToken:    "source-token",
		PushUrl:        "https://opencsg.com/models/ns/n.git",
		SourceRepoPath: "upstream/repo",
		LocalRepoPath:  "github_model_ns_n",
		RepositoryID:   repo.ID,
		Repository:     repo,
		Priority:       types.ASAPMirrorPriority,
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

// TestMirrorComponent_CreateMirrorRepoAddsGitSuffixToSourceURL verifies mirror source URLs are normalized before records are created.
func TestMirrorComponent_CreateMirrorRepoAddsGitSuffixToSourceURL(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo/",
		ForkNamespace:     "alice",
		ForkName:          "forked",
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
			require.Nil(t, got)
		})
	}
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
		SourceGitCloneUrl: "https://github.com/upstream/repo",
		ForkNamespace:     "alice",
		ForkName:          "forked",
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.ModelRepo}
	mirror := &database.Mirror{ID: 3, RepositoryID: repo.ID, SourceUrl: "https://github.com/upstream/repo.git"}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(mirror, nil)
	expectMirrorRepoRequeue(ctx, t, mc, repo, mirror)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.NoError(t, err)
	require.Equal(t, mirror.ID, got.ID)
}

// TestMirrorComponent_CreateMirrorRepoAddsSourceToExistingTargetWithoutMirror verifies an existing repo without a mirror can be bound to one source URL.
func TestMirrorComponent_CreateMirrorRepoAddsSourceToExistingTargetWithoutMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	fakeStore := &fakeMirrorRepoStore{}
	mc.mirrorRepoStore = fakeStore

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "repo",
		RepoType:          types.DatasetRepo,
		CurrentUser:       "admin",
		SourceGitCloneUrl: "https://github.com/upstream/repo.git",
		ForkNamespace:     "alice",
		ForkName:          "forked",
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

	req := types.CreateMirrorRepoReq{
		SourceNamespace:    "upstream",
		SourceName:         "repo",
		RepoType:           types.CodeRepo,
		CurrentUser:        "admin",
		SourceGitCloneUrl:  "https://github.com/upstream/repo.git",
		ForkNamespace:      "alice",
		ForkName:           "forked",
		RejectExistingRepo: true,
	}
	repo := &database.Repository{ID: 11, Path: "alice/forked", RepositoryType: types.CodeRepo}

	mc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "alice", membership.RoleWrite).Return(true, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "alice", "forked").Return(repo, nil)

	got, err := mc.CreateMirrorRepo(ctx, req)
	require.Error(t, err)
	require.ErrorIs(t, err, errorx.ErrRepoAlreadyExist)
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

			req := types.CreateMirrorRepoReq{
				SourceNamespace:   "upstream",
				SourceName:        "repo",
				RepoType:          repoType,
				CurrentUser:       "admin",
				SourceGitCloneUrl: "https://github.com/upstream/repo.git",
				ForkName:          "forked",
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
