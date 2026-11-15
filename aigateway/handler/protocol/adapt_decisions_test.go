package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestResolveDecisionsNativeUpstream(t *testing.T) {
	decision, err := ResolveRouting(types.ProtocolDecisions, RoutingTarget{
		Target: "https://api.openai.com/v1/decisions",
	})
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, types.ProtocolDecisions, decision.UpstreamProtocol)
	assert.Equal(t, "https://api.openai.com/v1/decisions", decision.BackendURL)
}

func TestResolveDecisionsWithDeclaredEndpoint(t *testing.T) {
	decision, err := ResolveRouting(types.ProtocolDecisions, RoutingTarget{
		Target:             "https://api.openai.com/v1",
		SupportedEndpoints: map[commontypes.ApiFormat]string{commontypes.ApiFormatDecisions: ""},
	})
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, "https://api.openai.com/v1/decisions", decision.BackendURL)
}

// TestResolveRouting_DecisionsPathNormalization covers a full decisions
// endpoint URL combined with a supported_endpoints declaration and
// IsFullURL=false: the resource tail must be normalized (stripped, then the
// standard path appended) instead of being appended a second time.
func TestResolveRouting_DecisionsPathNormalization(t *testing.T) {
	cases := []struct {
		target   string
		expected string
	}{
		{"https://example.com/v1/decisions", "https://example.com/v1/decisions"},
		{"https://example.com/decisions", "https://example.com/v1/decisions"},
		{"https://example.com/api/v1/decisions", "https://example.com/api/v1/decisions"},
		{"https://example.com/v1", "https://example.com/v1/decisions"},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			decision, err := ResolveRouting(types.ProtocolDecisions, RoutingTarget{
				ModelID:            "decision-model",
				Target:             tc.target,
				SupportedEndpoints: map[commontypes.ApiFormat]string{commontypes.ApiFormatDecisions: ""},
				IsFullURL:          false,
			})
			require.NoError(t, err)
			assert.Equal(t, ModeNative, decision.Mode)
			assert.Equal(t, tc.expected, decision.BackendURL)
		})
	}
}
