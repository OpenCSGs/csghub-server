package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceResourceComponent_Update(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{}, nil,
	)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Update(ctx, database.SpaceResource{
		Name:      "n",
		Resources: "r",
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: "r"}, nil)

	data, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: "r",
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: "r",
	}, data)
}

func TestSpaceResourceComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Create(ctx, database.SpaceResource{
		Name:      "n",
		Resources: "r",
		ClusterID: "c",
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: "r"}, nil)

	data, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: "r",
		ClusterID: "c",
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: "r",
	}, data)
}

func TestSpaceResourceComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{}, nil,
	)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Delete(ctx, database.SpaceResource{}).Return(nil)

	err := sc.Delete(ctx, 1)
	require.Nil(t, err)
}

func TestSpaceResourceComponent_ListHardwareTypes(t *testing.T) {
	t.Run("list hardware types", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)

		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAllResourceTypes(ctx, "c1").Return(
			[]string{"type1", "type2"}, nil,
		)

		types, err := sc.ListHardwareTypes(ctx, "c1")
		require.Nil(t, err)
		require.Equal(t, []string{"type1", "type2"}, types)
	})
	t.Run("error listing hardware types", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)
		assertError := errors.New("database error")
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAllResourceTypes(ctx, "c1").Return(
			nil, assertError,
		)

		types, err := sc.ListHardwareTypes(ctx, "c1")
		require.NotNil(t, err)
		require.Nil(t, types)
	})
}
