package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

func TestCachedLicenseComponent_GetLicenseStatusInternal_CachesResult(t *testing.T) {
	ctx := context.Background()
	inner := mockcomponent.NewMockLicenseComponent(t)
	inner.EXPECT().GetLicenseStatusInternal(ctx).Return(
		&types.LicenseStatusResp{ID: 1, DataBody: types.DataBody{ExpireTime: time.Now().Add(time.Hour)}}, nil,
	).Once()

	cached := NewCachedLicenseComponent(inner)

	first, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.ID)

	second, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), second.ID)
}

func TestCachedLicenseComponent_ImportLicense_InvalidatesCache(t *testing.T) {
	ctx := context.Background()
	inner := mockcomponent.NewMockLicenseComponent(t)
	inner.EXPECT().ImportLicense(ctx, types.ImportLicenseReq{Data: "data"}).Return(nil).Once()
	inner.EXPECT().GetLicenseStatusInternal(ctx).Return(
		&types.LicenseStatusResp{ID: 2, DataBody: types.DataBody{ExpireTime: time.Now().Add(time.Hour)}}, nil,
	).Once()

	cached := NewCachedLicenseComponent(inner)

	require.NoError(t, cached.ImportLicense(ctx, types.ImportLicenseReq{Data: "data"}))

	status, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), status.ID)
}

func TestCachedLicenseComponent_CreateLicense_InvalidatesCache(t *testing.T) {
	ctx := context.Background()
	inner := mockcomponent.NewMockLicenseComponent(t)
	inner.EXPECT().GetLicenseStatusInternal(ctx).Return(
		&types.LicenseStatusResp{ID: 1, DataBody: types.DataBody{ExpireTime: time.Now().Add(time.Hour)}}, nil,
	).Once()
	inner.EXPECT().CreateLicense(ctx, &types.CreateLicenseReq{}).Return("license", nil).Once()
	inner.EXPECT().GetLicenseStatusInternal(ctx).Return(
		&types.LicenseStatusResp{ID: 2, DataBody: types.DataBody{ExpireTime: time.Now().Add(time.Hour)}}, nil,
	).Once()

	cached := NewCachedLicenseComponent(inner)

	status, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), status.ID)

	license, err := cached.CreateLicense(ctx, &types.CreateLicenseReq{})
	require.NoError(t, err)
	assert.Equal(t, "license", license)

	status, err = cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), status.ID)
}

func TestCachedLicenseComponent_DeleteLicenseByID_InvalidatesCache(t *testing.T) {
	ctx := context.Background()
	inner := mockcomponent.NewMockLicenseComponent(t)
	inner.EXPECT().DeleteLicenseByID(ctx, int64(1), "admin").Return(nil).Once()

	cached := NewCachedLicenseComponent(inner)
	require.NoError(t, cached.DeleteLicenseByID(ctx, 1, "admin"))
}
