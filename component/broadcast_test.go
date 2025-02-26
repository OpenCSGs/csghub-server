package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestBroadcastComponent_GetBroadcast(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestBroadcastComponent(ctx, t)

	broadcast := database.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"}

	cc.mocks.stores.BroadcastMock().EXPECT().Get(ctx, 1).Return(
		&broadcast, nil,
	)
	data, err := cc.GetBroadcast(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, &types.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"}, data)
}

func TestBroadcastComponent_AllBroadcasts(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestBroadcastComponent(ctx, t)

	broadcasts := []database.Broadcast{
		{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"},
		{ID: 2, Content: "test2", BcType: "message", Theme: "dark", Status: "inactive"},
	}

	cc.mocks.stores.BroadcastMock().EXPECT().FindAll(ctx).Return(
		broadcasts, nil,
	)
	data, err := cc.AllBroadcasts(ctx)
	require.Nil(t, err)

	resBroadcasts := []types.Broadcast{
		{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"},
		{ID: 2, Content: "test2", BcType: "message", Theme: "dark", Status: "inactive"},
	}

	require.Equal(t, resBroadcasts, data)
}

func TestBroadcastComponent_NewBroadcast(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestBroadcastComponent(ctx, t)

	dbBroadcast := database.Broadcast{Content: "test", BcType: "banner", Theme: "light", Status: "active"}
	broadcast := database.Broadcast{Content: "test", BcType: "banner", Theme: "light", Status: "active"}

	cc.mocks.stores.BroadcastMock().EXPECT().Save(ctx, dbBroadcast).Return(nil)

	err := cc.NewBroadcast(ctx, types.Broadcast(broadcast))
	require.Nil(t, err)
}

func TestBroadcastComponent_UpdateBroadcast(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestBroadcastComponent(ctx, t)

	dbBroadcast := database.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"}
	broadcast := types.Broadcast{ID: 1, Content: "test", BcType: "banner", Theme: "light", Status: "active"}

	cc.mocks.stores.BroadcastMock().EXPECT().Get(ctx, int64(1)).Return(&dbBroadcast, nil)
	cc.mocks.stores.BroadcastMock().EXPECT().Update(ctx, dbBroadcast).Return(&dbBroadcast, nil)

	data, _ := cc.UpdateBroadcast(ctx, broadcast)

	require.Equal(t, &broadcast, data)
}
