package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMCPServerComponent_Create(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	req := &types.CreateMCPServerReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:      "user",
			Namespace:     "test-namespace",
			Name:          "test-server",
			Nickname:      "n",
			License:       "MIT",
			DefaultBranch: "main",
		},
	}

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}

	dbrepo := &database.Repository{
		ID:       321,
		User:     user,
		Tags:     []database.Tag{{Name: "t1"}},
		Name:     "test-server",
		License:  "MIT",
		Nickname: "n",
		Path:     "test-namespace/test-server",
	}

	mc.mocks.components.repo.EXPECT().CreateRepo(ctx, types.CreateRepoReq{
		DefaultBranch: "main",
		Readme:        generateReadmeData("MIT"),
		License:       "MIT",
		Namespace:     "test-namespace",
		Name:          "test-server",
		Nickname:      "n",
		RepoType:      types.MCPServerRepo,
		Username:      "user",
	}).Return(nil, dbrepo, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().Create(ctx, database.MCPServer{
		RepositoryID:  dbrepo.ID,
		Configuration: req.Configuration,
		Repository:    dbrepo,
	}).Return(&database.MCPServer{
		RepositoryID:  dbrepo.ID,
		Configuration: req.Configuration,
		Repository:    dbrepo,
	}, nil)

	mc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbrepo.User.Username,
		Email:     dbrepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   generateReadmeData("MIT"),
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.ReadmeFileName,
	}, types.MCPServerRepo)).Return(nil)

	res, err := mc.Create(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.RepositoryID, int64(321))
	require.Equal(t, res.Name, "test-server")
	require.Equal(t, res.Nickname, "n")
	require.Equal(t, res.Path, "test-namespace/test-server")
}

func TestMCPServerComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	req := &types.UpdateMCPServerReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Username:  "user",
			Namespace: "test-namespace",
			Name:      "test-server",
		},
	}

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}

	dbrepo := &database.Repository{
		ID:       321,
		User:     user,
		Tags:     []database.Tag{{Name: "t1"}},
		Name:     "test-server",
		License:  "MIT",
		Nickname: "n",
		Path:     "test-namespace/test-server",
	}

	mc.mocks.stores.MCPServerMock().EXPECT().ByPath(ctx, "test-namespace", "test-server").Return(&database.MCPServer{
		RepositoryID: 123,
		Repository:   dbrepo,
	}, nil)

	mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dbrepo).Return(&types.UserRepoPermission{
		CanAdmin: true,
	}, nil)

	mc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  req.Username,
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  types.MCPServerRepo,
	}).Return(nil, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().Delete(ctx, database.MCPServer{
		RepositoryID: 123,
		Repository:   dbrepo,
	}).Return(nil)

	err := mc.Delete(ctx, req)
	require.Nil(t, err)
}

func TestMCPServerComponent_Update(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	config := "abc"

	req := &types.UpdateMCPServerReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Username:  "user",
			Namespace: "test-namespace",
			Name:      "test-server",
			RepoType:  types.MCPServerRepo,
		},
		Configuration: &config,
	}

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}

	dbrepo := &database.Repository{
		ID:       321,
		User:     user,
		Tags:     []database.Tag{{Name: "t1"}},
		Name:     "test-server",
		License:  "MIT",
		Nickname: "n",
		Path:     "test-namespace/test-server",
	}

	mc.mocks.stores.MCPServerMock().EXPECT().ByPath(ctx, "test-namespace", "test-server").Return(&database.MCPServer{
		RepositoryID: 123,
		Repository:   dbrepo,
	}, nil)

	mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dbrepo).Return(&types.UserRepoPermission{
		CanAdmin: true,
	}, nil)

	mc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(dbrepo, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().Update(ctx, database.MCPServer{
		RepositoryID:  int64(123),
		Repository:    dbrepo,
		Configuration: config,
	}).Return(&database.MCPServer{
		RepositoryID:  int64(123),
		Repository:    dbrepo,
		Configuration: config,
	}, nil)

	res, err := mc.Update(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.Configuration, config)
}

func TestMCPServerComponent_Show(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}

	dbrepo := &database.Repository{
		ID:       321,
		User:     user,
		Tags:     []database.Tag{{Name: "t1"}},
		Name:     "n",
		License:  "MIT",
		Nickname: "n",
		Path:     "ns/n",
	}

	mc.mocks.stores.MCPServerMock().EXPECT().ByPath(ctx, "ns", "n").Return(&database.MCPServer{
		RepositoryID: 123,
		Repository:   dbrepo,
	}, nil)

	mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dbrepo).Return(&types.UserRepoPermission{
		CanRead: true,
	}, nil)

	mc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)

	mc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", dbrepo.ID).Return(false, nil)

	res, err := mc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.UserLikes, false)
	require.Equal(t, res.Path, "ns/n")
}

func TestMCPServerComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}

	dbrepo := &database.Repository{
		ID:       321,
		User:     user,
		Tags:     []database.Tag{{Name: "t1"}},
		Name:     "n",
		License:  "MIT",
		Nickname: "n",
		Path:     "ns/n",
	}

	filter := &types.RepoFilter{
		Username: "user",
	}

	mc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.MCPServerRepo, "user", filter, 10, 1).
		Return([]*database.Repository{dbrepo}, 1, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().ByRepoIDs(ctx, []int64{321}).Return([]database.MCPServer{
		{
			RepositoryID: 321,
			Repository:   dbrepo,
		},
	}, nil)

	res, total, err := mc.Index(ctx, filter, 10, 1, false)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, total, 1)
}

func TestMCPServerComponent_Properties(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	req := &types.MCPPropertyFilter{
		CurrentUser: "user",
		Kind:        types.MCPPropTool,
	}

	mc.mocks.userSvcClient.EXPECT().GetUserInfo(ctx, "user", "user").Return(&rpc.User{
		Username: "user",
		Roles:    []string{"admin"},
	}, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().ListProperties(ctx, req).Return([]database.MCPServerProperty{
		{
			ID:          1,
			Kind:        "tool",
			MCPServerID: 1,
			MCPServer: &database.MCPServer{
				RepositoryID: 1,
				Repository:   &database.Repository{},
			},
		},
	}, 1, nil)

	res, total, err := mc.Properties(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, total, 1)
}

func TestMCPServerComponent_OrgMCPServers(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	cases := []struct {
		role       membership.Role
		publicOnly bool
	}{
		{membership.RoleUnknown, true},
		{membership.RoleAdmin, false},
	}

	for _, c := range cases {
		t.Run(string(c.role), func(t *testing.T) {
			mc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "foo").Return(c.role, nil).Once()
			mc.mocks.stores.MCPServerMock().EXPECT().ByOrgPath(ctx, "ns", 1, 1, c.publicOnly).Return([]database.MCPServer{
				{ID: 1, Repository: &database.Repository{Name: "r1"}},
				{ID: 2, Repository: &database.Repository{Name: "r2"}},
				{ID: 3, Repository: &database.Repository{Name: "r3"}},
			}, 100, nil)
			res, count, err := mc.OrgMCPServers(ctx, &types.OrgMCPsReq{
				Namespace:   "ns",
				CurrentUser: "foo",
				PageOpts: types.PageOpts{
					Page:     1,
					PageSize: 1,
				},
			})
			require.Nil(t, err)
			require.Equal(t, 100, count)
			require.Equal(t, []types.MCPServer{
				{ID: 1, Name: "r1"},
				{ID: 2, Name: "r2"},
				{ID: 3, Name: "r3"},
			}, res)
		})

	}
}
