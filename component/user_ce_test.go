//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestUserComponent_ListDeploys(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.DeployReq{
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.DeployTaskMock().EXPECT().ListDeployByOwnerNamespace(ctx, "user", req).Return([]database.Deploy{
		{
			SvcName: "svc", ClusterID: "cluster", SKU: "sku",
			GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123,
		},
	}, 100, nil)
	uc.mocks.components.repo.EXPECT().GenerateEndpoint(ctx, &database.Deploy{
		SvcName:   "svc",
		ClusterID: "cluster",
	}).Return("endpoint", "foo")
	uc.mocks.stores.RepoMock().EXPECT().TagsWithCategory(ctx, int64(123), "task").Return([]database.Tag{
		{Name: "tag1"},
		{Name: "tag2"},
	}, nil)

	data, total, err := uc.ListDeploys(ctx, types.ModelRepo, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.DeployRequest{
		{
			Path: "foo/bar", Status: "Pending", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123, SvcName: "svc", Endpoint: "endpoint", ClusterID: "cluster",
			Provider: "foo", RepoTag: "tag1",
			ResourceType: "cpu",
		},
	}, data)

}

func TestUserComponent_ListInstances(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserRepoReq{
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.DeployTaskMock().EXPECT().ListFinetunesByOwnerNamespace(ctx, "user", 10, 1).Return([]database.Deploy{
		{
			SvcName: "svc", ClusterID: "cluster", SKU: "sku",
			GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123,
		},
	}, 100, nil)

	data, total, err := uc.ListInstances(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.DeployRequest{
		{
			Path: "foo/bar", Status: "Pending", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123, SvcName: "svc", ClusterID: "cluster",
		},
	}, data)

}

func TestUserComponent_ListDeploysByNamespace(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.OrgRunDeploysReq{
		Namespace:   "org1",
		CurrentUser: "user",
		RepoType:    types.ModelRepo,
		DeployType:  types.InferenceType,
		PageOpts:    types.PageOpts{Page: 1, PageSize: 10},
	}
	deployReq := &types.DeployReq{
		PageOpts:   types.PageOpts{Page: 1, PageSize: 10},
		RepoType:   types.ModelRepo,
		DeployType: types.InferenceType,
	}
	uc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "org1", membership.RoleRead).Return(true, nil)
	uc.mocks.stores.DeployTaskMock().EXPECT().ListDeployByOwnerNamespace(ctx, "org1", deployReq).Return([]database.Deploy{
		{
			SvcName: "svc", ClusterID: "cluster", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123, DeployName: "d1",
		},
	}, 100, nil)
	uc.mocks.components.repo.EXPECT().GenerateEndpoint(ctx, &database.Deploy{
		SvcName:   "svc",
		ClusterID: "cluster",
	}).Return("endpoint", "foo")
	uc.mocks.stores.RepoMock().EXPECT().TagsWithCategory(ctx, int64(123), "task").Return([]database.Tag{
		{Name: "tag1"},
	}, nil)

	data, total, err := uc.ListDeploysByNamespace(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Len(t, data, 1)
	require.Equal(t, "foo/bar", data[0].Path)
	require.Equal(t, "svc", data[0].SvcName)
	require.Equal(t, "endpoint", data[0].Endpoint)
	require.Equal(t, "tag1", data[0].RepoTag)
}

func TestUserComponent_ListNotebooksByNamespace(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.OrgNotebooksReq{
		Namespace:   "org1",
		CurrentUser: "user",
		PageOpts:    types.PageOpts{Page: 1, PageSize: 10},
	}
	deployReq := &types.DeployReq{
		PageOpts:   types.PageOpts{Page: 1, PageSize: 10},
		DeployType: types.NotebookType,
	}
	uc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user", "org1", membership.RoleRead).Return(true, nil)
	uc.mocks.stores.DeployTaskMock().EXPECT().ListDeployByOwnerNamespace(ctx, "org1", deployReq).Return([]database.Deploy{
		{
			ID: 1, DeployName: "nb1", SvcName: "svc", ClusterID: "cluster",
			ImageID: "img:1.0", Hardware: `{}`, RuntimeFramework: "jupyter",
		},
	}, 1, nil)
	uc.mocks.components.repo.EXPECT().GenerateEndpoint(ctx, &database.Deploy{
		SvcName:   "svc",
		ClusterID: "cluster",
	}).Return("https://ep", "provider")

	data, total, err := uc.ListNotebooksByNamespace(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, data, 1)
	require.Equal(t, "nb1", data[0].DeployName)
	require.Equal(t, "https://ep", data[0].Endpoint)
	require.Equal(t, "1.0", data[0].RuntimeFrameworkVersion)
}
