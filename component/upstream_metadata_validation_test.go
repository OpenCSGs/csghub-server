package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestValidateUpstreamMetadataReasoningRequest(t *testing.T) {
	valid := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled":      true,
					"effort_field": "reasoning_effort",
					"enable_extra": map[string]any{"enable_thinking": true},
				},
			},
		},
	}
	require.NoError(t, validateUpstreamMetadata(valid))
	require.NoError(t, validateUpstreamMetadata(nil))
	require.NoError(t, validateUpstreamMetadata(map[string]any{}))

	err := validateUpstreamMetadata(map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled": "yes",
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "enabled must be a boolean")

	err = validateUpstreamMetadata(map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"effort_field": 123,
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "effort_field must be a string")

	err = validateUpstreamMetadata(map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"effort_field": "reasoning_effort",
					"enable_extra": "bad",
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "enable_extra must be a JSON object")

	err = validateUpstreamMetadata(map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"effort_field": "reasoning_effort",
					"enable_extra": map[string]any{"reasoning_effort": "high"},
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not also appear in enable_extra")
}

func TestBuildUpstreamConfigsMetadataPassthrough(t *testing.T) {
	metadata := map[string]any{
		"responses": map[string]any{
			"chat_adapter": map[string]any{
				"reasoning_request": map[string]any{
					"enabled": true,
				},
			},
		},
	}
	result := buildUpstreamConfigs([]database.Upstream{{
		ID:       1,
		URL:      "http://upstream.example.com/v1/chat/completions",
		Enabled:  true,
		Metadata: metadata,
	}})
	require.Len(t, result, 1)
	require.Equal(t, metadata, result[0].Metadata)
}
