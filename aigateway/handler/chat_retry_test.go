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
		"https://api.example.com/node-b/v1/chat/completions",
		"logical-model",
		[]commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"},
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"},
		},
		2,
	)
	require.Equal(t, []chatAttemptTarget{
		{Target: "https://api.example.com/node-b/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"}, ModelName: "provider-model-b"},
		{Target: "https://api.example.com/node-a/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"}, ModelName: "provider-model-a"},
	}, targets)
}

func TestBuildChatAttemptTargets_RespectMaxFallbackAttempts(t *testing.T) {
	targets := buildChatAttemptTargets(
		"https://api.example.com/node-d/v1/chat/completions",
		"logical-model",
		[]commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true},
			{URL: "https://api.example.com/node-c/v1/chat/completions", Enabled: true, ModelName: "provider-model-c"},
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		2,
	)
	require.Equal(t, []chatAttemptTarget{
		{Target: "https://api.example.com/node-d/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true}, ModelName: "logical-model"},
		{Target: "https://api.example.com/node-a/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true}, ModelName: "logical-model"},
		{Target: "https://api.example.com/node-b/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true}, ModelName: "logical-model"},
	}, targets)
}

func TestBuildChatAttemptTargets_DisableFallbacks(t *testing.T) {
	targets := buildChatAttemptTargets(
		"https://api.example.com/node-b/v1/chat/completions",
		"logical-model",
		[]commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		0,
	)
	require.Equal(t, []chatAttemptTarget{
		{Target: "https://api.example.com/node-b/v1/chat/completions", Endpoint: commontypes.UpstreamConfig{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true}, ModelName: "logical-model"},
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
