package types_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

type testSampleProvider struct{}

func (testSampleProvider) Supports(endpoint string) bool {
	return endpoint == "https://api.example.com/v1/test"
}

func (testSampleProvider) ExecutionPolicy(types.SampleKind) (types.SampleExecutionPolicy, error) {
	return types.SampleExecutionPolicy{Timeout: 30 * time.Second}, nil
}

func (testSampleProvider) Execute(_ context.Context, _ types.SampleKind, input types.SampleInput, _ types.HTTPDoer) (*types.SampleExecutionResult, error) {
	return &types.SampleExecutionResult{
		Request: &types.SampleRequest{Endpoint: input.Endpoint, Headers: input.Headers},
	}, nil
}

func TestSampleProviderContract(t *testing.T) {
	var provider types.SampleProvider = testSampleProvider{}
	headers := make(http.Header)
	headers.Set("X-Test", "value")

	result, err := provider.Execute(context.Background(), types.SampleKindInference, types.SampleInput{
		Endpoint: "https://api.example.com/v1/test",
		Headers:  headers,
		Text:     "sample",
	}, nil)

	require.NoError(t, err)
	require.True(t, provider.Supports(result.Request.Endpoint))
	require.Equal(t, "value", result.Request.Headers.Get("X-Test"))
	policy, err := provider.ExecutionPolicy(types.SampleKindInference)
	require.NoError(t, err)
	require.Equal(t, 30*time.Second, policy.Timeout)
	require.False(t, policy.Multimodal)
}
