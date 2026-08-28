//go:build !ee && !saas

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

func TestCachedLicenseComponent_GetLicenseStatusInternal_DoesNotCacheExpired(t *testing.T) {
	ctx := context.Background()
	inner := mockcomponent.NewMockLicenseComponent(t)
	// CE does not use the licensed feature evaluator, so preserve its existing
	// behavior while verifying that expired results are not cached.
	inner.EXPECT().GetLicenseStatusInternal(ctx).Return(
		&types.LicenseStatusResp{ID: 1, DataBody: types.DataBody{ExpireTime: time.Now().Add(-time.Minute)}}, nil,
	).Twice()

	cached := NewCachedLicenseComponent(inner)

	first, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.ID)

	second, err := cached.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), second.ID)
}
