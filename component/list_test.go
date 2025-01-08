package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestListComponent_ListModelsByPath(t *testing.T) {
	ctx := context.TODO()
	lc := initializeTestListComponent(ctx, t)

	lc.mocks.stores.ModelMock().EXPECT().ListByPath(ctx, []string{"foo"}).Return(
		[]database.Model{
			{Repository: &database.Repository{
				Name: "r1",
				Tags: []database.Tag{{Name: "t1"}},
			}},
		}, nil,
	)

	data, err := lc.ListModelsByPath(ctx, &types.ListByPathReq{Paths: []string{"foo"}})
	require.Nil(t, err)
	require.Equal(t, []*types.ModelResp{{Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}}}, data)
}

func TestListComponent_ListDatasetByPath(t *testing.T) {
	ctx := context.TODO()
	lc := initializeTestListComponent(ctx, t)

	lc.mocks.stores.DatasetMock().EXPECT().ListByPath(ctx, []string{"foo"}).Return(
		[]database.Dataset{
			{Repository: &database.Repository{
				Name: "r1",
				Tags: []database.Tag{{Name: "t1"}},
			}},
		}, nil,
	)

	data, err := lc.ListDatasetsByPath(ctx, &types.ListByPathReq{Paths: []string{"foo"}})
	require.Nil(t, err)
	require.Equal(t, []*types.ModelResp{{Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}}}, data)
}
