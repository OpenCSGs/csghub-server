//go:build ee || (saas && !license_issuer)

package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/feature"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// newLicenseStatusReader builds the cached entitlement reader that backs the
// license feature provider for license-consuming editions.
func newLicenseStatusReader(config *config.Config) (feature.LicenseStatusReader, error) {
	reader := &licenseEntitlementReader{licenseStore: database.NewLicenseStore()}
	return NewSharedCachedLicenseStatusReaderWithKey(config, reader), nil
}

type licenseEntitlementReader struct {
	licenseStore database.LicenseStore
}

func (r *licenseEntitlementReader) GetLicenseStatusInternal(ctx context.Context) (*types.LicenseStatusResp, error) {
	license, err := r.licenseStore.GetLatestActive(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active license entitlements error: %w", err)
	}
	return licenseStatusFromLicense(license), nil
}

func (r *licenseEntitlementReader) GetNextLicenseStart(ctx context.Context) (*upcomingLicenseStart, error) {
	license, err := r.licenseStore.GetNextUpcoming(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get next upcoming license error: %w", err)
	}
	return &upcomingLicenseStart{StartTime: license.StartTime}, nil
}
