package component

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
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
		UUID:     "user-uuid",
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
		CommitFiles: []types.CommitFile{
			{
				Content: "\n---\nlicense: MIT\n---\n\t",
				Path:    types.ReadmeFileName,
			},
		},
	}).Return(nil, dbrepo, &gitserver.CommitFilesReq{}, nil)

	mc.mocks.gitServer.EXPECT().CommitFiles(ctx, gitserver.CommitFilesReq{}).Return(nil)

	mc.mocks.stores.MCPServerMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.MCPServer{
		RepositoryID:  dbrepo.ID,
		Configuration: req.Configuration,
		Repository:    dbrepo,
	}, "test-namespace/test-server").Return(&database.MCPServer{
		ID:            321,
		RepositoryID:  dbrepo.ID,
		Configuration: req.Configuration,
		Repository:    dbrepo,
	}, nil)

	var wg sync.WaitGroup
	wg.Add(1)
	mc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.MCPServerRepo &&
				req.Operation == types.OperationCreate &&
				req.RepoPath == "test-namespace/test-server" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()
	res, err := mc.Create(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.RepositoryID, int64(321))
	require.Equal(t, res.Name, "test-server")
	require.Equal(t, res.Nickname, "n")
	require.Equal(t, res.Path, "test-namespace/test-server")
	wg.Wait()
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
		UUID:     "user-uuid",
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
		ID:           321,
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
	}).Return(dbrepo, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().Delete(ctx, database.MCPServer{
		ID:           321,
		RepositoryID: 123,
		Repository:   dbrepo,
	}).Return(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	mc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.MCPServerRepo &&
				req.Operation == types.OperationDelete &&
				req.RepoPath == "test-namespace/test-server" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()
	err := mc.Delete(ctx, req)
	require.Nil(t, err)
	wg.Wait()
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
	mc.mocks.components.repo.EXPECT().GetMirrorTaskStatusAndSyncStatus(dbrepo).Return(
		types.MirrorRepoSyncStart, types.SyncStatusInProgress,
	)

	mc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)

	mc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", dbrepo.ID).Return(false, nil)

	res, err := mc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.UserLikes, false)
	require.Equal(t, res.Path, "ns/n")
	require.Equal(t, res.MirrorTaskStatus, types.MirrorRepoSyncStart)
	require.Equal(t, res.SyncStatus, types.SyncStatusInProgress)
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

func TestMCPServerComponent_updateSpaceMetaTag(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	user := &rpc.User{
		Username: "user",
		Roles:    []string{"admin"},
	}

	req := &types.DeployMCPServerReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:  "user",
			Namespace: "test-namespace",
			Name:      "test-server",
			RepoType:  types.SpaceRepo,
		},
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.DefaultBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.SpaceRepo,
	}

	updateReq := &types.UpdateFileReq{
		Username:  req.Username,
		Branch:    types.MainBranch,
		Message:   "update mcp server tag",
		FilePath:  types.REPOCARD_FILENAME,
		RepoType:  types.SpaceRepo,
		Namespace: req.Namespace,
		Name:      req.Name,
		Content:   "LS0tCm1jcHNlcnZlcnM6CiAgICAtIC8KCi0tLQo=",
	}

	mc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, getFileContentReq).Return(&types.File{
		Content: "",
	}, nil)

	mc.mocks.gitServer.EXPECT().UpdateRepoFile(updateReq).Return(nil)

	err := mc.updateSpaceMetaTag(req, user)
	require.Nil(t, err)
}

func TestMCPServerComponent_CheckDeployBranch(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMCPServerComponent(ctx, t)

	req := &types.DeployMCPServerReq{
		CurrentUser: "user",
		MCPRepo: types.RepoRequest{
			Namespace: "ns",
			Name:      "n",
		},
		CreateRepoReq: types.CreateRepoReq{
			Username:  "user",
			Namespace: "test-namespace",
			Name:      "test-server",
			RepoType:  types.SpaceRepo,
		},
		ResourceID: 1,
		ClusterID:  "cls",
	}

	dbrepo := &database.Repository{
		ID:            321,
		Tags:          []database.Tag{{Name: "t1"}},
		Name:          "n",
		License:       "MIT",
		Nickname:      "n",
		Path:          "ns/n",
		DefaultBranch: "master",
	}

	mc.mocks.stores.MCPServerMock().EXPECT().ByPath(ctx, req.MCPRepo.Namespace, req.MCPRepo.Name).Return(&database.MCPServer{
		RepositoryID: 123,
		Repository:   dbrepo,
	}, nil)

	mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, req.CurrentUser, dbrepo).Return(&types.UserRepoPermission{
		CanAdmin: true,
		CanRead:  true,
	}, nil)

	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, req.ResourceID).Return(&database.SpaceResource{
		ID: 1,
	}, nil)

	mc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, req.CurrentUser, req.ClusterID, int64(0), &database.SpaceResource{
		ID: 1,
	}).Return(nil)

	mc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, req.Namespace).Return(database.Namespace{
		ID: 1,
	}, nil)

	mc.mocks.userSvcClient.EXPECT().GetUserInfo(ctx, req.CurrentUser, req.CurrentUser).Return(&rpc.User{
		ID:       1,
		Username: req.CurrentUser,
		Email:    "email@example.com",
		Roles:    []string{"admin"},
	}, nil)

	mc.mocks.stores.MCPServerMock().EXPECT().CreateSpaceAndRepoForDeploy(ctx, &database.Repository{
		UserID:         int64(1),
		Path:           path.Join(req.Namespace, req.Name),
		GitPath:        common.BuildRelativePath(fmt.Sprintf("%ss", string(req.RepoType)), req.Namespace, req.Name),
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        "MIT",
		DefaultBranch:  "master",
		RepositoryType: req.RepoType,
		Hashed:         true,
	}, &database.Space{
		Sdk:           types.MCPSERVER.Name,
		SdkVersion:    "",
		CoverImageUrl: req.CoverImageUrl,
		Env:           "",
		Hardware:      "",
		Secrets:       "",
		Variables:     "",
		Template:      "",
		SKU:           strconv.FormatInt(1, 10), // space resource id
		ClusterID:     req.ClusterID,
	}).Return(nil)

	mc.mocks.gitServer.EXPECT().CopyRepository(mock.Anything, mock.Anything).Return(nil)

	mc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, mock.Anything).Return(&types.File{
		Content: "",
	}, nil)

	mc.mocks.gitServer.EXPECT().UpdateRepoFile(mock.Anything).Return(nil)

	mc.mocks.gitServer.EXPECT().DeleteRepo(mock.Anything, mock.Anything).Return(nil)

	mc.mocks.stores.MCPServerMock().EXPECT().DeleteSpaceAndRepoForDeploy(mock.Anything, int64(0), int64(0)).Return(nil)

	_, err := mc.Deploy(ctx, req)
	require.NotNil(t, err)
}
