//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceResourceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.deployer.EXPECT().ListCluster(ctx).Return([]types.ClusterRes{
		{ClusterID: "c1"},
	}, nil)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, "c1").Return(
		[]database.SpaceResource{
			{ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`},
			{ID: 2, Name: "sr2", Resources: `{"memory": "1000"}`},
		}, nil,
	)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)

	data, err := sc.Index(ctx, "", types.FinetuneType, "user")
	require.Nil(t, err)
	require.Equal(t, []types.SpaceResource{
		{
			ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`,
			IsAvailable: false, Type: "gpu",
		},
	}, data)

}
