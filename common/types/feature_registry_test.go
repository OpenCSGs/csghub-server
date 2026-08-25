package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/i18n"
)

func TestFeatureCatalogIsValid(t *testing.T) {
	definitions := FeatureDefinitions()
	require.NoError(t, ValidateFeatureCatalog(definitions))
	require.Equal(t, []FeatureDefinition{FeatureAuditLog}, definitions)
}

func TestFeatureCatalogTranslations(t *testing.T) {
	i18n.InitLocalizersFromEmbedFile()
	for _, lang := range []string{"en-US", "zh-CN", "zh-HK"} {
		for _, def := range FeatureDefinitions() {
			prefix := "license." + def.Key
			name, ok := i18n.TranslateText(lang, prefix+".name", "")
			require.True(t, ok, "%s name translation missing for %s", lang, def.Key)
			require.NotEmpty(t, name)
			description, ok := i18n.TranslateText(lang, prefix+".description", "")
			require.True(t, ok, "%s description translation missing for %s", lang, def.Key)
			require.NotEmpty(t, description)
		}
	}
}

func TestValidateFeatureCatalog(t *testing.T) {
	valid := FeatureAuditLog
	tests := []struct {
		name string
		defs []FeatureDefinition
	}{
		{name: "empty key", defs: []FeatureDefinition{{Type: FeatureTypeBoolean, DefaultValue: true}}},
		{name: "duplicate key", defs: []FeatureDefinition{valid, valid}},
		{name: "boolean namespace", defs: []FeatureDefinition{{Key: "quota.audit", Type: FeatureTypeBoolean, DefaultValue: true}}},
		{name: "integer namespace", defs: []FeatureDefinition{{Key: "feature.users", Type: FeatureTypeInt, DefaultValue: 1}}},
		{name: "boolean default", defs: []FeatureDefinition{{Key: "feature.audit", Type: FeatureTypeBoolean, DefaultValue: 1}}},
		{name: "integer default", defs: []FeatureDefinition{{Key: "quota.users", Type: FeatureTypeInt, DefaultValue: true}}},
		{name: "unsupported type", defs: []FeatureDefinition{{Key: "feature.audit", Type: "string", DefaultValue: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, ValidateFeatureCatalog(tt.defs))
		})
	}
}

func TestValidateRegisteredDefinition(t *testing.T) {
	require.NoError(t, ValidateRegisteredDefinition(FeatureAuditLog))
	require.Error(t, ValidateRegisteredDefinition(FeatureDefinition{Key: "feature.audit_lgo", Type: FeatureTypeBoolean, DefaultValue: true}))

	modified := FeatureAuditLog
	modified.DefaultValue = false
	require.Error(t, ValidateRegisteredDefinition(modified))

	original := FeatureAuditLog
	FeatureAuditLog.DefaultValue = false
	t.Cleanup(func() { FeatureAuditLog = original })
	require.NoError(t, ValidateRegisteredDefinition(original))
	require.Error(t, ValidateRegisteredDefinition(FeatureAuditLog))
}
