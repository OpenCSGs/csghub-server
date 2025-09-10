package component

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceComponent_Show(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Space{
		ID:         1,
		Repository: &database.Repository{ID: 123, Name: "n", Path: "foo/bar"},
		HasAppFile: true,
	}, nil)
	sc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", &database.Repository{
		ID:   123,
		Name: "n",
		Path: "foo/bar",
	}).Return(
		&types.UserRepoPermission{CanRead: true, CanAdmin: true}, nil,
	)
	sc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{Path: "ns"}, nil)

	sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(
		&database.Deploy{
			SvcName: "svc",
		}, nil,
	)

	sc.mocks.deployer.EXPECT().GetReplica(ctx, types.DeployRepo{
		SpaceID:   1,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
	}).Return(0, 0, nil, nil)

	sc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(123)).Return(true, nil)

	space, err := sc.Show(ctx, "ns", "n", "user", false)
	require.Nil(t, err)
	require.Equal(t, &types.Space{
		ID:                   1,
		Name:                 "n",
		Namespace:            &types.Namespace{Path: "ns"},
		UserLikes:            true,
		RepositoryID:         123,
		Status:               "Stopped",
		CanManage:            true,
		User:                 &types.User{},
		Path:                 "foo/bar",
		SensitiveCheckStatus: "Pending",
		Repository: &types.Repository{
			HTTPCloneURL: "/s/foo/bar.git",
			SSHCloneURL:  ":s/foo/bar.git",
		},
		Endpoint: "endpoint/svc",
		SvcName:  "svc",
	}, space)
}

func TestSpaceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.components.repo.EXPECT().PublicToUser(
		ctx, types.SpaceRepo, "user", &types.RepoFilter{Sort: "z", Username: "user"}, 10, 1).Return([]*database.Repository{
		{
			ID:   123,
			Name: "r1",
			Tags: []database.Tag{{Name: "t1"}},
			User: database.User{
				ID:       1,
				Username: "user",
				NickName: "nickname",
				Avatar:   "avatar",
			},
		},
		{
			ID:   124,
			Name: "r2",
			Tags: []database.Tag{{Name: "t2"}},
			User: database.User{
				ID:       1,
				Username: "user",
				NickName: "nickname",
				Avatar:   "avatar",
			},
		},
	}, 100, nil)

	sc.mocks.stores.SpaceMock().EXPECT().ByRepoIDs(ctx, []int64{123, 124}).Return(
		[]database.Space{
			{ID: 11, RepositoryID: 123, Repository: &database.Repository{
				ID:   123,
				Name: "r1",
				User: database.User{
					ID:       1,
					Username: "user",
					NickName: "nickname",
					Avatar:   "avatar",
				},
			}},
			{ID: 12, RepositoryID: 124, Repository: &database.Repository{
				ID:   124,
				Name: "r2",
				User: database.User{
					ID:       1,
					Username: "user",
					NickName: "nickname",
					Avatar:   "avatar",
				},
			}},
			{ID: 12, RepositoryID: 124, Repository: &database.Repository{
				ID:   124,
				Name: "r2",
				User: database.User{
					ID:       1,
					Username: "user",
					NickName: "nickname",
					Avatar:   "avatar",
				},
			}},
		}, nil,
	)

	data, total, err := sc.Index(ctx, &types.RepoFilter{Sort: "z", Username: "user"}, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []*types.Space{
		{
			RepositoryID: 123, Name: "r1", Tags: []types.RepoTag{{Name: "t1"}},
			Status:        "NoAppFile",
			RecomOpWeight: 0,
			User: &types.User{
				Nickname: "nickname",
				Avatar:   "avatar",
			},
		},
		{
			RepositoryID: 124, Name: "r2", Tags: []types.RepoTag{{Name: "t2"}},
			Status:        "NoAppFile",
			RecomOpWeight: 0,
			User: &types.User{
				Nickname: "nickname",
				Avatar:   "avatar",
			},
		},
	}, data)

}

func TestSpaceComponent_OrgSpaces(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "user").Return(membership.RoleAdmin, nil)
	sc.mocks.stores.SpaceMock().EXPECT().ByOrgPath(ctx, "ns", 10, 1, false).Return(
		[]database.Space{
			{ID: 1, Repository: &database.Repository{ID: 11, Name: "r1"}},
			{ID: 2, Repository: &database.Repository{ID: 12, Name: "r2"}},
		}, 100, nil,
	)

	data, total, err := sc.OrgSpaces(ctx, &types.OrgDatasetsReq{
		Namespace:   "ns",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Space{
		{ID: 1, Name: "r1", RepositoryID: 11, Status: "NoAppFile"},
		{ID: 2, Name: "r2", RepositoryID: 12, Status: "NoAppFile"},
	}, data)

}

