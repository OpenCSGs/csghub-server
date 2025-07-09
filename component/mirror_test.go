package component

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/queue"
)

func TestMirrorComponent_CreatePushMirrorForFinishedMirrorTask(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().NoPushMirror(ctx).Return([]database.Mirror{
		{MirrorTaskID: 1},
		{MirrorTaskID: 2, LocalRepoPath: "foo"},
	}, nil)
	mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(1)).Return(
		&mirrorserver.MirrorTaskInfo{}, nil,
	)
	mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(2)).Return(
		&mirrorserver.MirrorTaskInfo{
			Status: mirrorserver.TaskStatusFinished,
		}, nil,
	)
	mc.mocks.mirrorServer.EXPECT().CreatePushMirror(ctx, mirrorserver.CreatePushMirrorReq{
		Name:     "foo",
		Interval: "8h",
	}).Return(nil)
	mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, &database.Mirror{
		MirrorTaskID: 2, LocalRepoPath: "foo", PushMirrorCreated: true,
	}).Return(nil)

	err := mc.CreatePushMirrorForFinishedMirrorTask(ctx)
	require.Nil(t, err)
}

func TestMirrorComponent_CreateMirrorRepo(t *testing.T) {

	cases := []struct {
		repoType types.RepositoryType
		gitea    bool
	}{
		{types.ModelRepo, false},
		{types.DatasetRepo, false},
		{types.CodeRepo, false},
		{types.CodeRepo, true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)

			req := types.CreateMirrorRepoReq{
				SourceNamespace: "sns",
				SourceName:      "sn",
				RepoType:        c.repoType,
				CurrentUser:     "user",
			}

			if c.gitea {
				mc.config.GitServer.Type = types.GitServerTypeGitea
			} else {
				mc.config.GitServer.Type = types.GitServerTypeGitaly
			}

			repo := &database.Repository{}
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
			mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(
				ctx, "AIWizards",
			).Return(database.Namespace{
				User: database.User{Username: "user"},
			}, nil)
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
			}).Return(&gitserver.CreateRepoResp{}, &database.Repository{}, nil)
			switch req.RepoType {
			case types.ModelRepo:
				mc.mocks.stores.ModelMock().EXPECT().Create(ctx, database.Model{
					Repository:   repo,
					RepositoryID: repo.ID,
				}).Return(nil, nil)
			case types.DatasetRepo:
				mc.mocks.stores.DatasetMock().EXPECT().Create(ctx, database.Dataset{
					Repository:   repo,
					RepositoryID: repo.ID,
				}).Return(nil, nil)
			case types.CodeRepo:
				mc.mocks.stores.CodeMock().EXPECT().Create(ctx, database.Code{
					Repository:   repo,
					RepositoryID: repo.ID,
				}).Return(nil, nil)
			}
			mc.mocks.stores.GitServerAccessTokenMock().EXPECT().FindByType(ctx, "git").Return(
				[]database.GitServerAccessToken{
					{},
				}, nil,
			)
			mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(0)).Return(
				&database.MirrorSource{}, nil,
			)
			if c.gitea {
				mc.mocks.mirrorServer.EXPECT().CreateMirrorRepo(ctx, mirrorserver.CreateMirrorRepoReq{
					Name:      "_code_sns_sn",
					Namespace: "root",
					Private:   false,
					SyncLfs:   req.SyncLfs,
				}).Return(123, nil)
			}
			reqMirror := &database.Mirror{
				ID:       1,
				Priority: types.HighMirrorPriority,
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
				PushUsername:   "root",
				SourceRepoPath: "sns/sn",
				LocalRepoPath:  localRepoPath,
				Priority:       types.HighMirrorPriority,
				Repository:     &database.Repository{},
			}
			if c.gitea {
				cm.MirrorTaskID = 123
			}
			mc.mocks.stores.MirrorMock().EXPECT().Create(ctx, cm).Return(
				reqMirror, nil,
			)
			if !c.gitea {
				mc.mocks.mirrorQueue.EXPECT().PushRepoMirror(&queue.MirrorTask{
					MirrorID: reqMirror.ID,
					Priority: queue.PriorityMap[reqMirror.Priority],
				})
				mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, reqMirror).Return(nil)
			}

			m, err := mc.CreateMirrorRepo(ctx, req)
			require.Nil(t, err)
			require.Equal(t, reqMirror, m)

		})
	}

}

