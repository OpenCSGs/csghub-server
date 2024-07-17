package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types/telemetry"
)

type TelemetryComponent struct {
	// Add telemetry related fields and methods here
	ts *database.TelemetryStore
}

func NewTelemetryComponent() (*TelemetryComponent, error) {
	ts := database.NewTelemetryStore()
	return &TelemetryComponent{ts: ts}, nil
}

func (tc *TelemetryComponent) SaveUsageData(ctx context.Context, usage telemetry.Usage) error {
	t := database.Telemetry{
		UUID:                 usage.UUID,
		RecordedAt:           usage.RecordedAt,
		Hostname:             usage.Hostname,
		Version:              usage.Version,
		InstallationType:     usage.InstallationType,
		ActiveUserCount:      usage.ActiveUserCount,
		Edition:              usage.Edition,
		LicenseMD5:           usage.LicenseMD5,
		LicenseID:            usage.LicenseID,
		HistoricalMaxUsers:   usage.HistoricalMaxUsers,
		Licensee:             usage.Licensee,
		LicenseUserCount:     usage.LicenseUserCount,
		LicenseBillableUsers: usage.LicenseBillableUsers,
		LicenseStartsAt:      usage.LicenseStartsAt,
		LicenseExpiresAt:     usage.LicenseExpiresAt,
		LicensePlan:          usage.LicensePlan,
		LicenseAddOns:        usage.LicenseAddOns,
		Settings:             usage.Settings,
		Counts:               usage.Counts,
	}
	err := tc.ts.Save(ctx, &t)
	if err != nil {
		return fmt.Errorf("failed to save telemetry data to db: %w", err)
	}

	return nil
}
