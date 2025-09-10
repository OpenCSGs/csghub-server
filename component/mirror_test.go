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
			mc.mocks.components.repo.EXPECT().CreateRepo(ctx, types.CreateRepoReq{
				Username:      "user",
				Namespace:     "AIWizards",
				Name:          "sns_sn",
				Nickname:      "sns_sn",
				Description:   req.Description,
				Private:       true,
				License:       req.License,
				DefaultBranch: req.DefaultBranch,
				RepoType:      req.RepoType,
			}).Return(&gitserver.CreateRepoResp{}, repo1, nil)
			switch req.RepoType {
			case types.ModelRepo:
				mc.mocks.stores.ModelMock().EXPECT().Create(ctx, database.Model{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}).Return(nil, nil)
			case types.DatasetRepo:
				mc.mocks.stores.DatasetMock().EXPECT().Create(ctx, database.Dataset{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}).Return(nil, nil)
			case types.CodeRepo:
				mc.mocks.stores.CodeMock().EXPECT().Create(ctx, database.Code{
					Repository:   repo1,
					RepositoryID: repo1.ID,
				}).Return(nil, nil)
			}

			mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(0)).Return(
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
