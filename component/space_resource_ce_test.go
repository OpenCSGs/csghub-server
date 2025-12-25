//go:build !ee && !saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceResourceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "c1"}, 50, 1).Return(
		[]database.SpaceResource{
			{ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`},
			{ID: 2, Name: "sr2", Resources: `{"memory": "1000"}`},
		}, 0, nil,
	)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)
	req := &types.SpaceResourceIndexReq{
		ClusterID:   "c1",
		DeployType:  types.FinetuneType,
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			PageSize: 50,
			Page:     1,
		},
	}
	data, _, err := sc.Index(ctx, req)
	require.Nil(t, err)
	require.Equal(t, []types.SpaceResource{
		{
			ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`,
			IsAvailable: false, Type: "gpu",
		},
	}, data)

}

func TestSpaceResourceComponent_Index_With_Status_Filter(t *testing.T) {
	ctx := context.TODO()
	t.Run("found running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		clusters := []types.ClusterRes{
			{ClusterID: "cluster1", Status: types.ClusterStatusUnavailable},
			{ClusterID: "cluster2", Status: types.ClusterStatusRunning},
		}
		sc.mocks.deployer.EXPECT().ListCluster(ctx).Return(clusters, nil)
		sc.mocks.deployer.EXPECT().CheckHeartbeatTimeout(ctx, "cluster1").Once().Return(true, nil)
		sc.mocks.deployer.EXPECT().CheckHeartbeatTimeout(ctx, "cluster2").Once().Return(false, nil)
		sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "cluster2"}, 50, 1).Return([]database.SpaceResource{}, 0, nil)
		sc.mocks.deployer.EXPECT().GetClusterById(ctx, "cluster2").Return(&types.ClusterRes{}, nil)
		req := &types.SpaceResourceIndexReq{
			CurrentUser: "user1",
			PageOpts: types.PageOpts{
				PageSize: 50,
				Page:     1,
			},
		}
		_, _, err := sc.Index(ctx, req)
		require.Nil(t, err)
	})

	t.Run("no running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		clusters := []types.ClusterRes{
			{ClusterID: "cluster1", Status: types.ClusterStatusUnavailable},
			{ClusterID: "cluster2", Status: types.ClusterStatusUnavailable},
		}
		sc.mocks.deployer.EXPECT().ListCluster(ctx).Return(clusters, nil)
		sc.mocks.deployer.EXPECT().CheckHeartbeatTimeout(ctx, "cluster1").Once().Return(true, nil)
		sc.mocks.deployer.EXPECT().CheckHeartbeatTimeout(ctx, "cluster2").Once().Return(true, nil)
		req := &types.SpaceResourceIndexReq{
			CurrentUser: "user1",
			PageOpts: types.PageOpts{
				PageSize: 50,
				Page:     1,
			},
		}
		_, _, err := sc.Index(ctx, req)
		require.Error(t, err)
		require.Equal(t, "can not find any running clusters", err.Error())
	})

	t.Run("no clusters", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		clusters := []types.ClusterRes{}
		sc.mocks.deployer.EXPECT().ListCluster(ctx).Return(clusters, nil)
		req := &types.SpaceResourceIndexReq{
			CurrentUser: "user1",
			PageOpts: types.PageOpts{
				PageSize: 50,
				Page:     1,
			},
		}
		_, _, err := sc.Index(ctx, req)
		require.Error(t, err)
		require.Equal(t, "can not find any running clusters", err.Error())
	})

	t.Run("deployer error", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		sc.mocks.deployer.EXPECT().ListCluster(ctx).Return(nil, errors.New("deployer error"))
		req := &types.SpaceResourceIndexReq{
			CurrentUser: "user1",
			PageOpts: types.PageOpts{
				PageSize: 50,
				Page:     1,
			},
		}
		_, _, err := sc.Index(ctx, req)
		require.Error(t, err)
		require.Equal(t, "deployer error", err.Error())
	})
}
