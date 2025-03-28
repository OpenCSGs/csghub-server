package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSyncClientSettingComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSyncClientSettingComponent(ctx, t)

	sc.mocks.stores.SyncClientSettingMock().EXPECT().SyncClientSettingExists(ctx).Return(true, nil)
	sc.mocks.stores.SyncClientSettingMock().EXPECT().DeleteAll(ctx).Return(nil)
	sc.mocks.stores.SyncClientSettingMock().EXPECT().Create(ctx, &database.SyncClientSetting{
		Token:           "t",
		ConcurrentCount: 1,
		MaxBandwidth:    5,
	}).Return(&database.SyncClientSetting{}, nil)

	data, err := sc.Create(ctx, types.CreateSyncClientSettingReq{
		Token:           "t",
		ConcurrentCount: 1,
		MaxBandwidth:    5,
	})
	require.Nil(t, err)
	require.Equal(t, &database.SyncClientSetting{}, data)

}

func TestSyncClientSettingComponent_Show(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSyncClientSettingComponent(ctx, t)

	sc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	sc.mocks.stores.SyncClientSettingMock().EXPECT().First(ctx).Return(&database.SyncClientSetting{}, nil)

	data, err := sc.Show(ctx, "user")
	require.Nil(t, err)
	require.Equal(t, &database.SyncClientSetting{}, data)
}
