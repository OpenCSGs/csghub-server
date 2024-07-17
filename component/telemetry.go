package component

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/telemetry"
)

type TelemetryComponent struct {
	// Add telemetry related fields and methods here
	ts *database.TelemetryStore
	us *database.UserStore
	rs *database.RepoStore
}

func NewTelemetryComponent() (*TelemetryComponent, error) {
	ts := database.NewTelemetryStore()
	us := database.NewUserStore()
	rs := database.NewRepoStore()
	return &TelemetryComponent{ts: ts, us: us, rs: rs}, nil
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

func (tc *TelemetryComponent) GenUsageData(ctx context.Context) (telemetry.Usage, error) {
	var usage telemetry.Usage

	uuid, err := uuid.NewV7()
	if err != nil {
		return usage, fmt.Errorf("failed to generate uuid: %w", err)
	}
	usage.UUID = uuid.String()
	usage.RecordedAt = time.Now()
	usage.Version = ""
	usage.InstallationType = ""
	usage.ActiveUserCount, err = tc.getUserCnt(ctx)
	if err != nil {
		return usage, fmt.Errorf("failed to get user count: %w", err)
	}
	usage.Edition = ""
	usage.HistoricalMaxUsers = 0
	//TODO:load license data
	// usage.LicenseMD5 = ""
	// usage.LicenseID = 0
	// usage.Licensee = telemetry.Licensee{}
	// usage.LicenseUserCount = 0
	// usage.LicenseBillableUsers = 0
	// usage.LicenseStartsAt =
	// usage.LicenseExpiresAt = ""
	// usage.LicensePlan = ""
	// usage.LicenseAddOns = ""
	usage.Settings = telemetry.Settings{
		// LdapEncryptedSecretsEnabled:         false,
		// SmtpEncryptedSecretsEnabled:         false,
		// OperatingSystem:                     "",
		// GitalyApdex:                         0,
		// CollectedDataCategories:             []string{},
		// ServicePingFeaturesEnabled:          false,
		// SnowplowEnabled:                     false,
		// SnowplowConfiguredToGitlabCollector: false,
	}
	usage.Counts, err = tc.getCounts(ctx)
	if err != nil {
		return usage, fmt.Errorf("failed to get counts: %w", err)
	}
	return usage, nil
}

func (tc *TelemetryComponent) getUserCnt(ctx context.Context) (int, error) {
	return tc.us.GetActiveUserCount(ctx)
}

func (tc *TelemetryComponent) getCounts(ctx context.Context) (telemetry.Counts, error) {
	var counts telemetry.Counts
	modelCnt, err := tc.rs.CountByRepoType(ctx, types.ModelRepo)
	if err != nil {
		return counts, fmt.Errorf("failed to get model repo count: %w", err)
	}

	dsCnt, err := tc.rs.CountByRepoType(ctx, types.DatasetRepo)
	if err != nil {
		return counts, fmt.Errorf("failed to get dataset repo count: %w", err)
	}

	codeCnt, err := tc.rs.CountByRepoType(ctx, types.CodeRepo)
	if err != nil {
		return counts, fmt.Errorf("failed to get code repo count: %w", err)
	}

	spaceCnt, err := tc.rs.CountByRepoType(ctx, types.SpaceRepo)
	if err != nil {
		return counts, fmt.Errorf("failed to get space repo count: %w", err)
	}

	counts.Codes = codeCnt
	counts.Datasets = dsCnt
	counts.Models = modelCnt
	counts.Spaces = spaceCnt
	counts.TotalRepos = modelCnt + dsCnt + codeCnt + spaceCnt
	return counts, nil
}
