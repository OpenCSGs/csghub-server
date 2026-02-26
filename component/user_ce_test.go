//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
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
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.DeployTaskMock().EXPECT().ListDeployByUserID(ctx, int64(1), req).Return([]database.Deploy{
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
	require.Equal(t, []types.DeployRepo{
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
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.DeployTaskMock().EXPECT().ListInstancesByUserID(ctx, int64(1), 10, 1).Return([]database.Deploy{
		{
			SvcName: "svc", ClusterID: "cluster", SKU: "sku",
			GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123,
		},
	}, 100, nil)

	data, total, err := uc.ListInstances(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.DeployRepo{
		{
			Path: "foo/bar", Status: "Pending", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123, SvcName: "svc", ClusterID: "cluster",
		},
	}, data)

}
