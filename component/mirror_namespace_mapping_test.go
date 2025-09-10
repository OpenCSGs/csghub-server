package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorNamespaceMappingComponent_Create(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorNamespaceMappingComponent(ctx, t)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Create(ctx, &database.MirrorNamespaceMapping{
		SourceNamespace: "sn",
		TargetNamespace: "u",
	}).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)

	data, err := mc.Create(ctx, types.CreateMirrorNamespaceMappingReq{
		SourceNamespace: "sn",
		TargetNamespace: "u",
	})
	require.Nil(t, err)
	require.Equal(t, &database.MirrorNamespaceMapping{ID: 1}, data)
}

func TestMirrorNamespaceMappingComponent_Get(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorNamespaceMappingComponent(ctx, t)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Get(ctx, int64(1)).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)

	data, err := mc.Get(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, &database.MirrorNamespaceMapping{ID: 1}, data)
}

func TestMirrorNamespaceMappingComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorNamespaceMappingComponent(ctx, t)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Index(ctx).Return([]database.MirrorNamespaceMapping{
		{ID: 1},
	}, nil)

	data, err := mc.Index(ctx)
	require.Nil(t, err)
	require.Equal(t, []database.MirrorNamespaceMapping{
		{ID: 1},
	}, data)
}

func TestMirrorNamespaceMappingComponent_Update(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorNamespaceMappingComponent(ctx, t)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Update(ctx, &database.MirrorNamespaceMapping{
		ID:              1,
		SourceNamespace: "sn",
		TargetNamespace: "u",
	}).Return(database.MirrorNamespaceMapping{
		ID:              1,
		SourceNamespace: "sn",
		TargetNamespace: "u",
	}, nil)
	var (
		sn = "sn"
		u  = "u"
	)

	data, err := mc.Update(ctx, types.UpdateMirrorNamespaceMappingReq{
		ID:              1,
		SourceNamespace: &sn,
		TargetNamespace: &u,
	})
	require.Nil(t, err)
	require.Equal(t, &database.MirrorNamespaceMapping{
		ID:              1,
		SourceNamespace: "sn",
		TargetNamespace: "u",
	}, data)
}

func TestMirrorNamespaceMappingComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorNamespaceMappingComponent(ctx, t)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Get(ctx, int64(1)).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)
	mc.mocks.stores.MirrorNamespaceMappingMock().EXPECT().Delete(ctx, &database.MirrorNamespaceMapping{ID: 1}).Return(nil)

	err := mc.Delete(ctx, 1)
	require.Nil(t, err)
}
