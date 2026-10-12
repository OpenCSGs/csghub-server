package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestDetectUpstreamProtocol(t *testing.T) {
	tests := []struct {
		name     string
		target   RoutingTarget
		expected types.Protocol
	}{
		{
			name:     "metadata explicit messages",
			target:   RoutingTarget{Target: "https://api.anthropic.com/v1/messages", ProtocolOverride: "messages"},
			expected: types.ProtocolMessages,
		},
		{
			name:     "metadata explicit responses",
			target:   RoutingTarget{Target: "https://api.openai.com/v1/responses", ProtocolOverride: "responses"},
			expected: types.ProtocolResponses,
		},
		{
			name:     "metadata explicit chat",
			target:   RoutingTarget{Target: "https://example.com/v1/chat/completions", ProtocolOverride: "chat"},
			expected: types.ProtocolChat,
		},
		{
			name:     "url path messages",
			target:   RoutingTarget{Target: "https://api.anthropic.com/v1/messages"},
			expected: types.ProtocolMessages,
		},
		{
			name:     "url path responses",
			target:   RoutingTarget{Target: "https://api.openai.com/v1/responses"},
			expected: types.ProtocolResponses,
		},
		{
			name:     "url path chat completions",
			target:   RoutingTarget{Target: "https://example.com/v1/chat/completions"},
			expected: types.ProtocolChat,
		},
		{
			name:     "csghub hosted defaults to chat",
			target:   RoutingTarget{Target: "http://model-svc:8080", CSGHubHosted: true},
			expected: types.ProtocolChat,
		},
		{
			name:     "bare url defaults to chat",
			target:   RoutingTarget{Target: "https://example.com"},
			expected: types.ProtocolChat,
		},
		{
			name:     "metadata overrides url path",
			target:   RoutingTarget{Target: "https://example.com/v1/chat/completions", ProtocolOverride: "messages"},
			expected: types.ProtocolMessages,
		},
		{
			name:     "metadata invalid protocol falls through to url",
			target:   RoutingTarget{Target: "https://example.com/v1/responses", ProtocolOverride: "invalid"},
			expected: types.ProtocolResponses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectUpstreamProtocol(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveRoutingNative(t *testing.T) {
	tests := []struct {
		name           string
		clientProtocol types.Protocol
		target         RoutingTarget
	}{
		{
			name:           "messages to messages",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://api.anthropic.com/v1/messages"},
		},
		{
			name:           "chat to chat",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://example.com/v1/chat/completions"},
		},
		{
			name:           "responses to responses",
			clientProtocol: types.ProtocolResponses,
			target:         RoutingTarget{Target: "https://example.com/v1/responses"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ResolveRouting(tt.clientProtocol, tt.target)
			require.NoError(t, err)
			assert.Equal(t, ModeNative, decision.Mode)
			assert.Equal(t, AdapterNone, decision.AdapterKind)
			assert.Equal(t, tt.target.Target, decision.BackendURL)
			assert.Equal(t, "protocol_match", decision.Reason)
		})
	}
}

func TestResolveRoutingAdapter(t *testing.T) {
	tests := []struct {
		name           string
		clientProtocol types.Protocol
		target         RoutingTarget
		expectedKind   AdapterKind
		expectedProto  types.Protocol
	}{
		{
			name:           "messages to chat adapter",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://vllm.example.com/v1/chat/completions"},
			expectedKind:   AdapterMessagesToChat,
			expectedProto:  types.ProtocolChat,
		},
		{
			name:           "messages to responses adapter",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://openai.example.com/v1/responses"},
			expectedKind:   AdapterMessagesToResponses,
			expectedProto:  types.ProtocolResponses,
		},
		{
			name:           "responses to chat adapter",
			clientProtocol: types.ProtocolResponses,
			target:         RoutingTarget{Target: "https://vllm.example.com/v1/chat/completions"},
			expectedKind:   AdapterResponsesToChat,
			expectedProto:  types.ProtocolChat,
		},
		{
			name:           "messages to chat via csghub hosted",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "http://model-svc:8080", CSGHubHosted: true},
			expectedKind:   AdapterMessagesToChat,
			expectedProto:  types.ProtocolChat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ResolveRouting(tt.clientProtocol, tt.target)
			require.NoError(t, err)
			assert.Equal(t, ModeAdapter, decision.Mode)
			assert.Equal(t, tt.expectedKind, decision.AdapterKind)
			assert.Equal(t, tt.expectedProto, decision.UpstreamProtocol)
		})
	}
}

func TestResolveRoutingDisabled(t *testing.T) {
	tests := []struct {
		name           string
		clientProtocol types.Protocol
		target         RoutingTarget
	}{
		{
			name:           "responses to messages - no adapter",
			clientProtocol: types.ProtocolResponses,
			target:         RoutingTarget{Target: "https://api.anthropic.com/v1/messages"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ResolveRouting(tt.clientProtocol, tt.target)
			require.NoError(t, err)
			assert.Equal(t, ModeDisabled, decision.Mode)
			assert.Contains(t, decision.Reason, "no_adapter")
		})
	}
}

// TestResolveRoutingChatFallback verifies that when the client protocol is
// Chat (/v1/chat/completions) and the upstream is configured as Responses
// (/v1/responses) or Messages (/v1/messages), the request is NOT rejected.
// Instead the upstream URL path is rewritten to /v1/chat/completions and the
// request is forwarded natively — Chat is the most widely compatible protocol
// and most upstreams that expose /responses or /messages also accept
// /chat/completions, so we proxy directly rather than disabling the endpoint.
func TestResolveRoutingChatFallback(t *testing.T) {
	tests := []struct {
		name           string
		clientProtocol types.Protocol
		target         RoutingTarget
		expectedURL    string
	}{
		{
			name:           "chat to responses - rewrite path to chat/completions",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://example.com/v1/responses"},
			expectedURL:    "https://example.com/v1/chat/completions",
		},
		{
			name:           "chat to messages - rewrite path to chat/completions",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://api.anthropic.com/v1/messages"},
			expectedURL:    "https://api.anthropic.com/v1/chat/completions",
		},
		{
			name:           "chat to messages under /anthropic namespace - strip namespace",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://api.deepseek.com/anthropic/messages"},
			expectedURL:    "https://api.deepseek.com/chat/completions",
		},
		{
			name:           "chat to messages with path prefix under /anthropic namespace - strip namespace only",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://gateway.example.com/api/anthropic/messages"},
			expectedURL:    "https://gateway.example.com/api/chat/completions",
		},
		{
			name:           "chat to responses via metadata - rewrite path",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://example.com/v1/responses", UpstreamMetadata: map[string]any{"protocol": "responses"}},
			expectedURL:    "https://example.com/v1/chat/completions",
		},
		{
			name:           "chat to messages via metadata - rewrite path",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://api.anthropic.com/v1/messages", UpstreamMetadata: map[string]any{"protocol": "messages"}},
			expectedURL:    "https://api.anthropic.com/v1/chat/completions",
		},
		{
			name:           "chat to a DeepSeek-style /anthropic messages endpoint - rewrite path",
			clientProtocol: types.ProtocolChat,
			// DeepSeek-style /anthropic namespaces expose only Messages; their
			// chat-completions endpoint lives OUTSIDE the namespace, so the
			// fallback strips the namespace tail together with /messages.
			target:      RoutingTarget{Target: "https://api.anthropic.com/v1/anthropic/messages"},
			expectedURL: "https://api.anthropic.com/v1/chat/completions",
		},
		{
			name:           "chat to responses with path prefix - preserve prefix",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://gateway.example.com/api/v1/responses"},
			expectedURL:    "https://gateway.example.com/api/v1/chat/completions",
		},
		{
			name:           "chat to messages with path prefix - preserve prefix",
			clientProtocol: types.ProtocolChat,
			target:         RoutingTarget{Target: "https://gateway.example.com/api/v1/messages"},
			expectedURL:    "https://gateway.example.com/api/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ResolveRouting(tt.clientProtocol, tt.target)
			require.NoError(t, err)
			assert.Equal(t, ModeNative, decision.Mode)
			assert.Equal(t, AdapterNone, decision.AdapterKind)
			assert.Equal(t, tt.expectedURL, decision.BackendURL)
			// UpstreamProtocol should still reflect the actual upstream protocol.
			assert.NotEqual(t, types.ProtocolChat, decision.UpstreamProtocol)
		})
	}
}

// TestRewriteToChatPath pins the URL rewriting contract of the Chat fallback.
// Only the terminal path segment is replaced so that any prefix is preserved
// (/v1/messages -> /v1/chat/completions), with one exception: a trailing
// /anthropic namespace segment directly before /messages is dropped, because
// providers such as DeepSeek serve their Anthropic-compatible API under
// /anthropic (https://api.deepseek.com/anthropic/messages) while the
// chat-completions endpoint lives outside that namespace — rewriting the
// terminal segment in place would produce /anthropic/chat/completions, which
// upstreams answer with 404.
func TestRewriteToChatPath(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		expectedURL string
	}{
		{
			name:        "anthropic namespace messages endpoint strips /anthropic",
			target:      "https://api.deepseek.com/anthropic/messages",
			expectedURL: "https://api.deepseek.com/chat/completions",
		},
		{
			name:        "v1 messages endpoint keeps /v1 prefix",
			target:      "https://api.anthropic.com/v1/messages",
			expectedURL: "https://api.anthropic.com/v1/chat/completions",
		},
		{
			name:        "generic path prefix messages endpoint keeps prefix",
			target:      "https://gateway.example.com/api/v1/messages",
			expectedURL: "https://gateway.example.com/api/v1/chat/completions",
		},
		{
			name:        "path prefix with anthropic namespace strips namespace only",
			target:      "https://gateway.example.com/api/anthropic/messages",
			expectedURL: "https://gateway.example.com/api/chat/completions",
		},
		{
			name:        "bare messages endpoint rewrites terminal segment",
			target:      "https://example.com/messages",
			expectedURL: "https://example.com/chat/completions",
		},
		{
			name:        "anthropic namespace messages endpoint with trailing slash strips /anthropic",
			target:      "https://api.deepseek.com/anthropic/messages/",
			expectedURL: "https://api.deepseek.com/chat/completions",
		},
		{
			name:        "anthropic terminal segment keeps old terminal replacement",
			target:      "https://api.anthropic.com/v1/anthropic",
			expectedURL: "https://api.anthropic.com/v1/chat/completions",
		},
		{
			name:        "responses endpoint keeps old terminal replacement",
			target:      "https://example.com/v1/responses",
			expectedURL: "https://example.com/v1/chat/completions",
		},
		{
			name:        "responses endpoint under anthropic namespace keeps namespace",
			target:      "https://example.com/anthropic/responses",
			expectedURL: "https://example.com/anthropic/chat/completions",
		},
		{
			name:        "bare host appends default chat path",
			target:      "https://example.com",
			expectedURL: "https://example.com/v1/chat/completions",
		},
		{
			name:        "empty target returns empty",
			target:      "",
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedURL, rewriteToChatPath(RoutingTarget{Target: tt.target}))
		})
	}
}

