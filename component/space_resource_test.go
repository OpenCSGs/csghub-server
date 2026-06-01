package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const validResourcesJSON = `{"cpu":{"num":"2","type":"intel"},"memory":"8G"}`

func TestValidateResources(t *testing.T) {
	t.Run("empty resources", func(t *testing.T) {
		err := validateResources("")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("blank resources", func(t *testing.T) {
		err := validateResources("   ")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("invalid json", func(t *testing.T) {
		err := validateResources("not-json")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("valid hardware json", func(t *testing.T) {
		err := validateResources(validResourcesJSON)
		require.Nil(t, err)
	})
	t.Run("empty json object", func(t *testing.T) {
		err := validateResources("{}")
		require.Nil(t, err)
	})
}

func TestSpaceResourceComponent_Update(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{}, nil,
	)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Update(ctx, database.SpaceResource{
		Name:      "n",
		Resources: validResourcesJSON,
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: validResourcesJSON}, nil)

	data, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
	}, data)
}

func TestSpaceResourceComponent_Update_InvalidResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: "invalid",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestSpaceResourceComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Create(ctx, database.SpaceResource{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: validResourcesJSON}, nil)

	data, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
	}, data)
}

func TestSpaceResourceComponent_Create_EmptyResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: "",
		ClusterID: "c",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestSpaceResourceComponent_Create_InvalidResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: "not-json",
		ClusterID: "c",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
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

func TestSpaceResourceComponent_ListAll(t *testing.T) {
	t.Run("list all resources", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)

		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAll(ctx).Return(
			[]database.SpaceResource{
				{ID: 1, Name: "resource1", ClusterID: "c1", Resources: "{}"},
				{ID: 2, Name: "resource2", ClusterID: "c2", Resources: "{}"},
			}, nil,
		)

		resources, err := sc.ListAll(ctx)
		require.Nil(t, err)
		require.Equal(t, []types.SpaceResource{
			{ID: 1, Name: "resource1", ClusterID: "c1", Resources: "{}"},
			{ID: 2, Name: "resource2", ClusterID: "c2", Resources: "{}"},
		}, resources)
	})
	t.Run("error listing all resources", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)
		assertError := errors.New("database error")
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAll(ctx).Return(
			nil, assertError,
		)

		resources, err := sc.ListAll(ctx)
		require.NotNil(t, err)
		require.Nil(t, resources)
	})
}
