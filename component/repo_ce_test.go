//go:build !saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoComponent_DeployUpdate(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	req := &types.DeployUpdateReq{
		ResourceID: tea.Int64(111),
		ClusterID:  tea.String("cluster"),
	}

	deploy := &database.Deploy{
		ID:               1,
		SpaceID:          2,
		ModelID:          3,
		SvcName:          "svc",
		ClusterID:        "cluster",
		RuntimeFramework: "fm",
	}
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(deploy, nil)
	repo.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(111)).Return(&database.SpaceResource{
		ID:        111,
		ClusterID: "cluster",
		Resources: `{ "gpu": { "type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": { "aliyun.accelerator/nvidia_name": "NVIDIA-A10" } }, "cpu": { "type": "Intel", "num": "12" },  "memory": "46Gi" }`,
	}, nil)

	repo.mocks.deployer.EXPECT().CheckResourceAvailable(ctx, "cluster", int64(0), mock.Anything).Return(true, nil)

	repo.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByName(ctx, "fm").Return(&database.RuntimeFramework{
		ID: 999,
	}, nil)
	repo.mocks.stores.ClusterInfoMock().EXPECT().ByClusterID(ctx, "cluster").Return(database.ClusterInfo{}, nil)

	repo.mocks.deployer.EXPECT().Exist(ctx, types.DeployRepo{
		DeployID:  1,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
		SpaceID:   2,
		ModelID:   3,
	}).Return(false, nil)
	repo.mocks.deployer.EXPECT().UpdateDeploy(ctx, req, deploy).Return(nil)

	err := repo.DeployUpdate(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     1,
		DeployType:   1,
		InstanceName: "i1",
	}, req)
	require.Nil(t, err)
}

func TestRepoComponent_DeployStart(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	deploy := &database.Deploy{
		ID:               1,
		SpaceID:          2,
		ModelID:          3,
		SvcName:          "svc",
		ClusterID:        "cluster",
		RuntimeFramework: "fm",
		SKU:              "111",
	}
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(deploy, nil)

	repo.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(111)).Return(&database.SpaceResource{
		ID:        111,
		Resources: `{ "gpu": { "type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": { "aliyun.accelerator/nvidia_name": "NVIDIA-A10" } }, "cpu": { "type": "Intel", "num": "12" },  "memory": "46Gi" }`,
	}, nil)

	repo.mocks.deployer.EXPECT().CheckResourceAvailable(ctx, "cluster", int64(0), mock.Anything).Return(true, nil)

	repo.mocks.deployer.EXPECT().Exist(ctx, types.DeployRepo{
		DeployID:  1,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
		SpaceID:   2,
		ModelID:   3,
	}).Return(false, nil)

	repo.mocks.deployer.EXPECT().StartDeploy(ctx, deploy).Return(nil)

	err := repo.DeployStart(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     1,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.Nil(t, err)
}

func TestRepoComponent_DeployStart_ExistAndRunning(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	deploy := &database.Deploy{
		ID:               1,
		SpaceID:          2,
		ModelID:          3,
		SvcName:          "svc",
		ClusterID:        "cluster",
		RuntimeFramework: "fm",
		SKU:              "111",
	}
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(deploy, nil)

	repo.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(111)).Return(&database.SpaceResource{
		ID:        111,
		Resources: `{ "gpu": { "type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": { "aliyun.accelerator/nvidia_name": "NVIDIA-A10" } }, "cpu": { "type": "Intel", "num": "12" },  "memory": "46Gi" }`,
	}, nil)

	repo.mocks.deployer.EXPECT().CheckResourceAvailable(ctx, "cluster", int64(0), mock.Anything).Return(true, nil)

	deployRepo := types.DeployRepo{
		DeployID:  1,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
		SpaceID:   2,
		ModelID:   3,
	}
	repo.mocks.deployer.EXPECT().Exist(ctx, deployRepo).Return(true, nil)
	
	// status 4 means running
	repo.mocks.deployer.EXPECT().Status(ctx, deployRepo, false).Return("svc", 4, []types.Instance{}, nil)

	err := repo.DeployStart(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     1,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "stop deploy first")
}

