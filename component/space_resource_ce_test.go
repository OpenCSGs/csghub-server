//go:build !ee && !saas

package component

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceResourceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "c1"}, math.MaxInt, 1).Return(
		[]database.SpaceResource{
			{ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`},
			{ID: 2, Name: "sr2", Resources: `{"memory": "1000"}`},
		}, 0, nil,
	)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)
	req := &types.SpaceResourceIndexReq{
		ClusterIDs:  []string{"c1"},
		DeployType:  types.FinetuneType,
		CurrentUser: "user",
		Per:         50,
		Page:        1,
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

func TestSpaceResourceComponent_Index_No_Cluster(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "", ResourceType: "", HardwareType: ""}, math.MaxInt, 1).
		Return([]database.SpaceResource{}, 0, nil)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "").Return(nil, nil)

	req := &types.SpaceResourceIndexReq{
		ClusterIDs:  []string{""},
		DeployType:  types.FinetuneType,
		CurrentUser: "user",
		Per:         50,
		Page:        1,
	}

	data, total, err := sc.Index(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Nil(t, data)
}

func TestSpaceResourceComponent_Index_With_Status_Filter(t *testing.T) {
	ctx := context.TODO()
	t.Run("found running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "cluster2"}, math.MaxInt, 1).
			Return([]database.SpaceResource{}, 20, nil)
		sc.mocks.deployer.EXPECT().GetClusterById(ctx, "cluster2").Return(&types.ClusterRes{}, nil)
		req := &types.SpaceResourceIndexReq{
			ClusterIDs:  []string{"cluster2"},
			DeployType:  types.FinetuneType,
			CurrentUser: "user1",
			Per:         50,
			Page:        1,
		}
		_, _, err := sc.Index(ctx, req)
		require.Nil(t, err)
	})

	t.Run("no running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		req := &types.SpaceResourceIndexReq{
			DeployType:  types.FinetuneType,
			CurrentUser: "user1",
			Per:         50,
			Page:        1,
		}
		_, total, err := sc.Index(ctx, req)
		require.NoError(t, err)
		require.Equal(t, 0, total)
	})
}