func TestResolveRoutingEmptyTarget(t *testing.T) {
	_, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{Target: ""})
	assert.Error(t, err)
}

// TestResolveRoutingNativeFallback verifies that when the upstream protocol
// cannot be inferred from the URL or metadata, and the model is NOT
// CSGHub-hosted, the routing falls back to the client's own protocol (native
// passthrough) rather than forcing the Chat adapter.  This is critical for
// Claude Code compatibility: an external Anthropic-native upstream whose URL
// does not end in /v1/messages should receive the request natively, not through
// the chat adapter (which injects type:"function" into tools).
func TestResolveRoutingNativeFallback(t *testing.T) {
	tests := []struct {
		name           string
		clientProtocol types.Protocol
		target         RoutingTarget
		expectedMode   ExecutionMode
		expectedProto  types.Protocol
	}{
		{
			name:           "messages client, bare external url → native messages",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://external.example.com"},
			expectedMode:   ModeNative,
			expectedProto:  types.ProtocolMessages,
		},
		{
			name:           "messages client, external url with generic path → native messages",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://external.example.com/api"},
			expectedMode:   ModeNative,
			expectedProto:  types.ProtocolMessages,
		},
		{
			name:           "responses client, bare external url → native responses",
			clientProtocol: types.ProtocolResponses,
			target:         RoutingTarget{Target: "https://external.example.com"},
			expectedMode:   ModeNative,
			expectedProto:  types.ProtocolResponses,
		},
		{
			name:           "messages client, csghub hosted → still chat adapter (no fallback)",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "http://model-svc:8080", CSGHubHosted: true},
			expectedMode:   ModeAdapter,
			expectedProto:  types.ProtocolChat,
		},
		{
			name:           "messages client, external url with metadata protocol=chat → chat adapter (metadata wins)",
			clientProtocol: types.ProtocolMessages,
			target:         RoutingTarget{Target: "https://external.example.com", ProtocolOverride: "chat"},
			expectedMode:   ModeAdapter,
			expectedProto:  types.ProtocolChat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ResolveRouting(tt.clientProtocol, tt.target)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMode, decision.Mode)
			assert.Equal(t, tt.expectedProto, decision.UpstreamProtocol)
		})
	}
}

