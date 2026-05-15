package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestShouldRetryChatAttempt(t *testing.T) {
	require.True(t, shouldRetryChatAttempt(http.StatusBadGateway, false))
	require.True(t, shouldRetryChatAttempt(http.StatusServiceUnavailable, false))
	require.True(t, shouldRetryChatAttempt(http.StatusGatewayTimeout, false))
	require.False(t, shouldRetryChatAttempt(http.StatusNotFound, false))
	require.False(t, shouldRetryChatAttempt(http.StatusBadRequest, false))
	require.False(t, shouldRetryChatAttempt(http.StatusBadGateway, true))
}

func TestBuildChatAttemptTargets(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"},
			{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"},
		},
		2,
	)
	require.Equal(t, []chatAttemptTarget{
		{Upstream: commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"}},
		{Upstream: commontypes.UpstreamConfig{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"}},
	}, targets)
}

func TestBuildChatAttemptTargets_RespectMaxFallbackAttempts(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true},
			{ID: 2, URL: "https://api.example.com/node-c/v1/chat/completions", Enabled: true, ModelName: "provider-model-c"},
			{ID: 3, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{ID: 4, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
			{ID: 5, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		2,
	)
	require.Equal(t, []chatAttemptTarget{
		{Upstream: commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true}},
		{Upstream: commontypes.UpstreamConfig{ID: 4, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true}},
		{Upstream: commontypes.UpstreamConfig{ID: 3, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true}},
	}, targets)
}

func TestBuildChatAttemptTargets_DisableFallbacks(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		0,
	)
	require.Equal(t, []chatAttemptTarget{
		{Upstream: commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true}},
	}, targets)
}

func TestSessionKeyDigest(t *testing.T) {
	digest1 := sessionKeyDigest("session-1")
	digest2 := sessionKeyDigest("session-1")
	digest3 := sessionKeyDigest("session-2")

	require.NotEmpty(t, digest1)
	require.Equal(t, digest1, digest2)
	require.NotEqual(t, digest1, digest3)
}
