package types

import (
	"slices"
	"strings"
)

// ParseModelScope splits an API key's model scope allocation (a
// comma-separated model ID list) into trimmed, non-empty model IDs. A nil or
// empty result means the key is not restricted to any model list.
func ParseModelScope(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil
	}
	parts := strings.Split(scope, ",")
	models := make([]string, 0, len(parts))
	for _, part := range parts {
		if id := strings.TrimSpace(part); id != "" {
			models = append(models, id)
		}
	}
	return models
}

// ModelInScope reports whether modelID is contained in an API key's model
// scope. An empty scope allows every model.
func ModelInScope(scope []string, modelID string) bool {
	if len(scope) == 0 {
		return true
	}
	return slices.Contains(scope, modelID)
}
