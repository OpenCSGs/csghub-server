package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceSdkComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceSdkComponent(ctx, t)

	sc.mocks.stores.SpaceSdkMock().EXPECT().Index(ctx).Return([]database.SpaceSdk{
		{ID: 1, Name: "s", Version: "1"},
	}, nil)

	data, err := sc.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, []types.SpaceSdk{{ID: 1, Name: "s", Version: "1"}}, data)
}

func TestSpaceSdkComponent_Update(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceSdkComponent(ctx, t)

	s := &database.SpaceSdk{ID: 1}
	sc.mocks.stores.SpaceSdkMock().EXPECT().FindByID(ctx, int64(1)).Return(s, nil)
	s2 := *s
	s2.Name = "n"
	s2.Version = "v1"
	sc.mocks.stores.SpaceSdkMock().EXPECT().Update(ctx, s2).Return(s, nil)

	data, err := sc.Update(ctx, &types.UpdateSpaceSdkReq{ID: 1, Name: "n", Version: "v1"})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceSdk{ID: 1, Name: "n", Version: "v1"}, data)
}

func TestSpaceSdkComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceSdkComponent(ctx, t)

	s := database.SpaceSdk{Name: "n", Version: "v1"}
	sc.mocks.stores.SpaceSdkMock().EXPECT().Create(ctx, s).Return(&s, nil)
	s.ID = 1

	data, err := sc.Create(ctx, &types.CreateSpaceSdkReq{Name: "n", Version: "v1"})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceSdk{ID: 1, Name: "n", Version: "v1"}, data)
}

func TestSpaceSdkComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceSdkComponent(ctx, t)

	s := &database.SpaceSdk{}
	sc.mocks.stores.SpaceSdkMock().EXPECT().FindByID(ctx, int64(1)).Return(s, nil)
	sc.mocks.stores.SpaceSdkMock().EXPECT().Delete(ctx, *s).Return(nil)

	err := sc.Delete(ctx, int64(1))
	require.Nil(t, err)
}
