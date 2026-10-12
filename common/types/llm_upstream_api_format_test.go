package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferProtocolFromURLPath(t *testing.T) {
	tests := []struct {
		raw      string
		expected ApiFormat
		ok       bool
	}{
		{"https://api.openai.com/v1/chat/completions", ApiFormatChat, true},
		{"https://api.openai.com/v1/responses", ApiFormatResponses, true},
		{"https://api.anthropic.com/v1/messages", ApiFormatMessages, true},
		{"https://api.deepseek.com/anthropic", ApiFormatMessages, true},
		{"https://gateway.example.com/v1/systemone", ApiFormatSystemOne, true},
		{"http://model-svc:8080", "", false},
		{"https://gw.example.com/v1", "", false},
		{"", "", false},
		{"https://api.openai.com/v1/completions", "", false},
	}
	for _, tt := range tests {
		got, ok := InferProtocolFromURLPath(tt.raw)
		assert.Equal(t, tt.ok, ok, tt.raw)
		assert.Equal(t, tt.expected, got, tt.raw)
	}
}

func TestSupportsProtocol(t *testing.T) {
	t.Run("explicit declaration wins", func(t *testing.T) {
		uc := UpstreamConfig{
			URL:                "https://api.openai.com/v1/chat/completions",
			SupportedEndpoints: map[ApiFormat]string{"chat": "", "responses": ""},
		}
		assert.True(t, uc.SupportsProtocol("chat"))
		assert.True(t, uc.SupportsProtocol("responses"))
		assert.False(t, uc.SupportsProtocol("messages"))
	})

	t.Run("falls back to URL inference for legacy rows", func(t *testing.T) {
		uc := UpstreamConfig{URL: "https://api.anthropic.com/v1/messages"}
		assert.True(t, uc.SupportsProtocol("messages"))
		assert.False(t, uc.SupportsProtocol("chat"))
	})

	t.Run("falls back to metadata protocol", func(t *testing.T) {
		uc := UpstreamConfig{
			URL:      "http://model-svc:8080",
			Metadata: &UpstreamMetadata{Protocol: "responses"},
		}
		assert.True(t, uc.SupportsProtocol("responses"))
		assert.False(t, uc.SupportsProtocol("chat"))
	})

	t.Run("metadata protocol merges into a non-empty declaration map", func(t *testing.T) {
		// A map key and the metadata override coexist: the routing layer's
		// SupportedProtocolsOf unions them, so candidate selection must too.
		uc := UpstreamConfig{
			URL:      "https://api.example.com/v1",
			Metadata: &UpstreamMetadata{Protocol: "messages"},
		}
		uc.SupportedEndpoints = map[ApiFormat]string{"chat": ""}
		assert.True(t, uc.SupportsProtocol("chat"))
		assert.True(t, uc.SupportsProtocol("messages"))
		assert.False(t, uc.SupportsProtocol("responses"))
		// The metadata-declared format has no endpoint override of its own.
		assert.Empty(t, uc.DeclaredProtocolEndpoint("messages"))
	})

	t.Run("CSGHub-hosted defaults to chat", func(t *testing.T) {
		uc := UpstreamConfig{URL: "http://model-svc:8080", Source: UpstreamSourceCSGHubDeploy}
		assert.True(t, uc.SupportsProtocol("chat"))
		assert.False(t, uc.SupportsProtocol("messages"))
	})

	t.Run("undeclared external upstream supports nothing", func(t *testing.T) {
		uc := UpstreamConfig{URL: "https://gw.example.com/custom"}
		assert.False(t, uc.SupportsProtocol("chat"))
	})

	t.Run("per-protocol endpoint override is returned by DeclaredProtocolEndpoint", func(t *testing.T) {
		uc := UpstreamConfig{
			URL:                "https://api.deepseek.com/v1",
			SupportedEndpoints: map[ApiFormat]string{"chat": "", "messages": "https://api.deepseek.com/anthropic"},
		}
		assert.True(t, uc.SupportsProtocol("chat"))
		assert.True(t, uc.SupportsProtocol("messages"))
		assert.False(t, uc.SupportsProtocol("responses"))
		assert.Empty(t, uc.DeclaredProtocolEndpoint("chat"))
		assert.Equal(t, "https://api.deepseek.com/anthropic", uc.DeclaredProtocolEndpoint("messages"))
		assert.Empty(t, uc.DeclaredProtocolEndpoint("responses"))
	})
}

