package component

import (
	"context"
	"errors"
	"fmt"
	"testing"

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

	dbRepo := &database.Repository{ID: 123, Name: "n", Path: "foo/bar", Tags: []database.Tag{{Name: "t1"}}}
	dbSpace := &database.Space{
		ID:         1,
		Repository: dbRepo,
		HasAppFile: true,
	}

	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dbSpace, nil)
	sc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dbRepo).Return(
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
		Tags:     []types.RepoTag{{Name: "t1"}},
		Endpoint: "endpoint/svc",
		SvcName:  "svc",
	}, space)
	require.Equal(t, []types.RepoTag{{Name: "t1"}}, space.Tags)
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

	r, err := sc.Logs(ctx, "ns", "n", "")

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

func TestSpaceComponent_GetMCPServiceBySvcName(t *testing.T) {
	ctx := context.Background()
	sc := initializeTestSpaceComponent(ctx, t)
	svcName := "test-svc"

	deploy := &database.Deploy{
		ID:      1,
		SpaceID: 1,
		SvcName: svcName,
		Status:  23, // Corresponds to Running
		RepoID:  1,
	}

	repo := database.Repository{
		ID:          1,
		Name:        "test-space",
		Path:        "test/test-space",
		Description: "test description",
		License:     "mit",
		Private:     false,
		User:        database.User{},
	}

	space := &database.Space{
		ID:           1,
		RepositoryID: 1,
		Repository:   &repo,
		HasAppFile:   true,
		Env:          "{}",
	}

	t.Run("success", func(t *testing.T) {
		sc.mocks.stores.DeployTaskMock().EXPECT().GetDeployBySvcName(ctx, svcName).Return(deploy, nil).Once()
		sc.mocks.stores.SpaceMock().EXPECT().ByID(ctx, deploy.SpaceID).Return(space, nil).Once()
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, space.ID).Return(deploy, nil).Once()

		mcpService, err := sc.GetMCPServiceBySvcName(ctx, svcName)

		require.NoError(t, err)
		require.NotNil(t, mcpService)
		require.Equal(t, space.ID, mcpService.ID)
		require.Equal(t, "test-space", mcpService.Name)
		require.Equal(t, "test/test-space", mcpService.Path)
		require.Equal(t, "test-svc", mcpService.SvcName)
		require.Equal(t, "Running", mcpService.Status)
		require.Equal(t, "endpoint/test-svc", mcpService.Endpoint)
	})

	t.Run("deploy not found", func(t *testing.T) {
		sc.mocks.stores.DeployTaskMock().EXPECT().GetDeployBySvcName(ctx, svcName).Return(nil, errors.New("not found")).Once()

		_, err := sc.GetMCPServiceBySvcName(ctx, svcName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get deploy by svcName")
	})

	t.Run("space not found", func(t *testing.T) {
		sc.mocks.stores.DeployTaskMock().EXPECT().GetDeployBySvcName(ctx, svcName).Return(deploy, nil).Once()
		sc.mocks.stores.SpaceMock().EXPECT().ByID(ctx, deploy.SpaceID).Return(nil, errors.New("not found")).Once()

		_, err := sc.GetMCPServiceBySvcName(ctx, svcName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get space by id")
	})
}

func TestSpaceComponent_StatusByPaths(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	t.Run("empty paths", func(t *testing.T) {
		result, err := sc.StatusByPaths(ctx, []string{})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 0, len(result))
	})

	t.Run("space not found", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"nonexistent/space"}).
			Return([]database.Space{}, nil).Once()

		// When no spaces are found, spaceIDs will be empty, but GetLatestDeploysBySpaceIDs is still called
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{}).
			Return(map[int64]*database.Deploy{}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"nonexistent/space"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusEmpty, result["nonexistent/space"])
	})

	t.Run("space without HasAppFile - NGINX", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 1, Path: "ns1/space1"}
		dbSpace := &database.Space{
			ID:         1,
			Repository: dbRepo,
			HasAppFile: false,
			Sdk:        types.NGINX.Name,
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns1/space1"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		// GetLatestDeploysBySpaceIDs is called with all found space IDs, even if they don't have HasAppFile
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{1}).
			Return(map[int64]*database.Deploy{}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns1/space1"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusNoNGINXConf, result["ns1/space1"])
	})

	t.Run("space without HasAppFile - non-NGINX", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 2, Path: "ns2/space2"}
		dbSpace := &database.Space{
			ID:         2,
			Repository: dbRepo,
			HasAppFile: false,
			Sdk:        "streamlit",
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns2/space2"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		// GetLatestDeploysBySpaceIDs is called with all found space IDs, even if they don't have HasAppFile
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{2}).
			Return(map[int64]*database.Deploy{}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns2/space2"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusNoAppFile, result["ns2/space2"])
	})

	t.Run("space with HasAppFile but no deploy", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 3, Path: "ns3/space3"}
		dbSpace := &database.Space{
			ID:         3,
			Repository: dbRepo,
			HasAppFile: true,
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns3/space3"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{3}).
			Return(map[int64]*database.Deploy{}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns3/space3"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusStopped, result["ns3/space3"])
	})

	t.Run("space with HasAppFile and deploy - Running", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 4, Path: "ns4/space4"}
		dbSpace := &database.Space{
			ID:         4,
			Repository: dbRepo,
			HasAppFile: true,
		}

		deploy := &database.Deploy{
			ID:     1,
			Status: 23, // Running
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns4/space4"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{4}).
			Return(map[int64]*database.Deploy{4: deploy}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns4/space4"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusRunning, result["ns4/space4"])
	})

	t.Run("space with HasAppFile and deploy - Stopped", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 5, Path: "ns5/space5"}
		dbSpace := &database.Space{
			ID:         5,
			Repository: dbRepo,
			HasAppFile: true,
		}

		deploy := &database.Deploy{
			ID:     2,
			Status: 26, // Stopped
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns5/space5"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{5}).
			Return(map[int64]*database.Deploy{5: deploy}, nil).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns5/space5"})
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(result))
		require.Equal(t, SpaceStatusStopped, result["ns5/space5"])
	})

	t.Run("multiple spaces with different scenarios", func(t *testing.T) {
		paths := []string{"ns6/space6", "ns7/space7", "ns8/space8", "ns9/space9"}

		dbRepo6 := &database.Repository{ID: 6, Path: "ns6/space6"}
		dbSpace6 := &database.Space{
			ID:         6,
			Repository: dbRepo6,
			HasAppFile: false,
			Sdk:        types.NGINX.Name,
		}

		dbRepo7 := &database.Repository{ID: 7, Path: "ns7/space7"}
		dbSpace7 := &database.Space{
			ID:         7,
			Repository: dbRepo7,
			HasAppFile: true,
		}

		dbRepo8 := &database.Repository{ID: 8, Path: "ns8/space8"}
		dbSpace8 := &database.Space{
			ID:         8,
			Repository: dbRepo8,
			HasAppFile: true,
		}

		deploy8 := &database.Deploy{
			ID:     3,
			Status: 23, // Running
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, paths).
			Return([]database.Space{*dbSpace6, *dbSpace7, *dbSpace8}, nil).Once()

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{6, 7, 8}).
			Return(map[int64]*database.Deploy{8: deploy8}, nil).Once()

		result, err := sc.StatusByPaths(ctx, paths)
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, 4, len(result))

		// Space 6: no app file, NGINX
		require.Equal(t, SpaceStatusNoNGINXConf, result["ns6/space6"])

		// Space 7: has app file but no deploy
		require.Equal(t, SpaceStatusStopped, result["ns7/space7"])

		// Space 8: has app file and running deploy
		require.Equal(t, SpaceStatusRunning, result["ns8/space8"])

		// Space 9: not found
		require.Equal(t, SpaceStatusEmpty, result["ns9/space9"])
	})

	t.Run("ListByPath error", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"error/space"}).
			Return(nil, errors.New("database error")).Once()

		result, err := sc.StatusByPaths(ctx, []string{"error/space"})
		require.NotNil(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to find spaces by paths")
	})

	t.Run("GetLatestDeploysBySpaceIDs error", func(t *testing.T) {
		dbRepo := &database.Repository{ID: 10, Path: "ns10/space10"}
		dbSpace := &database.Space{
			ID:         10,
			Repository: dbRepo,
			HasAppFile: true,
		}

		sc.mocks.stores.SpaceMock().EXPECT().ListByPath(ctx, []string{"ns10/space10"}).
			Return([]database.Space{*dbSpace}, nil).Once()

		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeploysBySpaceIDs(ctx, []int64{10}).
			Return(nil, errors.New("deploy query error")).Once()

		result, err := sc.StatusByPaths(ctx, []string{"ns10/space10"})
		require.NotNil(t, err)
		require.Nil(t, result)
	})
}
