package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LicenseExtra defines the structured content that can be stored in
// database.License.Extra as JSON. It holds runtime feature flags and
// numeric limits that drive the FeatureGate abstraction.
type LicenseExtra struct {
	// Features maps feature flag keys to their enabled state.
	// Example: {"feature.multi_cluster": true, "feature.audit_log": false}
	Features map[string]bool `json:"features"`

	// Limits maps limit keys to their maximum allowed value.
	// Example: {"quota.max_models": 100, "quota.max_repos": 3}
	Limits map[string]int `json:"limits"`
}

// ParseLicenseExtra parses the JSON string stored in License.Extra.
// If extra is empty, it returns a zero-value LicenseExtra (all flags/limits
// missing) so the caller can fall back to defaults.
func ParseLicenseExtra(extra string) (LicenseExtra, error) {
	le := LicenseExtra{
		Features: make(map[string]bool),
		Limits:   make(map[string]int),
	}
	if extra == "" {
		return le, nil
	}
	var sections struct {
		Features map[string]json.RawMessage `json:"features"`
		Limits   map[string]json.RawMessage `json:"limits"`
	}
	if err := json.Unmarshal([]byte(extra), &sections); err != nil {
		return le, err
	}

	known := make(map[string]FeatureType)
	for _, def := range FeatureDefinitions() {
		known[def.Key] = def.Type
	}

	for key, value := range sections.Features {
		switch known[key] {
		case FeatureTypeBoolean:
			var parsed bool
			if err := json.Unmarshal(value, &parsed); err != nil {
				return le, fmt.Errorf("feature flag %q in license extra is not a boolean feature flag: %w", key, err)
			}
			le.Features[key] = parsed
		case FeatureTypeInt:
			return le, fmt.Errorf("feature flag %q in license extra is not a boolean feature flag", key)
		}
	}

	for key, value := range sections.Limits {
		switch known[key] {
		case FeatureTypeInt:
			var parsed int
			if err := json.Unmarshal(value, &parsed); err != nil {
				return le, fmt.Errorf("limit %q in license extra is not an integer limit: %w", key, err)
			}
			le.Limits[key] = parsed
		case FeatureTypeBoolean:
			return le, fmt.Errorf("limit %q in license extra is not an integer limit", key)
		}
	}
	return le, nil
}

// CSGLite owns the "lite." product segment of the flag namespace. Its
// catalog (internal/license/features.go in github.com/opencsgs/csglite) is the
// source of truth for those keys, so the issuer accepts any well-formed key in
// that segment without requiring it to be registered here first: the type is
// fixed by the prefix ("feature.lite.*" is boolean, "quota.lite.*" is an
// integer limit). Registered lite keys additionally get names in the
// management UI.
const (
	csgliteFeaturePrefix = "feature.lite."
	csgliteQuotaPrefix   = "quota.lite."
)

func isCSGLiteFeatureKey(key string) bool {
	return len(key) > len(csgliteFeaturePrefix) && strings.HasPrefix(key, csgliteFeaturePrefix)
}

func isCSGLiteQuotaKey(key string) bool {
	return len(key) > len(csgliteQuotaPrefix) && strings.HasPrefix(key, csgliteQuotaPrefix)
}

// ValidateLicenseExtraForIssue validates Extra before a license is signed.
// Issuers must reject unknown fields and keys so a typo cannot produce a
// signed license whose effective entitlements differ from the issuer input.
// Keys in CSGLite's "lite." segment are exempt from the registration check
// but are still type-checked by prefix.
func ValidateLicenseExtraForIssue(extra string) error {
	_, err := validateLicenseExtra(extra, true)
	return err
}

// ValidateLicenseExtraForImport validates Extra from a signed license. It
// rejects malformed JSON and known keys with the wrong type, but tolerates
// unknown fields and keys so older consumers can import licenses from newer
// issuers and existing legacy Extra payloads remain compatible.
func ValidateLicenseExtraForImport(extra string) ([]string, error) {
	return validateLicenseExtra(extra, false)
}

// ValidateLicenseExtra retains the permissive consumer behavior for callers
// that do not participate in license issuance.
func ValidateLicenseExtra(extra string) ([]string, error) {
	return ValidateLicenseExtraForImport(extra)
}

func validateLicenseExtra(extra string, strict bool) ([]string, error) {
	if extra == "" {
		return nil, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(extra), &raw); err != nil {
		return nil, fmt.Errorf("invalid license extra JSON: %w", err)
	}

	known := make(map[string]FeatureType)
	for _, def := range FeatureDefinitions() {
		known[def.Key] = def.Type
	}

	var warnings []string
	for field := range raw {
		if field != "features" && field != "limits" {
			if strict {
				return nil, fmt.Errorf("unknown top-level field %q in license extra", field)
			}
			warnings = append(warnings, fmt.Sprintf("unknown top-level field %q in license extra", field))
		}
	}

	var sections struct {
		Features map[string]json.RawMessage `json:"features"`
		Limits   map[string]json.RawMessage `json:"limits"`
	}
	if err := json.Unmarshal([]byte(extra), &sections); err != nil {
		return nil, fmt.Errorf("invalid license extra JSON: %w", err)
	}

	for key, value := range sections.Features {
		switch known[key] {
		case FeatureTypeBoolean:
			var parsed bool
			if err := json.Unmarshal(value, &parsed); err != nil {
				return nil, fmt.Errorf("feature flag %q in license extra is not a boolean feature flag: %w", key, err)
			}
		case FeatureTypeInt:
			return nil, fmt.Errorf("feature flag %q in license extra is not a boolean feature flag", key)
		default:
			if isCSGLiteFeatureKey(key) {
				var parsed bool
				if err := json.Unmarshal(value, &parsed); err != nil {
					return nil, fmt.Errorf("CSGLite feature flag %q in license extra is not a boolean: %w", key, err)
				}
				continue
			}
			if isCSGLiteQuotaKey(key) {
				return nil, fmt.Errorf("CSGLite limit %q must be listed under \"limits\", not \"features\"", key)
			}
			if strict {
				return nil, fmt.Errorf("unknown feature flag %q in license extra", key)
			}
			warnings = append(warnings, fmt.Sprintf("unknown feature flag %q in license extra", key))
		}
	}

	for key, value := range sections.Limits {
		switch known[key] {
		case FeatureTypeInt:
			var parsed int
			if err := json.Unmarshal(value, &parsed); err != nil {
				return nil, fmt.Errorf("limit %q in license extra is not an integer limit: %w", key, err)
			}
		case FeatureTypeBoolean:
			return nil, fmt.Errorf("limit %q in license extra is not an integer limit", key)
		default:
			if isCSGLiteQuotaKey(key) {
				var parsed int
				if err := json.Unmarshal(value, &parsed); err != nil {
					return nil, fmt.Errorf("CSGLite limit %q in license extra is not an integer: %w", key, err)
				}
				continue
			}
			if isCSGLiteFeatureKey(key) {
				return nil, fmt.Errorf("CSGLite feature flag %q must be listed under \"features\", not \"limits\"", key)
			}
			if strict {
				return nil, fmt.Errorf("unknown limit %q in license extra", key)
			}
			warnings = append(warnings, fmt.Sprintf("unknown limit %q in license extra", key))
		}
	}

	sort.Strings(warnings)
	return warnings, nil
}