func TestHasProtocolDeclaration(t *testing.T) {
	assert.True(t, UpstreamConfig{SupportedEndpoints: map[ApiFormat]string{"chat": ""}}.HasProtocolDeclaration())
	assert.True(t, UpstreamConfig{Metadata: &UpstreamMetadata{Protocol: "chat"}}.HasProtocolDeclaration())
	assert.True(t, UpstreamConfig{URL: "https://api.openai.com/v1/responses"}.HasProtocolDeclaration())
	assert.True(t, UpstreamConfig{URL: "http://svc:8080", Source: UpstreamSourceCSGHubDeploy}.HasProtocolDeclaration())
	assert.False(t, UpstreamConfig{URL: "https://gw.example.com/custom"}.HasProtocolDeclaration())
}

func TestResolveSampleEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		uc       UpstreamConfig
		expected string
	}{
		{"empty url", UpstreamConfig{}, ""},
		{"full url used as-is", UpstreamConfig{URL: "https://api.openai.com/v1/chat/completions", IsFullURL: true, Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"chat": ""}}, "https://api.openai.com/v1/chat/completions"},
		{"inferable url used as-is", UpstreamConfig{URL: "https://api.openai.com/v1/responses", Source: UpstreamSourceExternal}, "https://api.openai.com/v1/responses"},
		{"declared base gets chat path", UpstreamConfig{URL: "https://api.openai.com/v1", Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"chat": "", "responses": ""}}, "https://api.openai.com/v1/chat/completions"},
		{"declared base with query keeps it", UpstreamConfig{URL: "https://api.openai.com/openai?api-version=2024-10-21", Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"chat": ""}}, "https://api.openai.com/openai/v1/chat/completions?api-version=2024-10-21"},
		{"declared base chat-only gets chat path", UpstreamConfig{URL: "https://api.openai.com/v1", Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"chat": ""}}, "https://api.openai.com/v1/chat/completions"},
		{"declared messages-only gets messages path", UpstreamConfig{URL: "https://api.anthropic.com/v1", Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"messages": ""}}, "https://api.anthropic.com/v1/messages"},
		{"undeclared external stays as-is", UpstreamConfig{URL: "https://gw.example.com/custom", Source: UpstreamSourceExternal}, "https://gw.example.com/custom"},
		{"csghub unknown-task deploy defaults to chat path", UpstreamConfig{URL: "http://model-svc:8080", Source: UpstreamSourceCSGHubDeploy}, "http://model-svc:8080/v1/chat/completions"},
		{"csghub text-generation deploy defaults to chat path", UpstreamConfig{URL: "http://model-svc:8080", Source: UpstreamSourceCSGHubDeploy, Metadata: &UpstreamMetadata{InternalModelInfo: &InternalModelInfo{Task: "text-generation"}}}, "http://model-svc:8080/v1/chat/completions"},
		{"csghub multimodal deploy keeps raw URL (registry skips)", UpstreamConfig{URL: "http://model-svc:8080", Source: UpstreamSourceCSGHubDeploy, Metadata: &UpstreamMetadata{InternalModelInfo: &InternalModelInfo{Task: "text-to-image"}}}, "http://model-svc:8080"},
		{"per-protocol override is probed directly", UpstreamConfig{URL: "https://api.deepseek.com/v1", Source: UpstreamSourceExternal, SupportedEndpoints: map[ApiFormat]string{"chat": "", "messages": "https://api.deepseek.com/anthropic/messages"}}, "https://api.deepseek.com/anthropic/messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSampleEndpoint(tt.uc)
			assert.Equal(t, tt.expected, got)
		})
	}
}