func TestSpaceComponent_UserSpaces(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().ByUsername(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}, true).Return(
		[]database.Space{
			{ID: 1, RepositoryID: 11, Repository: &database.Repository{ID: 11, Name: "r1"}},
			{ID: 2, RepositoryID: 12, Repository: &database.Repository{ID: 12, Name: "r2"}},
		}, 100, nil,
	)

	data, total, err := sc.UserSpaces(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Space{
		{ID: 1, Name: "r1", RepositoryID: 11, Status: "NoAppFile"},
		{ID: 2, Name: "r2", RepositoryID: 12, Status: "NoAppFile"},
	}, data)

}

func TestSpaceComponent_UserLikeSpaces(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().ByUserLikes(ctx, int64(111), 10, 1).Return(
		[]database.Space{
			{ID: 1, RepositoryID: 11, Repository: &database.Repository{ID: 11, Name: "r1"}},
			{ID: 2, RepositoryID: 12, Repository: &database.Repository{ID: 12, Name: "r2"}},
		}, 100, nil,
	)

	data, total, err := sc.UserLikesSpaces(ctx, &types.UserCollectionReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}, 111)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Space{
		{ID: 1, Name: "r1", Status: "NoAppFile"},
		{ID: 2, Name: "r2", Status: "NoAppFile"},
	}, data)

}

func TestSpaceComponent_ListByPath(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"foo"}).Return(
		[]database.Space{
			{ID: 1, RepositoryID: 11, Repository: &database.Repository{ID: 11, Name: "r1"}},
			{ID: 2, RepositoryID: 12, Repository: &database.Repository{ID: 12, Name: "r2"}},
		}, nil,
	)

	data, err := sc.ListByPath(ctx, []string{"foo"})
	require.Nil(t, err)
	require.Equal(t, []*types.Space{
		{Name: "r1", Status: "NoAppFile", RepositoryID: 11},
		{Name: "r2", Status: "NoAppFile", RepositoryID: 12},
	}, data)

}

func TestSpaceComponent_AllowCallApi(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().ByID(ctx, int64(123)).Return(&database.Space{
		Repository: &database.Repository{Path: "foo/bar", RepositoryType: types.ModelRepo},
	}, nil)

	sc.mocks.components.repo.EXPECT().AllowReadAccess(ctx, types.ModelRepo, "foo", "bar", "user").Return(true, nil)
	allow, err := sc.AllowCallApi(ctx, 123, "user")
	require.Nil(t, err)
	require.True(t, allow)

}

func TestSpaceComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(mock.Anything, "ns", "n").Return(&database.Space{ID: 1}, nil)
	sc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.SpaceRepo,
	}).Return(&database.Repository{
		User: database.User{
			UUID: "user-uuid",
		},
		Path: "ns/n",
	}, nil)
	sc.mocks.stores.SpaceMock().EXPECT().Delete(mock.Anything, database.Space{ID: 1}).Return(nil)

	sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(mock.Anything, int64(1)).Return(
		&database.Deploy{
			RepoID: 2,
			UserID: 3,
			ID:     4,
		}, nil,
	)
	var wgstop sync.WaitGroup
	wgstop.Add(1)
	sc.mocks.deployer.EXPECT().Stop(mock.Anything, mock.MatchedBy(func(req types.DeployRepo) bool {
		return req.SpaceID == 1 &&
			req.Namespace == "ns" &&
			req.Name == "n"
	})).
		RunAndReturn(func(ctx context.Context, req types.DeployRepo) error {
			wgstop.Done()
			return nil
		}).Once()
	sc.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(
		mock.Anything, types.SpaceRepo, int64(2), int64(3), int64(4),
	).Return(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	sc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.SpaceRepo &&
				req.Operation == types.OperationDelete &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()

	err := sc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)
	wg.Wait()
	wgstop.Wait()
}

func TestSpaceComponent_Deploy(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		Username: "user1",
	}, nil)
	t.Run("Deploy", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns1", "n1").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo1/bar1"},
			SKU:        "1",
			HasAppFile: true,
		}, nil)
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID: 1,
		}, nil)
		sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "user", "", int64(0), &database.SpaceResource{
			ID: 1,
		}).Return(nil)
		sc.mocks.deployer.EXPECT().Deploy(ctx, types.DeployRepo{
			SpaceID:       1,
			Path:          "foo1/bar1",
			Annotation:    "{\"hub-deploy-user\":\"user1\",\"hub-res-name\":\"ns1/n1\",\"hub-res-type\":\"space\"}",
			ContainerPort: 8080,
			SKU:           "1",
		}).Return(123, nil)

		id, err := sc.Deploy(ctx, "ns1", "n1", "user")
		require.Nil(t, err)
		require.Equal(t, int64(123), id)
	})
	t.Run("DeployWithoutAppFile", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns2", "n2").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo2/bar2"},
			SKU:        "1",
			HasAppFile: false,
		}, nil)
		id, err := sc.Deploy(ctx, "ns2", "n2", "user")
		require.Equal(t, true, errors.Is(err, errorx.ErrNoEntryFile))
		require.Equal(t, int64(-1), id)
	})
}

