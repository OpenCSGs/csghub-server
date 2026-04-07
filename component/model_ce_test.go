//go:build !ee && !saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestModelComponent_Deploy(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		RepositoryID: int64(123),
		Repository: &database.Repository{
			ID:   1,
			Path: "foo",
		},
	}, nil)
	mc.mocks.stores.DeployTaskMock().EXPECT().GetServerlessDeployByRepID(ctx, int64(1)).Return(
		nil, nil,
	)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		UUID:     "user-uuid",
	}, nil)
	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(11)).Return(
		&database.RuntimeFramework{}, nil,
	)
	mc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(123)).Return(
		&database.SpaceResource{
			ID:        123,
			Resources: `{"memory": "foo"}`,
			ClusterID: "cluster",
		}, nil,
	)

	// Model is under org "ns", current user "user" -> resolve billing UUID for namespace "ns"
	mc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "ns").Return("ns-billing-uuid", nil)
	mc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "ns", "cluster", int64(0), mock.Anything).Return(&types.CheckExclusiveResp{}, nil)

	mc.mocks.deployer.EXPECT().Deploy(ctx, mock.MatchedBy(func(dp types.DeployRequest) bool {
		return dp.DeployName == "dp" && dp.Path == "foo" && dp.ClusterID == "cluster" &&
			dp.RepoID == 1 && dp.SKU == "123" && dp.Type == types.ServerlessType &&
			dp.Task == "text-generation" && dp.UserUUID == "ns-billing-uuid" && dp.OwnerNamespace == "ns"
	})).Return(111, nil)

	id, err := mc.Deploy(ctx, types.DeployActReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
		DeployType:  types.ServerlessType,
	}, types.ModelRunReq{
		RuntimeFrameworkID: 11,
		ResourceID:         123,
		ClusterID:          "cluster",
		DeployName:         "dp",
		OwnerNamespace:     "ns",
	})
	require.Nil(t, err)
	require.Equal(t, int64(111), id)
}

func TestModelComponent_Deploy_OwnerNamespace_BillingUUIDError(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		RepositoryID: int64(123),
		Repository: &database.Repository{
			ID:   1,
			Path: "foo",
		},
	}, nil)
	mc.mocks.stores.DeployTaskMock().EXPECT().GetServerlessDeployByRepID(ctx, int64(1)).Return(nil, nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(11)).Return(&database.RuntimeFramework{}, nil)
	// Explicit OwnerNamespace: resolve billing UUID for "org1" fails (Deploy returns before FindByID)
	mc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "org1").Return("", errors.New("resolve billing error"))

	id, err := mc.Deploy(ctx, types.DeployActReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
		DeployType:  types.ServerlessType,
	}, types.ModelRunReq{
		RuntimeFrameworkID: 11,
		ResourceID:         123,
		ClusterID:          "cluster",
		DeployName:         "dp",
		OwnerNamespace:     "org1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to resolve billing UUID for namespace")
	require.Equal(t, int64(-1), id)
}
