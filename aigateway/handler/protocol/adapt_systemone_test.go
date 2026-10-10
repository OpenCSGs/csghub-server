package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// systemoneRoutingTarget builds a RoutingTarget for a Jev/System One model.
func systemoneRoutingTarget(targetURL, protocolOverride string, csgHubHosted bool) RoutingTarget {
	return RoutingTarget{
		ModelID:          "jev-1.13",
		Target:           targetURL,
		CSGHubHosted:     csgHubHosted,
		ProtocolOverride: protocolOverride,
	}
}

func TestResolveRouting_SystemOneClient_Native(t *testing.T) {
	// External upstream whose URL ends in /v1/systemone: native passthrough.
	decision, err := ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("https://openrouter.example/api/v1/systemone", "", false))
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, types.ProtocolSystemOne, decision.UpstreamProtocol)
	assert.Equal(t, "https://openrouter.example/api/v1/systemone", decision.BackendURL)
}

func TestResolveRouting_SystemOneClient_MetadataOverride(t *testing.T) {
	// Explicit metadata protocol wins even when the URL has no recognizable path.
	decision, err := ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("https://jev.internal/api/evaluate", "systemone", false))
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, types.ProtocolSystemOne, decision.UpstreamProtocol)
	assert.Equal(t, "https://jev.internal/api/evaluate", decision.BackendURL)
}

func TestResolveRouting_SystemOneClient_UnknownURL_NativeFallback(t *testing.T) {
	// External upstream with a bare URL and no metadata: the client's own
	// protocol wins so the request takes the native passthrough path.
	decision, err := ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("https://jev.internal:8080", "", false))
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, types.ProtocolSystemOne, decision.UpstreamProtocol)
}

func TestResolveRouting_SystemOneClient_CSGHubHosted_AppendsPath(t *testing.T) {
	// A CSGHub-hosted deployment speaks Chat by default (vLLM/SGLang), so a
	// systemone request is rejected unless the upstream explicitly declares
	// the System One protocol via metadata.
	decision, err := ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("http://jev-svc:8000", "", true))
	require.NoError(t, err)
	assert.Equal(t, ModeDisabled, decision.Mode)

	// With the metadata override the standard path is appended.
	decision, err = ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("http://jev-svc:8000", "systemone", true))
	require.NoError(t, err)
	assert.Equal(t, ModeNative, decision.Mode)
	assert.Equal(t, types.ProtocolSystemOne, decision.UpstreamProtocol)
	assert.Equal(t, "http://jev-svc:8000/v1/systemone", decision.BackendURL)
}

func TestResolveRouting_SystemOneClient_ToChatUpstream_Rejected(t *testing.T) {
	// A systemone request against a chat upstream has no adapter: reject.
	decision, err := ResolveRouting(types.ProtocolSystemOne,
		systemoneRoutingTarget("https://upstream/v1/chat/completions", "", false))
	require.NoError(t, err)
	assert.Equal(t, ModeDisabled, decision.Mode)
	assert.Equal(t, "no_adapter_for_systemone_to_chat", decision.Reason)
}

func TestResolveRouting_ChatClient_ToSystemOneUpstream_Rejected(t *testing.T) {
	// The reverse direction is rejected as well.
	decision, err := ResolveRouting(types.ProtocolChat,
		systemoneRoutingTarget("https://upstream/v1/systemone", "", false))
	require.NoError(t, err)
	assert.Equal(t, ModeDisabled, decision.Mode)
	assert.Equal(t, "no_adapter_for_chat_to_systemone", decision.Reason)
}

func TestDetectUpstreamProtocol_SystemOne(t *testing.T) {
	assert.Equal(t, types.ProtocolSystemOne,
		DetectUpstreamProtocol(systemoneRoutingTarget("https://x/api/v1/systemone", "", false)))
	assert.Equal(t, types.ProtocolSystemOne,
		DetectUpstreamProtocol(systemoneRoutingTarget("https://x/anything", "SystemOne", false)))
	// Chat remains the fallback for non-systemone inputs.
	assert.Equal(t, types.ProtocolChat,
		DetectUpstreamProtocol(systemoneRoutingTarget("https://x", "", false)))
}
