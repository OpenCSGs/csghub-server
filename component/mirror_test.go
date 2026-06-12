package component

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorComponent_CreateMirrorRepo(t *testing.T) {
	cases := []struct {
		repoType types.RepositoryType
	}{
		{types.ModelRepo},
		{types.DatasetRepo},
		{types.CodeRepo},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)

			req := types.CreateMirrorRepoReq{
				SourceNamespace:   "sns",
				SourceName:        "sn",
				RepoType:          c.repoType,
				CurrentUser:       "user",
				SourceGitCloneUrl: "https://github.com/foo/bar.git",
				MirrorSourceID:    1,
			}

			repo := &database.Repository{ID: 10}
			mc.mocks.stores.RepoMock().EXPECT().FindByMirrorSourceURL(
				ctx, "https://github.com/foo/bar.git",
			).Return(
				nil, nil,
			)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, req.RepoType, "AIWizards", "sn",
			).Return(
				repo, nil,
			)
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, req.RepoType, "AIWizards", "sns_sn",
			).Return(
				nil, sql.ErrNoRows,
			)
			mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().FindBySourceNamespace(context.Background(), "sns").Return(&database.MirrorNamespaceMapping{
				TargetNamespace: "AIWizards",
			}, nil)
			mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(
				ctx, "AIWizards",
			).Return(database.Namespace{
				User: database.User{Username: "user"},
			}, nil)

			repo1 := &database.Repository{ID: 11}
			mc.mocks.components.repo.EXPECT().CreateRepo(ctx, mock.AnythingOfType("types.CreateRepoReq")).Return(&gitserver.CreateRepoResp{}, repo1, &gitserver.CommitFilesReq{}, nil)
			switch req.RepoType {
			case types.ModelRepo:
				mc.mocks.stores.ModelMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Model{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}, "AIWizards/sns_sn").Return(nil, nil)
			case types.DatasetRepo:
				mc.mocks.stores.DatasetMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Dataset{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}, "AIWizards/sns_sn").Return(nil, nil)
			case types.CodeRepo:
				mc.mocks.stores.CodeMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Code{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}, "AIWizards/sns_sn").Return(nil, nil)
			}

			mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(1)).Return(
				&database.MirrorSource{}, nil,
			)
			mc.mocks.stores.RepoMock().EXPECT().UpdateSourcePath(ctx, repo1.ID, "foo/bar", "github").Return(nil)
			reqMirror := &database.Mirror{
				ID:        1,
				Priority:  types.ASAPMirrorPriority,
				SourceUrl: "https://github.com/foo/bar.git",
			}
			localRepoPath := ""
			switch req.RepoType {
			case types.ModelRepo:
				localRepoPath = "_model_sns_sn"
			case types.DatasetRepo:
				localRepoPath = "_dataset_sns_sn"
			case types.CodeRepo:
				localRepoPath = "_code_sns_sn"
			}

			cm := &database.Mirror{
				Username:       "sns",
				SourceRepoPath: "sns/sn",
				LocalRepoPath:  localRepoPath,
				Priority:       types.ASAPMirrorPriority,
				SourceUrl:      "https://github.com/foo/bar.git",
				Repository:     &database.Repository{ID: 11},
				RepositoryID:   11,
				MirrorSourceID: 1,
			}

			mc.mocks.stores.MirrorMock().EXPECT().Create(ctx, cm).Return(
				reqMirror, nil,
			)
			mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, mock.Anything).Return(
				nil,
			)
			mc.mocks.stores.MirrorTaskMock().EXPECT().Create(ctx, mock.Anything).Return(database.MirrorTask{ID: 123}, nil)

			m, err := mc.CreateMirrorRepo(ctx, req)
			require.Nil(t, err)
			require.Equal(t, reqMirror, m)

		})
	}

}

