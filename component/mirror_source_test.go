package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorSourceComponent_Create(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorSourceComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Create(ctx, &database.MirrorSource{
		SourceName: "sn",
		InfoAPIUrl: "url",
	}).Return(&database.MirrorSource{ID: 1}, nil)

	data, err := mc.Create(ctx, types.CreateMirrorSourceReq{
		SourceName:  "sn",
		InfoAPiUrl:  "url",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, &database.MirrorSource{ID: 1}, data)
}

func TestMirrorSourceComponent_Get(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorSourceComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(1)).Return(&database.MirrorSource{ID: 1}, nil)

	data, err := mc.Get(ctx, 1, "user")
	require.Nil(t, err)
	require.Equal(t, &database.MirrorSource{ID: 1}, data)
}

func TestMirrorSourceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorSourceComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Index(ctx).Return([]database.MirrorSource{
		{ID: 1},
	}, nil)

	data, err := mc.Index(ctx, "user")
	require.Nil(t, err)
	require.Equal(t, []database.MirrorSource{
		{ID: 1},
	}, data)
}

func TestMirrorSourceComponent_Update(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorSourceComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Update(ctx, &database.MirrorSource{
		ID:         1,
		SourceName: "sn",
		InfoAPIUrl: "url",
	}).Return(nil)

	data, err := mc.Update(ctx, types.UpdateMirrorSourceReq{
		ID:          1,
		SourceName:  "sn",
		InfoAPiUrl:  "url",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, &database.MirrorSource{
		ID:         1,
		SourceName: "sn",
		InfoAPIUrl: "url",
	}, data)
}

func TestMirrorSourceComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorSourceComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Get(ctx, int64(1)).Return(&database.MirrorSource{ID: 1}, nil)
	mc.mocks.stores.MirrorSourceMock().EXPECT().Delete(ctx, &database.MirrorSource{ID: 1}).Return(nil)

	err := mc.Delete(ctx, 1, "user")
	require.Nil(t, err)
}
