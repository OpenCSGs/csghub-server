package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestValidateUpstreamMetadataReasoningRequest(t *testing.T) {
	valid := &types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{
			ReasoningRequest: &types.ReasoningRequestConfig{
				Enabled:      true,
				EffortField:  "reasoning_effort",
				EnableExtra:  map[string]any{"enable_thinking": true},
			},
		},
	}
	require.NoError(t, validateUpstreamMetadata(valid))
	require.NoError(t, validateUpstreamMetadata(nil))
	require.NoError(t, validateUpstreamMetadata(&types.UpstreamMetadata{}))
	require.NoError(t, validateUpstreamMetadata(&types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{},
	}))

	// effort_field must not also appear in enable_extra
	err := validateUpstreamMetadata(&types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{
			ReasoningRequest: &types.ReasoningRequestConfig{
				EffortField: "reasoning_effort",
				EnableExtra: map[string]any{"reasoning_effort": "high"},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not also appear in enable_extra")

	// effort_field must not also appear in disable_extra
	err = validateUpstreamMetadata(&types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{
			ReasoningRequest: &types.ReasoningRequestConfig{
				EffortField:  "reasoning_effort",
				DisableExtra: map[string]any{"reasoning_effort": "low"},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not also appear in disable_extra")

	// nil ReasoningRequest is valid
	require.NoError(t, validateUpstreamMetadata(&types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{
			ReasoningRequest: nil,
		},
	}))
}

func TestBuildUpstreamConfigsMetadataPassthrough(t *testing.T) {
	metadata := &types.UpstreamMetadata{
		ResponsesChatAdapter: &types.ResponsesChatAdapter{
			ReasoningRequest: &types.ReasoningRequestConfig{
				Enabled: true,
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