func TestAdaptBackendURL(t *testing.T) {
	t.Run("url already has path - keep as is", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target: "https://vllm.example.com/v1/chat/completions",
		})
		require.NoError(t, err)
		assert.Equal(t, "https://vllm.example.com/v1/chat/completions", decision.BackendURL)
	})

	t.Run("csghub hosted chat - append path", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:       "http://model-svc:8080",
			CSGHubHosted: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "http://model-svc:8080/v1/chat/completions", decision.BackendURL)
	})

	t.Run("csghub hosted responses - append path", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:           "http://model-svc:8080",
			CSGHubHosted:     true,
			ProtocolOverride: "responses",
		})
		require.NoError(t, err)
		assert.Equal(t, "http://model-svc:8080/v1/responses", decision.BackendURL)
	})
}

func TestPathEndsWithSegments(t *testing.T) {
	tests := []struct {
		path     string
		segments []string
		expected bool
	}{
		{"/v1/chat/completions", []string{"chat", "completions"}, true},
		{"/v1/responses", []string{"responses"}, true},
		{"/v1/messages", []string{"messages"}, true},
		{"/v1/chat", []string{"chat", "completions"}, false},
		{"/responses", []string{"chat", "completions"}, false},
		{"/v1/chat/completions/", []string{"chat", "completions"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, pathEndsWithSegments(tt.path, tt.segments...))
		})
	}
}

