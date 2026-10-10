package sample

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestSystemoneRegistryMatchesEndpointSuffixes(t *testing.T) {
	for _, endpoint := range []string{
		"https://openrouter.ai/api/v1/systemone",
		"http://jev.internal:8000/v1/systemone",
		"https://api.example.com/systemone",
	} {
		provider, ok := NewDefaultRegistry().Find(endpoint)
		require.True(t, ok, "endpoint %s should match the systemone provider", endpoint)
		require.True(t, provider.Supports(endpoint))
	}

	// The systemone provider itself must reject non-systemone endpoints:
	// Find() returns the first matching provider, so a chat-completions URL
	// resolves to the chat provider, never to systemone.
	systemone := newProtocolProvider(systemoneRoute, modelsL7Request(systemoneRoute), systemoneRequest, defaultSampleTimeout)
	for _, endpoint := range []string{
		"https://openrouter.ai/api/v1/chat/completions",
		"https://openrouter.ai/api/v1/systemoneextra",
		"https://openrouter.ai/api/v1/system",
	} {
		require.False(t, systemone.Supports(endpoint), "endpoint %s should not match the systemone provider", endpoint)
	}
}

func TestSystemoneInferenceSampleBody(t *testing.T) {
	endpoint := "https://openrouter.ai/api/v1/systemone"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	request, err := sampleRequestBuilder(t, provider).Build(types.SampleKindInference, sampleInput(endpoint, "typesafe/jev-1.13"))
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, endpoint, request.Endpoint)
	require.Equal(t, "application/json", request.Headers.Get("Content-Type"))

	var body types.JevRequest
	require.NoError(t, json.Unmarshal(request.Body, &body))
	require.Equal(t, "typesafe/jev-1.13", body.Model)
	require.JSONEq(t, `"hi"`, string(body.State))
	require.Len(t, body.Questions, 1)
	require.Equal(t, "noul", body.Questions["sample"].Type)
}

func TestSystemoneL7UsesModelsEndpoint(t *testing.T) {
	endpoint := "https://openrouter.ai/api/v1/systemone"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	request, err := sampleRequestBuilder(t, provider).Build(types.SampleKindL7API, sampleInput(endpoint, "model"))
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, request.Method)
	// The base URL keeps every path segment before the terminal
	// /systemone segment, so OpenRouter-style bases resolve to their
	// public models list.
	require.Equal(t, "https://openrouter.ai/api/v1/models", request.Endpoint)
}

func TestSystemoneExecutionPolicy(t *testing.T) {
	provider, ok := NewDefaultRegistry().Find("https://api.example.com/v1/systemone")
	require.True(t, ok)

	inferencePolicy, err := provider.ExecutionPolicy(types.SampleKindInference)
	require.NoError(t, err)
	require.Equal(t, defaultSampleTimeout, inferencePolicy.Timeout)
	require.False(t, inferencePolicy.Multimodal)

	l7Policy, err := provider.ExecutionPolicy(types.SampleKindL7API)
	require.NoError(t, err)
	require.Equal(t, defaultSampleTimeout, l7Policy.Timeout)
}

func TestSystemoneL7FallbackToInference(t *testing.T) {
	// An upstream without a /models endpoint: the L7 probe gets a 404 and
	// must flag the inference fallback so the health checker probes with a
	// real (minimal) System One call instead.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/systemone" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"gen-1","model":"jev","answers":{"sample":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	endpoint := server.URL + "/v1/systemone"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	input := sampleInput(endpoint, "jev")
	outcome, err := provider.Execute(t.Context(), types.SampleKindL7API, input, server.Client())
	require.NoError(t, err)
	require.True(t, outcome.InferenceFallbackRequired)

	inference, err := provider.Execute(t.Context(), types.SampleKindInference, input, server.Client())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, inference.StatusCode)
	var body map[string]any
	require.NoError(t, json.Unmarshal(inference.ResponseBody, &body))
	require.Equal(t, "gen-1", body["id"])
}

func TestSystemoneL7HealthyWhenModelsSupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	endpoint := server.URL + "/v1/systemone"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	outcome, err := provider.Execute(t.Context(), types.SampleKindL7API, sampleInput(endpoint, "jev"), server.Client())
	require.NoError(t, err)
	require.False(t, outcome.InferenceFallbackRequired)
	require.Equal(t, http.StatusOK, outcome.StatusCode)
}

func TestSystemoneInferenceTimeoutIsApplied(t *testing.T) {
	provider, ok := NewDefaultRegistry().Find("https://api.example.com/v1/systemone")
	require.True(t, ok)
	policy, err := provider.ExecutionPolicy(types.SampleKindInference)
	require.NoError(t, err)
	require.Less(t, policy.Timeout, longRunningInferenceTimeout, "systemone probes must not use the long-running timeout")
	require.Greater(t, policy.Timeout, time.Duration(0))
}