func TestRepoComponent_DeployStart_ExistButNotRunning(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	mockUserRepoAdminPermission(ctx, repo.mocks.stores, "user")

	deploy := &database.Deploy{
		ID:               1,
		SpaceID:          2,
		ModelID:          3,
		SvcName:          "svc",
		ClusterID:        "cluster",
		RuntimeFramework: "fm",
		SKU:              "111",
	}
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(deploy, nil)

	repo.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(111)).Return(&database.SpaceResource{
		ID:        111,
		Resources: `{ "gpu": { "type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": { "aliyun.accelerator/nvidia_name": "NVIDIA-A10" } }, "cpu": { "type": "Intel", "num": "12" },  "memory": "46Gi" }`,
	}, nil)

	repo.mocks.deployer.EXPECT().CheckResourceAvailable(ctx, "cluster", int64(0), mock.Anything).Return(true, nil)

	deployRepo := types.DeployRepo{
		DeployID:  1,
		Namespace: "ns",
		Name:      "n",
		SvcName:   "svc",
		ClusterID: "cluster",
		SpaceID:   2,
		ModelID:   3,
	}
	repo.mocks.deployer.EXPECT().Exist(ctx, deployRepo).Return(true, nil)
	
	// status 2 means failed (not running)
	repo.mocks.deployer.EXPECT().Status(ctx, deployRepo, false).Return("svc", 2, []types.Instance{}, nil)
	
	// should call Stop first
	repo.mocks.deployer.EXPECT().Stop(ctx, deployRepo).Return(nil)
	
	// then start deploy
	repo.mocks.deployer.EXPECT().StartDeploy(ctx, deploy).Return(nil)

	err := repo.DeployStart(ctx, types.DeployActReq{
		RepoType:     types.ModelRepo,
		Namespace:    "ns",
		Name:         "n",
		CurrentUser:  "user",
		DeployID:     1,
		DeployType:   1,
		InstanceName: "i1",
	})
	require.Nil(t, err)
}

func TestRepoComponentImpl_Update(t *testing.T) {
	ctx := context.TODO()

	t.Run("Successfully update repository", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		updatedRepo := &database.Repository{
			ID:             1,
			Path:           "test-namespace/test-repo",
			Name:           "test-repo",
			Nickname:       "New Nickname",
			Description:    "New Description",
			DefaultBranch:  "main",
			Private:        false,
			RepositoryType: types.ModelRepo,
		}
		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(updatedRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		assert.Equal(t, updatedRepo, result)
	})

	// Test case 2: Repository does not exist
	t.Run("Repository does not exist", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		// Mock repository lookup failure
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "non-exist-repo").Return(nil, errors.New("repository not found"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace: "test-namespace",
			Name:      "non-exist-repo",
			Username:  "test-user",
			RepoType:  types.ModelRepo,
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository does not exist")
		assert.Nil(t, result)
	})

	// Test case 3: Namespace does not exist
	t.Run("Namespace does not exist", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:             1,
			Path:           "non-exist-namespace/test-repo",
			Name:           "test-repo",
			RepositoryType: types.ModelRepo,
		}

		// Mock successful repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "non-exist-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup failure
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "non-exist-namespace").Return(database.Namespace{}, errors.New("namespace not found"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace: "non-exist-namespace",
			Name:      "test-repo",
			Username:  "test-user",
			RepoType:  types.ModelRepo,
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespace does not exist")
		assert.Nil(t, result)
	})

	// Test case 4: Git server update failure
	t.Run("Git server update failure", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock Git server update failure
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(nil, errors.New("git server error"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fail to update repo in git")
		assert.Nil(t, result)
	})

	// Test case 5: Database update failure
	t.Run("Database update failure", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update failure
		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(nil, errors.New("database error"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fail to update repo in database")
		assert.Nil(t, result)
	})
	// Test case 6: allow public for dataset repo with sensitive check pending
	t.Run("allow public for dataset repo with sensitive check pending", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "dataset", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		newRepo := *mockRepo
		newRepo.Private = false
		newRepo.Nickname = "New Nickname"
		newRepo.Description = "New Description"

		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&newRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.DatasetRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		require.Equal(t, *mockRepo, *result)
	})
	// Test case 7: allow public for dataset repo with sensitive check pass
	t.Run("allow public for dataset repo with sensitive check pass", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPass,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "dataset", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		newRepo := *mockRepo
		newRepo.Private = false
		newRepo.Nickname = "New Nickname"
		newRepo.Description = "New Description"

		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&newRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.DatasetRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		require.Equal(t, *mockRepo, *result)
	})
}

func TestRepoComponent_RemoteDiff(t *testing.T) {
	ctx := context.TODO()
	repoComp := initializeTestRepoComponent(ctx, t)

	req := types.RemoteDiffReq{
		Namespace:    "test-namespace",
		Name:         "test-repo",
		RepoType:     types.ModelRepo,
		LeftCommitID: "left-commit-id",
	}

	resp := []types.RemoteDiffs{
		{
			Added:    []string{"file1"},
			Removed:  []string{"file2"},
			Modified: []string{"file3"},
		},
	}

	repoComp.mocks.multiSyncClient.EXPECT().Diff(ctx, req).Return(resp, nil)

	req1 := types.GetDiffBetweenCommitsReq{
		Namespace:    "test-namespace",
		Name:         "test-repo",
		RepoType:     types.ModelRepo,
		LeftCommitID: "left-commit-id",
	}
	res, err := repoComp.RemoteDiff(ctx, req1)
	require.Nil(t, err)
	assert.Equal(t, resp, res)
}