func TestMirrorComponent_CreateMirrorForkRepo(t *testing.T) {
	cases := []struct {
		name              string
		repoType          types.RepositoryType
		localRepoPath     string
		expectRepoPathRow func(ctx context.Context, mc *testMirrorWithMocks, repo *database.Repository)
	}{
		{
			name:          "model",
			repoType:      types.ModelRepo,
			localRepoPath: "github_model_usera_repo",
			expectRepoPathRow: func(ctx context.Context, mc *testMirrorWithMocks, repo *database.Repository) {
				mc.mocks.stores.ModelMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Model{
					Repository:   repo,
					RepositoryID: repo.ID,
				}, "usera/repo").Return(nil, nil)
			},
		},
		{
			name:          "dataset",
			repoType:      types.DatasetRepo,
			localRepoPath: "github_dataset_usera_repo",
			expectRepoPathRow: func(ctx context.Context, mc *testMirrorWithMocks, repo *database.Repository) {
				mc.mocks.stores.DatasetMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Dataset{
					Repository:   repo,
					RepositoryID: repo.ID,
				}, "usera/repo").Return(nil, nil)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)

			req := types.CreateMirrorRepoReq{
				SourceNamespace:   "test",
				SourceName:        "repo",
				RepoType:          tc.repoType,
				CurrentUser:       "usera",
				SourceGitCloneUrl: "https://github.com/test/repo.git",
				MirrorSourceID:    1,
				ForkNamespace:     "usera",
				ForkName:          "repo",
				DefaultBranch:     "main",
				Description:       "desc",
				License:           "apache-2.0",
			}

			repo := &database.Repository{ID: 11}
			mc.mocks.stores.RepoMock().EXPECT().FindByPath(
				ctx, req.RepoType, "usera", "repo",
			).Return(nil, sql.ErrNoRows)
			mc.mocks.components.repo.EXPECT().CreateRepo(ctx, mock.MatchedBy(func(createReq types.CreateRepoReq) bool {
				return createReq.Username == "usera" &&
					createReq.Namespace == "usera" &&
					createReq.Name == "repo" &&
					createReq.Nickname == "repo" &&
					createReq.Description == "desc" &&
					createReq.Private &&
					createReq.License == "apache-2.0" &&
					createReq.DefaultBranch == "main" &&
					createReq.RepoType == tc.repoType
			})).Return(&gitserver.CreateRepoResp{}, repo, nil, nil)
			tc.expectRepoPathRow(ctx, mc, repo)
			mc.mocks.stores.RepoMock().EXPECT().UpdateSourcePath(ctx, repo.ID, "test/repo", "github").Return(nil)

			reqMirror := &database.Mirror{
				ID:        1,
				Priority:  types.ASAPMirrorPriority,
				SourceUrl: req.SourceGitCloneUrl,
			}
			mc.mocks.stores.MirrorMock().EXPECT().Create(ctx, mock.MatchedBy(func(mirror *database.Mirror) bool {
				return mirror.Username == "test" &&
					mirror.SourceRepoPath == "test/repo" &&
					mirror.LocalRepoPath == tc.localRepoPath &&
					mirror.Priority == types.ASAPMirrorPriority &&
					mirror.SourceUrl == req.SourceGitCloneUrl &&
					mirror.Repository == repo &&
					mirror.RepositoryID == repo.ID &&
					mirror.MirrorSourceID == 1
			})).Return(reqMirror, nil)
			mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, mock.MatchedBy(func(mirror *database.Mirror) bool {
				return mirror.ID == reqMirror.ID && mirror.Status == types.MirrorQueued
			})).Return(nil)
			mc.mocks.stores.MirrorTaskMock().EXPECT().Create(ctx, mock.MatchedBy(func(task database.MirrorTask) bool {
				return task.MirrorID == reqMirror.ID &&
					task.Status == types.MirrorQueued &&
					task.Priority == types.ASAPMirrorPriority
			})).Return(database.MirrorTask{ID: 123}, nil)

			mirror, err := mc.CreateMirrorForkRepo(ctx, req)
			require.Nil(t, err)
			require.Equal(t, reqMirror, mirror)
		})
	}
}

func TestMirrorComponent_CreateMirrorForkRepoRequeuesExistingMirror(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "test",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "usera",
		SourceGitCloneUrl: "https://github.com/test/repo.git",
		ForkNamespace:     "usera",
		ForkName:          "repo",
	}

	repo := &database.Repository{ID: 11}
	existingMirror := &database.Mirror{
		ID:        1,
		SourceUrl: req.SourceGitCloneUrl,
	}
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, req.RepoType, "usera", "repo").Return(repo, nil)
	mc.mocks.stores.MirrorMock().EXPECT().FindByRepoID(ctx, repo.ID).Return(existingMirror, nil)
	mc.mocks.components.repo.EXPECT().SyncMirror(ctx, req.RepoType, "usera", "repo", "usera").Return(nil)

	mirror, err := mc.CreateMirrorForkRepo(ctx, req)
	require.Nil(t, err)
	require.Equal(t, existingMirror, mirror)
}

func TestMirrorComponent_CreateMirrorForkRepoRejectsUnsupportedRepoTypeBeforeCreate(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "test",
		SourceName:        "repo",
		RepoType:          types.CodeRepo,
		CurrentUser:       "usera",
		SourceGitCloneUrl: "https://github.com/test/repo.git",
		ForkNamespace:     "usera",
		ForkName:          "repo",
	}

	mirror, err := mc.CreateMirrorForkRepo(ctx, req)
	require.Error(t, err)
	require.Nil(t, mirror)
}

