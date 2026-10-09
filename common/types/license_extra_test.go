package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLicenseExtra_Valid(t *testing.T) {
	extra := `{"features":{"feature.audit_log":true}}`
	warnings, err := ValidateLicenseExtra(extra)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateLicenseExtra_Empty(t *testing.T) {
	warnings, err := ValidateLicenseExtra("")
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateLicenseExtra_InvalidJSON(t *testing.T) {
	_, err := ValidateLicenseExtra("{not-json")
	require.Error(t, err)
}

func TestValidateLicenseExtra_UnknownFeatureKeyIsTolerated(t *testing.T) {
	extra := `{"features":{"feature.mutli_cluster":false}}`
	warnings, err := ValidateLicenseExtra(extra)
	require.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "feature.mutli_cluster")
}

func TestValidateLicenseExtra_UnknownLimitKeyIsTolerated(t *testing.T) {
	extra := `{"limits":{"quota.max_cluster":3}}`
	warnings, err := ValidateLicenseExtra(extra)
	require.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "quota.max_cluster")
}

func TestValidateLicenseExtra_MixedKnownAndUnknown(t *testing.T) {
	extra := `{"features":{"feature.audit_log":true,"feature.future":true},"limits":{"quota.max_models":50}}`
	warnings, err := ValidateLicenseExtra(extra)
	require.NoError(t, err)
	assert.Len(t, warnings, 2)
	assert.Contains(t, warnings[0], "feature.future")
	assert.Contains(t, warnings[1], "quota.max_models")
}

func TestValidateLicenseExtra_TypeMismatch(t *testing.T) {
	_, err := ValidateLicenseExtra(`{"limits":{"feature.audit_log":3}}`)
	require.Error(t, err)
}

func TestValidateLicenseExtraForIssue_RejectsUnknownFeature(t *testing.T) {
	err := ValidateLicenseExtraForIssue(`{"features":{"feature.audti_log":false}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature.audti_log")
}

func TestValidateLicenseExtraForIssue_RejectsUnknownTopLevelField(t *testing.T) {
	err := ValidateLicenseExtraForIssue(`{"feature":{"feature.audit_log":false}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature")
}

func TestValidateLicenseExtraForIssue_RejectsUnknownLimit(t *testing.T) {
	err := ValidateLicenseExtraForIssue(`{"limits":{"quota.max_models":50}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota.max_models")
}

func TestValidateLicenseExtraForImport_ToleratesUnknownFieldsAndKeys(t *testing.T) {
	warnings, err := ValidateLicenseExtraForImport(`{"feature":{"feature.audit_log":false},"features":{"feature.future":true},"limits":{"quota.max_models":50}}`)
	require.NoError(t, err)
	assert.Len(t, warnings, 3)
	assert.Contains(t, warnings, `unknown top-level field "feature" in license extra`)
	assert.Contains(t, warnings, `unknown feature flag "feature.future" in license extra`)
	assert.Contains(t, warnings, `unknown limit "quota.max_models" in license extra`)
}

func TestValidateLicenseExtraForImport_ToleratesLegacyExtra(t *testing.T) {
	warnings, err := ValidateLicenseExtraForImport(`{"token_limit":10}`)
	require.NoError(t, err)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "token_limit")
}

func TestParseLicenseExtra_IgnoresUnknownFutureKeys(t *testing.T) {
	extra := `{
		"features":{
			"feature.audit_log":false,
			"feature.future_object":{"enabled":true},
			"feature.future_string":"enabled"
		},
		"limits":{
			"quota.future_object":{"max":5},
			"quota.future_string":"5"
		}
	}`

	parsed, err := ParseLicenseExtra(extra)

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"feature.audit_log": false}, parsed.Features)
	assert.Empty(t, parsed.Limits)
}

func TestParseLicenseExtra_KnownFeatureWrongTypeErrors(t *testing.T) {
	_, err := ParseLicenseExtra(`{"features":{"feature.audit_log":"false"}}`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature.audit_log")
}

func TestParseLicenseExtra_KnownKeyInWrongSectionErrors(t *testing.T) {
	_, err := ParseLicenseExtra(`{"limits":{"feature.audit_log":true}}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feature.audit_log")
}

func TestValidateLicenseExtraForIssue_AcceptsCSGLiteKeys(t *testing.T) {
	extra := `{"features": {"feature.lite.observability": false, "feature.lite.ai_apps": true}, "limits": {"quota.lite.max_provider_pools": 3}}`
	require.NoError(t, ValidateLicenseExtraForIssue(extra))

	parsed, err := ParseLicenseExtra(extra)
	require.NoError(t, err)
	require.False(t, parsed.Features["feature.lite.observability"])
	require.True(t, parsed.Features["feature.lite.ai_apps"])
	require.Equal(t, 3, parsed.Limits["quota.lite.max_provider_pools"])
}

func TestValidateLicenseExtraForIssue_AcceptsUnregisteredCSGLiteKeys(t *testing.T) {
	// CSGLite's catalog is the source of truth for the "lite." segment, so a
	// key added there can be signed before this registry learns about it.
	extra := `{"features": {"feature.lite.brand_new": true}, "limits": {"quota.lite.brand_new_limit": 5}}`
	require.NoError(t, ValidateLicenseExtraForIssue(extra))

	warnings, err := ValidateLicenseExtraForImport(extra)
	require.NoError(t, err)
	require.Empty(t, warnings)
}

func TestValidateLicenseExtraForIssue_TypeChecksUnregisteredCSGLiteKeys(t *testing.T) {
	require.Error(t, ValidateLicenseExtraForIssue(`{"features": {"feature.lite.brand_new": "yes"}}`))
	require.Error(t, ValidateLicenseExtraForIssue(`{"limits": {"quota.lite.brand_new": true}}`))
	// Wrong section for the prefix is rejected even on import.
	_, err := ValidateLicenseExtraForImport(`{"limits": {"feature.lite.brand_new": 1}}`)
	require.Error(t, err)
	_, err = ValidateLicenseExtraForImport(`{"features": {"quota.lite.brand_new": true}}`)
	require.Error(t, err)
	// A bare prefix is not a key.
	require.Error(t, ValidateLicenseExtraForIssue(`{"features": {"feature.lite.": true}}`))
	// Other products' unknown keys are still rejected by the issuer.
	require.Error(t, ValidateLicenseExtraForIssue(`{"features": {"feature.hub.brand_new": true}}`))
}
