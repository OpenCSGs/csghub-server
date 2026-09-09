package sample

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestDefaultRegistrySupportedEndpoints(t *testing.T) {
	registry := NewDefaultRegistry()
	for _, endpoint := range []string{
		"https://api.example.com/v1/chat/completions", "https://api.example.com/v1/responses",
		"https://api.anthropic.com/v1/messages", "https://gateway.example.com/anthropic/v1/messages",
		"https://api.example.com/v1/embeddings", "https://api.example.com/rerank",
		"https://api.example.com/v1/images/generations", "https://api.example.com/v1/images/edits",
		"https://api.example.com/v1/audio/transcriptions", "https://api.example.com/v1/audio/speech",
		"https://api.example.com/v1/audio/speech/batch", "https://api.example.com/v1/audio/voices",
		"https://api.example.com/v1/video/generations",
	} {
		provider, ok := registry.Find(endpoint)
		require.True(t, ok, endpoint)
		require.NotNil(t, provider)
		for _, kind := range []types.SampleKind{types.SampleKindL7API, types.SampleKindInference} {
			policy, err := provider.ExecutionPolicy(kind)
			require.NoError(t, err)
			require.Positive(t, policy.Timeout)
		}
	}
}

func TestDefaultRegistryExecutionPolicies(t *testing.T) {
	registry := NewDefaultRegistry()
	tests := []struct {
		endpoint         string
		inferenceTimeout time.Duration
		multimodal       bool
	}{
		{endpoint: "https://api.example.com/v1/chat/completions", inferenceTimeout: defaultSampleTimeout},
		{endpoint: "https://api.example.com/v1/responses", inferenceTimeout: defaultSampleTimeout},
		{endpoint: "https://api.example.com/v1/messages", inferenceTimeout: defaultSampleTimeout},
		{endpoint: "https://api.example.com/v1/embeddings", inferenceTimeout: defaultSampleTimeout},
		{endpoint: "https://api.example.com/v1/rerank", inferenceTimeout: defaultSampleTimeout},
		{endpoint: "https://api.example.com/v1/images/generations", inferenceTimeout: longRunningInferenceTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/images/edits", inferenceTimeout: longRunningInferenceTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/audio/transcriptions", inferenceTimeout: defaultSampleTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/audio/speech", inferenceTimeout: defaultSampleTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/audio/speech/batch", inferenceTimeout: defaultSampleTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/audio/voices", inferenceTimeout: defaultSampleTimeout, multimodal: true},
		{endpoint: "https://api.example.com/v1/video/generations", inferenceTimeout: defaultSampleTimeout, multimodal: true},
	}
	for _, test := range tests {
		provider, ok := registry.Find(test.endpoint)
		require.True(t, ok)
		l7Policy, err := provider.ExecutionPolicy(types.SampleKindL7API)
		require.NoError(t, err)
		require.Equal(t, defaultSampleTimeout, l7Policy.Timeout)
		inferencePolicy, err := provider.ExecutionPolicy(types.SampleKindInference)
		require.NoError(t, err)
		require.Equal(t, test.inferenceTimeout, inferencePolicy.Timeout)
		require.Equal(t, test.multimodal, inferencePolicy.Multimodal)
	}
}

func TestDefaultRegistryRejectsUnsupportedEndpoints(t *testing.T) {
	registry := NewDefaultRegistry()
	for _, endpoint := range []string{
		"https://api.example.com/messages", "https://api.example.com/v1/messages/count_tokens",
		"https://bedrock.example.com/model/claude/invoke", "https://api.example.com/v1/ocr",
		"api.example.com/v1/messages",
	} {
		_, ok := registry.Find(endpoint)
		require.False(t, ok, endpoint)
	}
}
