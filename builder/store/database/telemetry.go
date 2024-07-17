package database

import (
	"context"
	"time"
)

type Telemetry struct {
	UUID                 string                 `bun:"" json:"uuid"`
	RecordedAt           time.Time              `bun:"" json:"recorded_at"`
	Hostname             string                 `bun:"" json:"hostname,omitempty"`
	Version              string                 `bun:"" json:"version"`
	InstallationType     string                 `bun:"" json:"installation_type,omitempty"`
	ActiveUserCount      int                    `bun:"" json:"active_user_count"`
	Edition              string                 `bun:"" json:"edition,omitempty"`
	LicenseMD5           string                 `bun:"" json:"license_md5,omitempty"`
	LicenseID            int                    `bun:"" json:"license_id,omitempty"`
	HistoricalMaxUsers   int                    `bun:"" json:"historical_max_users,omitempty"`
	Licensee             interface{}            `bun:"type:jsonb" json:"licensee,omitempty"`
	LicenseUserCount     int                    `bun:"" json:"license_user_count,omitempty"`
	LicenseBillableUsers int                    `bun:"" json:"license_billable_users,omitempty"`
	LicenseStartsAt      time.Time              `bun:"" json:"license_starts_at,omitempty"`
	LicenseExpiresAt     time.Time              `bun:"" json:"license_expires_at,omitempty"`
	LicensePlan          string                 `bun:"" json:"license_plan,omitempty"`
	LicenseAddOns        map[string]interface{} `bun:"type:jsonb" json:"license_add_ons,omitempty"`
	Settings             interface{}            `bun:"type:jsonb" json:"settings,omitempty"`
	Counts               interface{}            `bun:"type:jsonb" json:"counts,omitempty"`
}

type TelemetryStore struct {
	db *DB
}

func NewTelemetryStore() *TelemetryStore {
	return &TelemetryStore{
		db: defaultDB,
	}
}

func (s *TelemetryStore) Save(ctx context.Context, telemetry *Telemetry) error {
	return assertAffectedOneRow(s.db.Core.NewInsert().Model(telemetry).Exec(ctx))
}
