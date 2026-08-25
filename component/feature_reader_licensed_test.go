//go:build ee || (saas && !license_issuer)

package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestLicenseEntitlementReader_GetLicenseStatusInternal(t *testing.T) {
	ctx := context.Background()
	expireTime := time.Now().Add(time.Hour)
	store := mockdatabase.NewMockLicenseStore(t)
	store.EXPECT().GetLatestActive(ctx).Return(&database.License{
		ID:         12,
		Key:        "k1",
		Company:    "c1",
		Email:      "c1@example.com",
		Product:    "p1",
		Edition:    "e1",
		Version:    "v1",
		MaxUser:    5,
		ExpireTime: expireTime,
		Extra:      `{"features":{"feature.audit_log":false}}`,
	}, nil).Once()

	reader := &licenseEntitlementReader{licenseStore: store}
	status, err := reader.GetLicenseStatusInternal(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, int64(12), status.ID)
	assert.Equal(t, 5, status.MaxUser)
	assert.Equal(t, 0, status.Users)
	assert.Equal(t, `{"features":{"feature.audit_log":false}}`, status.Extra)
}