func TestResolveRoutingDeclaredFormat(t *testing.T) {
	t.Run("declared chat on base URL appends client path", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolChat, RoutingTarget{
			Target:             "https://dashscope.aliyuncs.com/compatible-mode/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", decision.BackendURL)
		assert.Equal(t, "declared_format_match", decision.Reason)
	})

	t.Run("declared multiple formats route each client protocol natively", func(t *testing.T) {
		target := RoutingTarget{
			Target:             "https://gw.example.com/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "responses": "", "messages": ""},
		}
		for _, tc := range []struct {
			client       types.Protocol
			expectedPath string
		}{
			{types.ProtocolChat, "/chat/completions"},
			{types.ProtocolResponses, "/responses"},
			{types.ProtocolMessages, "/messages"},
		} {
			decision, err := ResolveRouting(tc.client, target)
			require.NoError(t, err)
			assert.Equal(t, ModeNative, decision.Mode)
			assert.Equal(t, "https://gw.example.com/v1"+tc.expectedPath, decision.BackendURL)
		}
	})

	t.Run("full URL is used as-is", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolChat, RoutingTarget{
			Target:             "https://api.openai.com/v1/chat/completions",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
			IsFullURL:          true,
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://api.openai.com/v1/chat/completions", decision.BackendURL)
	})

	t.Run("declared format strips protocol tail before appending another", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolResponses, RoutingTarget{
			Target:             "https://gw.example.com/v1/chat/completions",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "responses": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://gw.example.com/v1/responses", decision.BackendURL)
	})

	t.Run("declared format on CSGHub-hosted bare host appends path", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolChat, RoutingTarget{
			Target:             "http://model-svc:8080",
			CSGHubHosted:       true,
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "http://model-svc:8080/v1/chat/completions", decision.BackendURL)
	})

	t.Run("undeclared client protocol falls back to legacy adapter", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolResponses, RoutingTarget{
			Target:             "https://dashscope.aliyuncs.com/compatible-mode/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeAdapter, decision.Mode)
		assert.Equal(t, AdapterResponsesToChat, decision.AdapterKind)
		// The declared base URL gets the chat path appended for the adapter.
		assert.Equal(t, "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions", decision.BackendURL)
	})

	t.Run("declared chat blocks undeclared passthrough for non-chat clients", func(t *testing.T) {
		// Without a declaration an un-inferable external URL passes the
		// client protocol through natively; a chat declaration must keep the
		// adapter path instead.
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:             "https://gw.example.com/custom",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeAdapter, decision.Mode)
		assert.Equal(t, AdapterMessagesToChat, decision.AdapterKind)
	})

	t.Run("undeclared un-inferable external URL keeps legacy passthrough", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target: "https://gw.example.com/custom",
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://gw.example.com/custom", decision.BackendURL)
	})

	t.Run("per-protocol endpoint override with matching tail is used as-is", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:             "https://api.deepseek.com/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "messages": "https://api.deepseek.com/anthropic/messages"},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://api.deepseek.com/anthropic/messages", decision.BackendURL)
	})

	t.Run("per-protocol endpoint override without tail gets the standard path", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:             "https://api.deepseek.com/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "messages": "https://relay.example.com/deepseek"},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://relay.example.com/deepseek/v1/messages", decision.BackendURL)
	})

	t.Run("main URL serves protocols with empty override values", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolChat, RoutingTarget{
			Target:             "https://api.deepseek.com/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "messages": "https://api.deepseek.com/anthropic/messages"},
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://api.deepseek.com/v1/chat/completions", decision.BackendURL)
	})

	t.Run("metadata protocol merges into declared set", func(t *testing.T) {
		decision, err := ResolveRouting(types.ProtocolMessages, RoutingTarget{
			Target:             "https://api.anthropic.com/v1",
			SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""},
			ProtocolOverride:   "messages",
		})
		require.NoError(t, err)
		assert.Equal(t, ModeNative, decision.Mode)
		assert.Equal(t, "https://api.anthropic.com/v1/messages", decision.BackendURL)
	})
}

func TestStripProtocolTail(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{"https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1"},
		{"https://api.openai.com/v1/responses", "https://api.openai.com/v1"},
		{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com/v1"},
		{"https://api.deepseek.com/anthropic/messages", "https://api.deepseek.com/anthropic"},
		{"https://gw.example.com/v1", "https://gw.example.com/v1"},
		{"http://model-svc:8080", "http://model-svc:8080"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, StripProtocolTail(tt.raw), tt.raw)
	}
}
