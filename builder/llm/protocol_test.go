package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func upstream(id int64, mutate func(*database.Upstream)) *database.Upstream {
	u := &database.Upstream{
		ID:      id,
		URL:     "https://upstream.example.com",
		Enabled: true,
	}
	if mutate != nil {
		mutate(u)
	}
	return u
}

func llmConfig(modelName string, upstreams ...*database.Upstream) *database.LLMConfig {
	cfg := &database.LLMConfig{ModelName: modelName}
	for _, u := range upstreams {
		cfg.Upstreams = append(cfg.Upstreams, *u)
	}
	return cfg
}

func TestResolveUpstreamCall_URLEndingInChatPathServesChat(t *testing.T) {
	// A legacy row whose URL ends with the chat path serves chat even
	// without an explicit declaration (URL inference counts as declared):
	// the SDK base keeps the /v1 segment and appends the resource path.
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://upstream.example.com/v1/chat/completions"
		u.AuthHeader = `{"Authorization":"Bearer token123"}`
		u.ModelName = "upstream-model"
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
	assert.Equal(t, "upstream-model", call.model)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token123"}, call.headers)
}

func TestResolveUpstreamCall_BareURLWithoutDeclarationPassthrough(t *testing.T) {
	// No declaration at all (external source, no protocol path, no
	// supported_endpoints): the URL is used as-is with the openai chat
	// wire format over manual HTTP (empty protocol marks the passthrough).
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://legacy.example.com/custom/endpoint"
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormat(""), call.protocol)
	assert.Equal(t, "https://legacy.example.com/custom/endpoint", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredChatBaseURL(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatChat: ""}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredChatBaseURLWithV1(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://upstream.example.com/v1"
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatChat: ""}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_PerProtocolOverrideEndpoint(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{
			types.ApiFormatChat: "https://override.example.com/chat",
		}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	// The override acts as the SDK base URL; a non-standard tail is kept
	// with a warning and the SDK appends its resource path.
	assert.Equal(t, "https://override.example.com/chat", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredResponsesFullURL(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://upstream.example.com/v1/responses"
		u.IsFullURL = true
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatResponses, call.protocol)
	// The openai SDK base keeps the /v1 segment and appends "responses".
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredResponsesBaseURL(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatResponses: ""}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatResponses, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredMessagesBaseURL(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://api.anthropic.example.com"
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatMessages: ""}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatMessages, call.protocol)
	// The anthropic SDK appends "v1/messages" to the base itself.
	assert.Equal(t, "https://api.anthropic.example.com", call.endpoint)
}

func TestResolveUpstreamCall_DeclaredMessagesFullURLWithV1(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.URL = "https://upstream.example.com/v1/messages"
		u.IsFullURL = true
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatMessages, call.protocol)
	assert.Equal(t, "https://upstream.example.com", call.endpoint)
}

func TestResolveUpstreamCall_MultiFormatPrefersChat(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{
			types.ApiFormatResponses: "",
			types.ApiFormatMessages:  "",
		}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatResponses, call.protocol)

	cfg = llmConfig("m", upstream(2, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{
			types.ApiFormatChat:     "",
			types.ApiFormatMessages: "",
		}
	}))
	call, err = resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
}

func TestResolveUpstreamCall_SkipsSystemOneOnlyUpstream(t *testing.T) {
	cfg := llmConfig("m",
		upstream(1, func(u *database.Upstream) {
			u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatSystemOne: ""}
		}),
		upstream(2, func(u *database.Upstream) {
			u.URL = "https://chat.example.com"
			u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatChat: ""}
		}),
	)

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://chat.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_SkipsGeminiOnlyUpstream(t *testing.T) {
	cfg := llmConfig("m",
		upstream(1, func(u *database.Upstream) {
			u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatGemini: ""}
		}),
		upstream(2, func(u *database.Upstream) {
			u.URL = "https://messages.example.com"
			u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatMessages: ""}
		}),
	)

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatMessages, call.protocol)
	assert.Equal(t, "https://messages.example.com", call.endpoint)
}

func TestResolveUpstreamCall_AllUpstreamsSystemOneOnlyError(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatSystemOne: ""}
	}))

	_, err := resolveUpstreamCall(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no upstream of llm config")
}

func TestResolveUpstreamCall_PrefersHealthyUpstream(t *testing.T) {
	unhealthy := upstream(1, func(u *database.Upstream) {
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatChat: ""}
		u.HealthState = &database.AIGatewayUpstreamHealthState{HealthState: "unhealthy"}
	})
	healthy := upstream(2, func(u *database.Upstream) {
		u.URL = "https://responses.example.com"
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatResponses: ""}
	})
	cfg := llmConfig("m", unhealthy, healthy)

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatResponses, call.protocol)
	assert.Equal(t, "https://responses.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_FallsBackToUnhealthyUpstream(t *testing.T) {
	unhealthy := upstream(1, func(u *database.Upstream) {
		u.HealthState = &database.AIGatewayUpstreamHealthState{HealthState: "unhealthy"}
		u.SupportedEndpoints = map[types.ApiFormat]string{types.ApiFormatChat: ""}
	})
	cfg := llmConfig("m", unhealthy)

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_CSGHubDeployDefaultsToChat(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.Source = types.UpstreamSourceCSGHubDeploy
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, types.ApiFormatChat, call.protocol)
	assert.Equal(t, "https://upstream.example.com/v1", call.endpoint)
}

func TestResolveUpstreamCall_UpstreamModelOverridesConfigModel(t *testing.T) {
	cfg := llmConfig("config-model", upstream(1, func(u *database.Upstream) {
		u.ModelName = "upstream-model"
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, "upstream-model", call.model)

	cfg = llmConfig("config-model", upstream(2, nil))
	call, err = resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, "config-model", call.model)
}

func TestResolveUpstreamCall_HostFromMetadata(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.Metadata = &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{Host: "deploy.example.com"},
		}
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, "deploy.example.com", call.host)
}

func TestResolveUpstreamCall_PlainBearerAuthHeader(t *testing.T) {
	cfg := llmConfig("m", upstream(1, func(u *database.Upstream) {
		u.AuthHeader = "Bearer raw-token"
	}))

	call, err := resolveUpstreamCall(cfg)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Authorization": "Bearer raw-token"}, call.headers)
}
