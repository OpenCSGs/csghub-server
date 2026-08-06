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
	require.Equal(t, target, decision.BackendURL)
	require.Equal(t, "upstream_url_responses", decision.Reason)
}

func TestResolveResponsesRoutingChatCompletionsURL(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID: "chat-model",
		Target:  "https://cloud.infini-ai.com/maas/v1/chat/completions",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
	require.Equal(t, "https://cloud.infini-ai.com/maas/v1/chat/completions", decision.BackendURL)
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

func TestResolveResponsesRoutingCSGHubHostedVLLMNative(t *testing.T) {
	for _, tc := range []struct {
		name             string
		runtimeFramework string
		imageID          string
	}{
		{name: "default vllm image", runtimeFramework: "vllm", imageID: "opencsghq/vllm:v0.24.0"},
		{name: "amd vllm image", runtimeFramework: "amd-vllm", imageID: "opencsghq/amd-vllm:rocm7.2.1_vllm_0.24.0"},
		{name: "future vllm image", runtimeFramework: "vllm", imageID: "opencsghq/vllm:v0.25.1"},
		{name: "future upstream major vllm image", runtimeFramework: "vllm", imageID: "opencsghq/vllm:v1.0.0"},
		{name: "multiple version tokens", runtimeFramework: "vllm", imageID: "opencsghq/vllm:v0.8.5_vllm_0.24.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := ResolveRouting(RoutingTarget{
				ModelID:          "hosted-vllm",
				Target:           "https://model.internal",
				CSGHubHosted:     true,
				RuntimeFramework: tc.runtimeFramework,
				ImageID:          tc.imageID,
			})
			require.NoError(t, err)
			require.Equal(t, ResponsesModeNative, decision.Mode)
			require.Equal(t, "https://model.internal/v1/responses", decision.BackendURL)
			require.Equal(t, "csghub_hosted_vllm_native", decision.Reason)
		})
	}
}

func TestResolveResponsesRoutingCSGHubHostedVLLMChatAdapter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		imageID string
	}{
		{name: "legacy cu118 image", imageID: "opencsghq/vllm:v0.9.2-cu118"},
		{name: "legacy dcu image", imageID: "opencsghq/vllm:v0.8.5-dtk25.04"},
		{name: "cpu image", imageID: "opencsghq/vllm-cpu:2.4"},
		{name: "nvidia image without vllm semver", imageID: "opencsghq/nvidia-vllm:25.11-py3"},
		{name: "packaging major is not upstream vllm semver", imageID: "opencsghq/vllm:2.5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := ResolveRouting(RoutingTarget{
				ModelID:          "hosted-vllm",
				Target:           "https://model.internal/",
				CSGHubHosted:     true,
				RuntimeFramework: "vllm",
				ImageID:          tc.imageID,
			})
			require.NoError(t, err)
			require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
			require.Equal(t, "https://model.internal/v1/chat/completions", decision.BackendURL)
			require.Equal(t, "csghub_hosted_vllm_chat_adapter", decision.Reason)
		})
	}
}

func TestResolveResponsesRoutingCSGHubHostedSchemeLessTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		target  string
		wantURL string
	}{
		{name: "host", target: "remote.app.internal", wantURL: "http://remote.app.internal/v1/responses"},
		{name: "host port", target: "remote.app.internal:8000", wantURL: "http://remote.app.internal:8000/v1/responses"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := ResolveRouting(RoutingTarget{
				ModelID:          "hosted-vllm",
				Target:           tc.target,
				CSGHubHosted:     true,
				RuntimeFramework: "vllm",
				ImageID:          "opencsghq/vllm:v0.24.0",
			})
			require.NoError(t, err)
			require.Equal(t, ResponsesModeNative, decision.Mode)
			require.Equal(t, tc.wantURL, decision.BackendURL)
			require.Equal(t, "csghub_hosted_vllm_native", decision.Reason)
		})
	}
}

func TestResolveResponsesRoutingCSGHubHostedSchemeLessTargetWithPath(t *testing.T) {
	_, err := ResolveRouting(RoutingTarget{
		ModelID:          "hosted-vllm",
		Target:           "cluster.local:8000/v1",
		CSGHubHosted:     true,
		RuntimeFramework: "vllm",
		ImageID:          "opencsghq/vllm:v0.24.0",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `looks like host:port with path`)
	require.Contains(t, err.Error(), `please prepend "http://"`)
}

func TestResolveResponsesRoutingCSGHubHostedSGLangChatAdapter(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID:          "hosted-sglang",
		Target:           "https://model.internal/base",
		CSGHubHosted:     true,
		RuntimeFramework: "nvidia-sglang",
		ImageID:          "opencsghq/nvidia-sglang:latest",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
	require.Equal(t, "https://model.internal/base/v1/chat/completions", decision.BackendURL)
	require.Equal(t, "csghub_hosted_sglang_chat_adapter", decision.Reason)
}

func TestResolveResponsesRoutingCSGHubHostedSGLangNative(t *testing.T) {
	for _, tc := range []struct {
		name             string
		runtimeFramework string
		imageID          string
	}{
		{name: "default sglang image", runtimeFramework: "sglang", imageID: "opencsghq/sglang:v0.5.14-cu130"},
		{name: "guard stream sglang image", runtimeFramework: "sglang-qwen3-guard-stream", imageID: "opencsghq/sglang:0.5.3rc0-qwen3-guard-stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := ResolveRouting(RoutingTarget{
				ModelID:          "hosted-sglang",
				Target:           "https://model.internal",
				CSGHubHosted:     true,
				RuntimeFramework: tc.runtimeFramework,
				ImageID:          tc.imageID,
			})
			require.NoError(t, err)
			require.Equal(t, ResponsesModeNative, decision.Mode)
			require.Equal(t, "https://model.internal/v1/responses", decision.BackendURL)
			require.Equal(t, "csghub_hosted_sglang_native", decision.Reason)
		})
	}
}

func TestResolveResponsesRoutingCSGHubHostedSGLangNonNative(t *testing.T) {
	for _, tc := range []struct {
		name    string
		imageID string
	}{
		{name: "nvidia image without sglang semver", imageID: "opencsghq/nvidia-sglang:25.10-py3"},
		{name: "old sglang image", imageID: "opencsghq/sglang:v0.4.9-cu121"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := ResolveRouting(RoutingTarget{
				ModelID:          "hosted-sglang",
				Target:           "https://model.internal",
				CSGHubHosted:     true,
				RuntimeFramework: "sglang",
				ImageID:          tc.imageID,
			})
			require.NoError(t, err)
			require.Equal(t, ResponsesModeChatAdapter, decision.Mode)
			require.Equal(t, "https://model.internal/v1/chat/completions", decision.BackendURL)
			require.Equal(t, "csghub_hosted_sglang_chat_adapter", decision.Reason)
		})
	}
}

func TestResolveResponsesRoutingCSGHubHostedUnknownRuntime(t *testing.T) {
	decision, err := ResolveRouting(RoutingTarget{
		ModelID:          "hosted-unknown",
		Target:           "https://model.internal",
		CSGHubHosted:     true,
		RuntimeFramework: "mindie",
		ImageID:          "opencsghq/mindie:latest",
	})
	require.NoError(t, err)
	require.Equal(t, ResponsesModeDisabled, decision.Mode)
	require.Equal(t, "unsupported_csghub_hosted_runtime", decision.Reason)
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