func TestMirrorComponent_CheckMirrorProgress(t *testing.T) {

	for _, saas := range []bool{false, true} {
		t.Run(fmt.Sprintf("saas %v", saas), func(t *testing.T) {
			ctx := context.TODO()
			mc := initializeTestMirrorComponent(ctx, t)
			mc.saas = saas

			mirrors := []database.Mirror{
				{
					ID: 1, MirrorTaskID: 11,
					Repository: &database.Repository{
						ID: 111, Path: "foo/bar", RepositoryType: types.ModelRepo,
					},
				},
				{
					ID: 2, MirrorTaskID: 12,
					Repository: &database.Repository{
						ID: 111, Path: "foo/bar", RepositoryType: types.ModelRepo,
					},
				},
				{
					ID: 3, MirrorTaskID: 13,
					Repository: &database.Repository{
						ID: 111, Path: "foo/bar", RepositoryType: types.ModelRepo,
					},
				},
				{
					ID: 4, MirrorTaskID: 14,
					Repository: &database.Repository{
						ID: 111, Path: "foo/bar", RepositoryType: types.ModelRepo,
					},
				},
			}
			mc.mocks.stores.MirrorMock().EXPECT().Unfinished(ctx).Return(mirrors, nil)

			if saas {
				mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(11)).Return(
					&mirrorserver.MirrorTaskInfo{
						Status: mirrorserver.TaskStatusQueued,
					}, nil,
				)
				mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(12)).Return(
					&mirrorserver.MirrorTaskInfo{
						Status: mirrorserver.TaskStatusRunning,
					}, nil,
				)
				mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(13)).Return(
					&mirrorserver.MirrorTaskInfo{
						Status: mirrorserver.TaskStatusFailed,
					}, nil,
				)
				mc.mocks.mirrorServer.EXPECT().GetMirrorTaskInfo(ctx, int64(14)).Return(
					&mirrorserver.MirrorTaskInfo{
						Status: mirrorserver.TaskStatusFinished,
					}, nil,
				)
			} else {
				mc.mocks.gitServer.EXPECT().GetMirrorTaskInfo(ctx, int64(11)).Return(
					&gitserver.MirrorTaskInfo{
						Status: gitserver.TaskStatusQueued,
					}, nil,
				)
				mc.mocks.gitServer.EXPECT().GetMirrorTaskInfo(ctx, int64(12)).Return(
					&gitserver.MirrorTaskInfo{
						Status: gitserver.TaskStatusRunning,
					}, nil,
				)
				mc.mocks.gitServer.EXPECT().GetMirrorTaskInfo(ctx, int64(13)).Return(
					&gitserver.MirrorTaskInfo{
						Status: gitserver.TaskStatusFailed,
					}, nil,
				)
				mc.mocks.gitServer.EXPECT().GetMirrorTaskInfo(ctx, int64(14)).Return(
					&gitserver.MirrorTaskInfo{
						Status: gitserver.TaskStatusFinished,
					}, nil,
				)
			}
			mirrors[0].Status = types.MirrorInit
			mirrors[1].Status = types.MirrorRepoSyncStart
			mirrors[1].Progress = 100
			mirrors[2].Status = types.MirrorLfsSyncFailed
			mirrors[3].Status = types.MirrorLfsSyncFinished
			mirrors[3].Progress = 100
			mc.mocks.gitServer.EXPECT().GetRepo(ctx, gitserver.GetRepoReq{
				Namespace: "foo",
				Name:      "bar",
				RepoType:  types.ModelRepo,
			}).Return(&gitserver.CreateRepoResp{}, nil)
			for _, m := range mirrors {
				m.Repository.SyncStatus = mirrorStatusAndRepoSyncStatusMapping[m.Status]
				mv := m
				mc.mocks.stores.MirrorMock().EXPECT().Update(ctx, &mv).Return(nil).Once()
				mc.mocks.stores.RepoMock().EXPECT().UpdateRepo(
					ctx, database.Repository{
						ID:             111,
						Path:           "foo/bar",
						RepositoryType: types.ModelRepo,
						SyncStatus:     mirrorStatusAndRepoSyncStatusMapping[mv.Status],
					},
				).Return(nil, nil).Once()
			}
			mc.mocks.gitServer.EXPECT().GetTree(
				mock.Anything, types.GetTreeRequest{Namespace: "foo", Name: "bar", RepoType: "model", Limit: 500, Recursive: true},
			).Return(&types.GetRepoFileTreeResp{Files: []*types.File{{Name: "foo.go"}}, Cursor: ""}, nil)

			err := mc.CheckMirrorProgress(ctx)
			require.Nil(t, err)
		})
	}

}

func TestMirrorComponent_Repos(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RepoMock().EXPECT().WithMirror(ctx, 10, 1).Return([]database.Repository{
		{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepositoryType: types.ModelRepo},
	}, 100, nil)

	data, total, err := mc.Repos(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MirrorRepo{
		{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepoType: types.ModelRepo},
	}, data)
}

func TestMirrorComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, "foo").Return(
		[]database.Mirror{{Username: "user", LastMessage: "msg", Repository: &database.Repository{}}}, 100, nil,
	)

	data, total, err := mc.Index(ctx, 10, 1, "foo")
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Mirror{
		{Username: "user", LastMessage: "msg", LocalRepoPath: "s/"},
	}, data)
}

func TestMirrorComponent_Statistic(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().StatusCount(ctx).Return([]database.MirrorStatusCount{
		{Status: types.MirrorLfsSyncFinished, Count: 100},
	}, nil)

	s, err := mc.Statistics(ctx)
	require.Nil(t, err)
	require.Equal(t, []types.MirrorStatusCount{
		{Status: types.MirrorLfsSyncFinished, Count: 100},
	}, s)

}
