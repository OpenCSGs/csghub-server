package sample

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

type sampleHTTPDoer func(*http.Request) (*http.Response, error)

func (f sampleHTTPDoer) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestProtocolProviderBuildRejectsUnknownKind(t *testing.T) {
	provider := newProtocolProvider(responsesRoute, responsesRequest, responsesRequest, defaultSampleTimeout)
	_, err := provider.Build(types.SampleKind("unknown"), sampleInput("https://api.example.com/v1/responses", "model"))
	require.ErrorContains(t, err, "unsupported sample kind")
	_, err = provider.ExecutionPolicy(types.SampleKind("unknown"))
	require.ErrorContains(t, err, "unsupported sample kind")
}

func TestProtocolProviderExecute(t *testing.T) {
	endpoint := "https://api.example.com/v1/chat/completions"
	provider := newProtocolProvider(chatCompletionsRoute, modelsL7Request(chatCompletionsRoute), chatCompletionsRequest, defaultSampleTimeout)
	result, err := provider.Execute(context.Background(), types.SampleKindInference, types.SampleInput{Endpoint: endpoint, Headers: sampleInput(endpoint, "chat-model").Headers, Model: "chat-model", MaxResponseBodyBytes: 4}, sampleHTTPDoer(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, req.Method)
		require.Equal(t, endpoint, req.URL.String())
		require.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
		return &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized", Body: io.NopCloser(strings.NewReader("response body"))}, nil
	}))
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, result.StatusCode)
	require.Equal(t, "resp", string(result.ResponseBody))
	require.NoError(t, result.Error)
}

func TestProtocolProviderExecuteNetworkError(t *testing.T) {
	provider := newProtocolProvider(responsesRoute, responsesRequest, responsesRequest, defaultSampleTimeout)
	result, err := provider.Execute(context.Background(), types.SampleKindInference, sampleInput("https://api.example.com/v1/responses", "model"), sampleHTTPDoer(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}))
	require.NoError(t, err)
	require.NotNil(t, result.Request)
	require.ErrorContains(t, result.Error, "connection refused")
}

func TestProtocolProviderExecutionPolicy(t *testing.T) {
	provider := newProtocolProvider(imageGenerationsRoute, modelsL7Request(imageGenerationsRoute), imageGenerationsRequest, longRunningInferenceTimeout)
	l7Policy, err := provider.ExecutionPolicy(types.SampleKindL7API)
	require.NoError(t, err)
	require.Equal(t, defaultSampleTimeout, l7Policy.Timeout)
	inferencePolicy, err := provider.ExecutionPolicy(types.SampleKindInference)
	require.NoError(t, err)
	require.Equal(t, longRunningInferenceTimeout, inferencePolicy.Timeout)
	require.True(t, inferencePolicy.Multimodal)
}

func TestModelsL7ProviderRequestsInferenceFallbackWhenModelsEndpointIsUnsupported(t *testing.T) {
	for _, statusCode := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			endpoint := "https://api.example.com/v1/chat/completions"
			provider := newProtocolProvider(chatCompletionsRoute, modelsL7Request(chatCompletionsRoute), chatCompletionsRequest, defaultSampleTimeout)
			requests := make([]*http.Request, 0, 1)
			result, err := provider.Execute(context.Background(), types.SampleKindL7API, sampleInput(endpoint, "chat-model"), sampleHTTPDoer(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req)
				return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader("unsupported"))}, nil
			}))

			require.NoError(t, err)
			require.Equal(t, statusCode, result.StatusCode)
			require.True(t, result.InferenceFallbackRequired)
			require.Len(t, requests, 1)
			require.Equal(t, http.MethodGet, requests[0].Method)
			require.Equal(t, "https://api.example.com/v1/models", requests[0].URL.String())
			require.Equal(t, "Bearer test-token", requests[0].Header.Get("Authorization"))
		})
	}
}

func TestDefaultResponsesAndMessagesProvidersRequestInferenceFallbackWhenModelsEndpointIsUnsupported(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		modelsEndpoint string
		anthropic      bool
	}{
		{
			name:           "responses",
			endpoint:       "https://api.example.com/compatible-mode/v1/responses",
			modelsEndpoint: "https://api.example.com/compatible-mode/v1/models",
		},
		{
			name:           "messages",
			endpoint:       "https://api.example.com/anthropic/v1/messages",
			modelsEndpoint: "https://api.example.com/anthropic/v1/models",
			anthropic:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, ok := NewDefaultRegistry().Find(test.endpoint)
			require.True(t, ok)
			requests := make([]*http.Request, 0, 1)
			result, err := provider.Execute(context.Background(), types.SampleKindL7API, sampleInput(test.endpoint, "model"), sampleHTTPDoer(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req)
				return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
			}))

			require.NoError(t, err)
			require.True(t, result.InferenceFallbackRequired)
			require.Equal(t, http.StatusNotFound, result.StatusCode)
			require.Len(t, requests, 1)
			require.Equal(t, http.MethodGet, requests[0].Method)
			require.Equal(t, test.modelsEndpoint, requests[0].URL.String())
			if test.anthropic {
				require.Equal(t, defaultAnthropicVersion, requests[0].Header.Get("anthropic-version"))
			}
		})
	}
}

func TestModelsL7ProviderDoesNotFallbackForOperationalFailures(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusUnauthorized,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			callCount := 0
			provider := newProtocolProvider(chatCompletionsRoute, modelsL7Request(chatCompletionsRoute), chatCompletionsRequest, defaultSampleTimeout)
			result, err := provider.Execute(context.Background(), types.SampleKindL7API, sampleInput("https://api.example.com/v1/chat/completions", "chat-model"), sampleHTTPDoer(func(*http.Request) (*http.Response, error) {
				callCount++
				return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader("failure"))}, nil
			}))

			require.NoError(t, err)
			require.Equal(t, statusCode, result.StatusCode)
			require.Equal(t, 1, callCount)
		})
	}
}

func TestInferenceBasedL7ProviderDoesNotRepeatUnsupportedEndpoint(t *testing.T) {
	callCount := 0
	provider := newProtocolProvider(responsesRoute, responsesRequest, responsesRequest, defaultSampleTimeout)
	result, err := provider.Execute(context.Background(), types.SampleKindL7API, sampleInput("https://api.example.com/v1/responses", "model"), sampleHTTPDoer(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	}))

	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, result.StatusCode)
	require.Equal(t, 1, callCount)
}

func TestL7ProviderDoesNotFallbackForNonModelsGet(t *testing.T) {
	callCount := 0
	l7 := func(input types.SampleInput) (*types.SampleRequest, error) {
		return &types.SampleRequest{Method: http.MethodGet, Endpoint: input.Endpoint, Headers: input.Headers.Clone()}, nil
	}
	provider := newProtocolProvider(chatCompletionsRoute, l7, chatCompletionsRequest, defaultSampleTimeout)
	result, err := provider.Execute(context.Background(), types.SampleKindL7API, sampleInput("https://api.example.com/v1/chat/completions", "model"), sampleHTTPDoer(func(*http.Request) (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found"))}, nil
	}))

	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, result.StatusCode)
	require.Equal(t, 1, callCount)
}
