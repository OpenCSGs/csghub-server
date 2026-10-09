package types

import (
	"fmt"
	"reflect"
	"strings"
)

// FeatureType describes the value type of a feature gate entry.
type FeatureType string

const (
	FeatureTypeBoolean FeatureType = "boolean"
	FeatureTypeInt     FeatureType = "int"
)

// FeatureDefinition is the single source of truth for all feature flags and
// license-backed limits. Every flag/limit used in the codebase should be
// declared here so that business code does not rely on raw strings.
//
// Keys are namespaced ("feature.*" for boolean entitlements, "quota.*" for
// numeric limits) so they can share one flat flag namespace with future
// remote providers (e.g. flagd) without collisions.
type FeatureDefinition struct {
	Key          string
	Type         FeatureType
	DefaultValue any
}

// Boolean feature flags.
// These are stored in License.Extra under the "features" key.
var (
	featureAuditLog = FeatureDefinition{
		Key:          "feature.audit_log",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureAuditLog = featureAuditLog
)

// CSGLite feature flags and limits.
// CSGLite (github.com/opencsgs/csglite) verifies licenses signed by this
// issuer with Product "CSGLite". Its keys carry a "lite." product segment so
// they cannot collide with CSGHub's own flags in this flat namespace, and they
// must match internal/license/features.go in the csglite repository verbatim.
var (
	FeatureLiteProviderPools = FeatureDefinition{
		Key:          "feature.lite.provider_pools",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureLiteObservability = FeatureDefinition{
		Key:          "feature.lite.observability",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureLiteRemoteAPIKeys = FeatureDefinition{
		Key:          "feature.lite.remote_api_keys",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureLiteAIApps = FeatureDefinition{
		Key:          "feature.lite.ai_apps",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureLiteRealtimeVoice = FeatureDefinition{
		Key:          "feature.lite.realtime_voice",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	FeatureLiteImageGeneration = FeatureDefinition{
		Key:          "feature.lite.image_generation",
		Type:         FeatureTypeBoolean,
		DefaultValue: true,
	}
	// QuotaLiteMaxProviderPools caps configured provider pools; 0 means unlimited.
	QuotaLiteMaxProviderPools = FeatureDefinition{
		Key:          "quota.lite.max_provider_pools",
		Type:         FeatureTypeInt,
		DefaultValue: 0,
	}
)

var csgliteFeatureCatalog = []FeatureDefinition{
	FeatureLiteProviderPools,
	FeatureLiteObservability,
	FeatureLiteRemoteAPIKeys,
	FeatureLiteAIApps,
	FeatureLiteRealtimeVoice,
	FeatureLiteImageGeneration,
	QuotaLiteMaxProviderPools,
}

var featureCatalog = append([]FeatureDefinition{featureAuditLog}, csgliteFeatureCatalog...)

// FeatureDefinitions returns all registered feature flags and limits. It is
// useful for validation, documentation generation, and SaaS license issuer UI.
func FeatureDefinitions() []FeatureDefinition {
	return append([]FeatureDefinition(nil), featureCatalog...)
}

func ValidateFeatureCatalog(definitions []FeatureDefinition) error {
	seen := make(map[string]struct{}, len(definitions))
	for _, def := range definitions {
		if def.Key == "" {
			return fmt.Errorf("feature key is empty")
		}
		if _, ok := seen[def.Key]; ok {
			return fmt.Errorf("duplicate feature key %q", def.Key)
		}
		seen[def.Key] = struct{}{}
		switch def.Type {
		case FeatureTypeBoolean:
			if !strings.HasPrefix(def.Key, "feature.") {
				return fmt.Errorf("boolean feature %q must use feature.* namespace", def.Key)
			}
			if _, ok := def.DefaultValue.(bool); !ok {
				return fmt.Errorf("boolean feature %q must have bool default", def.Key)
			}
		case FeatureTypeInt:
			if !strings.HasPrefix(def.Key, "quota.") {
				return fmt.Errorf("integer limit %q must use quota.* namespace", def.Key)
			}
			if _, ok := def.DefaultValue.(int); !ok {
				return fmt.Errorf("integer limit %q must have int default", def.Key)
			}
		default:
			return fmt.Errorf("feature %q has unsupported type %q", def.Key, def.Type)
		}
	}
	return nil
}

type LicenseFeatureDefinitionResp struct {
	Key          string      `json:"key"`
	Type         FeatureType `json:"type"`
	DefaultValue any         `json:"default_value"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
}

func ValidateRegisteredDefinition(def FeatureDefinition) error {
	for _, registered := range featureCatalog {
		if registered.Key != def.Key {
			continue
		}
		if !reflect.DeepEqual(registered, def) {
			return fmt.Errorf("feature definition %q does not match registered catalog", def.Key)
		}
		return nil
	}
	return fmt.Errorf("feature definition %q is not registered", def.Key)
}
