package responses

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveResponsesRoutingNativeResponsesURL(t *testing.T) {
	target := "https://api.openai.com/v1/responses?api-version=preview"
	decision, err := ResolveRouting(RoutingTarget{
		ModelID: "gpt",
		Target:  target,
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeNative, decision.Mode)
	require.Equal(t, target, decision.NativeURL)
	require.Equal(t, "upstream_url_responses", decision.Reason)
}

func TestResolveResponsesRoutingChatCompletionsURL(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID: "chat-model",
		Target:  "https://cloud.infini-ai.com/maas/v1/chat/completions",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
	require.Empty(t, decision.NativeURL)
	require.Equal(t, "upstream_url_chat_completions", decision.Reason)
}

func TestResolveResponsesRoutingAzureChatCompletionsURL(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID: "azure-gpt",
		Target:  "https://opencsg-us.openai.azure.com/openai/deployments/csg-gpt4/chat/completions?api-version=2024-02-15-preview",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
}

func TestResolveResponsesRoutingUnsupportedURL(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID: "embedding-model",
		Target:  "https://cloud.infini-ai.com/maas/v1/embeddings",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeDisabled, decision.Mode)
	require.Equal(t, "unsupported_upstream_url", decision.Reason)
}

func TestResolveResponsesRoutingRejectsPartialPathSegmentMatches(t *testing.T) {
	for _, target := range []string{
		"https://api.example.com/v1/responses-extra",
		"https://api.example.com/v1/chat/completions-extra",
	} {
		decision, err := ResolveRouting(RoutingTarget{
			ModelID: "model",
			Target:  target,
		})
		require.NoError(t, err)
		require.Equal(t, ResponsesModeDisabled, decision.Mode)
		require.Equal(t, "unsupported_upstream_url", decision.Reason)
	}
}

func TestResolveResponsesRoutingInvalidURL(t *testing.T) {
	_, err := ResolveRouting(RoutingTarget{
		ModelID: "bad-model",
		Target:  "not-a-url",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot resolve responses mode")
}