func TestSpaceComponent_Wakeup(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)
	t.Run("Wakeup", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Space{
			ID:         1,
			HasAppFile: true,
		}, nil)

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(
			&database.Deploy{SvcName: "svc"}, nil,
		)

		sc.mocks.deployer.EXPECT().Wakeup(ctx, types.DeployRepo{
			SpaceID:   1,
			Namespace: "ns",
			Name:      "n",
			SvcName:   "svc",
		}).Return(nil)

		err := sc.Wakeup(ctx, "ns", "n")
		require.Nil(t, err)
	})
	t.Run("WakeupWithoutAppFile", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns2", "n2").Return(&database.Space{
			ID:         1,
			HasAppFile: false,
		}, nil)

		err := sc.Wakeup(ctx, "ns2", "n2")
		require.Equal(t, true, errors.Is(err, errorx.ErrNoEntryFile))
	})

}

func TestSpaceComponent_Stop(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	t.Run("Stop", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Space{
			ID:         1,
			HasAppFile: true,
		}, nil)

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(
			&database.Deploy{SvcName: "svc", RepoID: 1, UserID: 2, ID: 3}, nil,
		)

		sc.mocks.deployer.EXPECT().Stop(ctx, types.DeployRepo{
			SpaceID:   1,
			Namespace: "ns",
			Name:      "n",
			SvcName:   "svc",
		}).Return(nil)
		sc.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(
			ctx, types.SpaceRepo, int64(1), int64(2), int64(3),
		).Return(nil)

		err := sc.Stop(ctx, "ns", "n", false)
		require.Nil(t, err)
	})
	t.Run("StopWithoutAppFile", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns2", "n2").Return(&database.Space{
			ID:         1,
			HasAppFile: false,
		}, nil)

		err := sc.Stop(ctx, "ns2", "n2", false)
		require.Equal(t, true, errors.Is(err, errorx.ErrNoEntryFile))
	})
}

func TestSpaceComponent_FixHasEntryFile(t *testing.T) {

	cases := []struct {
		nginx bool
		name  string
		tp    string
		exist bool
	}{
		{false, "app.py", "file", true},
		{false, "app.py", "foo", false},
		{false, "z.py", "file", false},
		{true, "app.py", "file", false},
		{true, "nginx.conf", "file", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			ctx := context.TODO()
			sc := initializeTestSpaceComponent(ctx, t)

			sc.mocks.gitServer.EXPECT().GetRepoFileTree(ctx, gitserver.GetRepoInfoByPathReq{
				Namespace: "foo",
				Name:      "bar",
				RepoType:  types.SpaceRepo,
			}).Return([]*types.File{
				{Type: c.tp, Path: c.name},
			}, nil)
			sdk := ""
			if c.nginx {
				sdk = types.NGINX.Name
			}
			exist := sc.HasEntryFile(ctx, &database.Space{
				Repository: &database.Repository{Path: "foo/bar"},
				Sdk:        sdk,
			})
			require.Equal(t, c.exist, exist)
		})
	}
}

func TestSpaceComponent_Logs(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)
	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Space{
		ID: 1,
	}, nil)

	sc.mocks.deployer.EXPECT().Logs(ctx, types.DeployRepo{
		SpaceID:   1,
		Namespace: "ns",
		Name:      "n",
	}).Return(&deploy.MultiLogReader{}, nil)

	r, err := sc.Logs(ctx, "ns", "n")

	require.Nil(t, err)
	require.Equal(t, &deploy.MultiLogReader{}, r)

}

func TestSpaceComponent_GetByID(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)
	sc.mocks.stores.SpaceMock().EXPECT().ByID(ctx, int64(1)).Return(&database.Space{
		ID: 1,
	}, nil)

	r, err := sc.GetByID(ctx, int64(1))

	require.Nil(t, err)
	require.Equal(t, &database.Space{
		ID: 1,
	}, r)
}

func TestSpaceComponent_MCPIndex(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.components.repo.EXPECT().PublicToUser(
		ctx, types.SpaceRepo, "user", &types.RepoFilter{Sort: "z", Username: "user"}, 10, 1).Return([]*database.Repository{
		{ID: 123, Name: "r1", Tags: []database.Tag{{Name: "t1"}}},
		{ID: 124, Name: "r2", Tags: []database.Tag{{Name: "t2"}}},
	}, 100, nil)

	sc.mocks.stores.SpaceMock().EXPECT().ByRepoIDs(ctx, []int64{123, 124}).Return(
		[]database.Space{
			{ID: 11, RepositoryID: 123, Repository: &database.Repository{
				ID:   123,
				Name: "r1",
			}},
			{ID: 12, RepositoryID: 124, Repository: &database.Repository{
				ID:   124,
				Name: "r2",
			}},
		}, nil,
	)

	data, total, err := sc.MCPIndex(ctx, &types.RepoFilter{Sort: "z", Username: "user"}, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []*types.MCPService{
		{ID: 11, RepositoryID: 123, Name: "r1", Status: "NoAppFile"},
		{ID: 12, RepositoryID: 124, Name: "r2", Status: "NoAppFile"},
	}, data)

}
