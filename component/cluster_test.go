package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestClusterComponent_Index(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	cc.mocks.deployer.EXPECT().ListCluster(ctx).Return(nil, nil)

	data, err := cc.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, []types.ClusterRes([]types.ClusterRes(nil)), data)
}

func TestClusterComponent_GetClusterById(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	cc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(nil, nil)

	data, err := cc.GetClusterById(ctx, "c1")
	require.Nil(t, err)
	require.Equal(t, (*types.ClusterRes)(nil), data)
}

func TestClusterComponent_Update(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestClusterComponent(ctx, t)

	cc.mocks.deployer.EXPECT().UpdateCluster(ctx, types.ClusterRequest{}).Return(nil, nil)

	data, err := cc.Update(ctx, types.ClusterRequest{})
	require.Nil(t, err)
	require.Equal(t, (*types.UpdateClusterResponse)(nil), data)
}
