package telemetry

import "time"

type Licensee struct {
	Name    string `json:"Name"`
	Company string `json:"Company"`
	Email   string `json:"Email"`
}

// Settings contains the settings for the On-Premise instance.
type Settings struct {
	LdapEncryptedSecretsEnabled         bool     `json:"ldap_encrypted_secrets_enabled"`
	SmtpEncryptedSecretsEnabled         bool     `json:"smtp_encrypted_secrets_enabled"`
	OperatingSystem                     string   `json:"operating_system"`
	GitalyApdex                         float64  `json:"gitaly_apdex"`
	CollectedDataCategories             []string `json:"collected_data_categories"`
	ServicePingFeaturesEnabled          bool     `json:"service_ping_features_enabled"`
	SnowplowEnabled                     bool     `json:"snowplow_enabled"`
	SnowplowConfiguredToGitlabCollector bool     `json:"snowplow_configured_to_gitlab_collector"`
}

type Counts struct {
	TotalRepos int `json:"total_repos"`
	Models     int `json:"models"`
	Datasets   int `json:"datasets"`
	Codes      int `json:"codes"`
	Spaces     int `json:"spaces"`
}

type Usage struct {
	RecordedAt           time.Time              `json:"recorded_at"`
	UUID                 string                 `json:"uuid"`
	Hostname             string                 `json:"hostname,omitempty"`
	Version              string                 `json:"version"`
	InstallationType     string                 `json:"installation_type,omitempty"`
	ActiveUserCount      int                    `json:"active_user_count"`
	Edition              string                 `json:"edition,omitempty"`
	LicenseMD5           string                 `json:"license_md5,omitempty"`
	LicenseID            int                    `json:"license_id,omitempty"`
	HistoricalMaxUsers   int                    `json:"historical_max_users,omitempty"`
	Licensee             Licensee               `json:"licensee,omitempty"`
	LicenseUserCount     int                    `json:"license_user_count,omitempty"`
	LicenseBillableUsers int                    `json:"license_billable_users,omitempty"`
	LicenseStartsAt      time.Time              `json:"license_starts_at,omitempty"`
	LicenseExpiresAt     time.Time              `json:"license_expires_at,omitempty"`
	LicensePlan          string                 `json:"license_plan,omitempty"`
	LicenseAddOns        map[string]interface{} `json:"license_add_ons,omitempty"`
	Settings             Settings               `json:"settings,omitempty"`
	Counts               Counts                 `json:"counts,omitempty"`
}