// TestMirrorComponent_CreateMirrorForkRepoAllowsUnknownSourceURL verifies unsupported source hosts can create mirror forks.
func TestMirrorComponent_CreateMirrorForkRepoAllowsUnknownSourceURL(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	req := types.CreateMirrorRepoReq{
		SourceNamespace:   "test",
		SourceName:        "repo",
		RepoType:          types.ModelRepo,
		CurrentUser:       "usera",
		SourceGitCloneUrl: "https://gitlab.com/test/repo.git",
		MirrorSourceID:    1,
		ForkNamespace:     "usera",
		ForkName:          "repo",
		DefaultBranch:     "main",
		Description:       "desc",
		License:           "apache-2.0",
	}

	repo := &database.Repository{ID: 11}
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(
		ctx, req.RepoType, "usera", "repo",
	).Return(nil, sql.ErrNoRows)
	mc.mocks.components.repo.EXPECT().CreateRepo(ctx, mock.MatchedBy(func(createReq types.CreateRepoReq) bool {
		return createReq.Username == "usera" &&
			createReq.Namespace == "usera" &&
			createReq.Name == "repo" &&
			createReq.Nickname == "repo" &&
			createReq.Description == "desc" &&
			createReq.Private &&
			createReq.License == "apache-2.0" &&
			createReq.DefaultBranch == "main" &&
			createReq.RepoType == types.ModelRepo
	})).Return(&gitserver.CreateRepoResp{}, repo, nil, nil)
	mc.mocks.stores.ModelMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Model{
		Repository:   repo,
		RepositoryID: repo.ID,
	}, "usera/repo").Return(nil, nil)

	reqMirror := &database.Mirror{
		ID:        1,
		Priority:  types.ASAPMirrorPriority,
		SourceUrl: req.SourceGitCloneUrl,
	}
	mc.mocks.stores.MirrorMock().EXPECT().Create(ctx, mock.MatchedBy(func(mirror *database.Mirror) bool {
		return mirror.Username == "test" &&
			mirror.SourceRepoPath == "test/repo" &&
			mirror.LocalRepoPath == "other_model_usera_repo" &&
			mirror.Priority == types.ASAPMirrorPriority &&
			mirror.SourceUrl == req.SourceGitCloneUrl &&
			mirror.Repository == repo &&
			mirror.RepositoryID == repo.ID &&
			mirror.MirrorSourceID == 1
	})).Return(reqMirror, nil)
	mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, mock.MatchedBy(func(mirror *database.Mirror) bool {
		return mirror.ID == reqMirror.ID && mirror.Status == types.MirrorQueued
	})).Return(nil)
	mc.mocks.stores.MirrorTaskMock().EXPECT().Create(ctx, mock.MatchedBy(func(task database.MirrorTask) bool {
		return task.MirrorID == reqMirror.ID &&
			task.Status == types.MirrorQueued &&
			task.Priority == types.ASAPMirrorPriority
	})).Return(database.MirrorTask{ID: 123}, nil)

	mirror, err := mc.CreateMirrorForkRepo(ctx, req)
	require.Nil(t, err)
	require.Equal(t, reqMirror, mirror)
}

func TestMirrorComponent_Repos(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, "", true).Return([]database.Mirror{
		{
			CurrentTask: &database.MirrorTask{Progress: 100, Status: types.MirrorLfsSyncFinished},
			Progress:    100,
			Repository:  &database.Repository{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepositoryType: types.ModelRepo},
		},
	}, 100, nil)

	data, total, err := mc.Repos(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MirrorRepo{
		{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepoType: types.ModelRepo, Progress: 100},
	}, data)
}

func TestMirrorComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, "foo", false).Return(
		[]database.Mirror{{CurrentTask: &database.MirrorTask{Status: types.MirrorLfsSyncFinished}, Username: "user", LastMessage: "msg", Repository: &database.Repository{}}}, 100, nil,
	)

	data, total, err := mc.Index(ctx, 10, 1, "foo")
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Mirror{
		{Username: "user", LastMessage: "msg", LocalRepoPath: "s/", Status: types.MirrorLfsSyncFinished},
	}, data)
}

func TestMirrorComponent_Statistic(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().StatusCount(ctx).Return([]database.MirrorStatusCount{
		{Status: types.MirrorRepoSyncFinished, Count: 100},
	}, nil)

	s, err := mc.Statistics(ctx)
	require.Nil(t, err)
	require.Equal(t, []types.MirrorStatusCount{
		{Status: types.MirrorRepoSyncFinished, Count: 100},
	}, s)

}
