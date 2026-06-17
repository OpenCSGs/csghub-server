package database

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployBenchmarkTaskToResp(t *testing.T) {
	resp, err := DeployBenchmarkTaskToResp(&DeployBenchmarkTask{
		ID:               1,
		DeployID:         2,
		TriggerSource:    types.DeployBenchmarkTriggerSourceManual,
		BenchmarkType:    types.DeployBenchmarkTypeOpenAIChatCompletions,
		Status:           types.DeployBenchmarkStatusSuccess,
		RuntimeFramework: "vllm",
		Task:             "text-generation",
		Endpoint:         "http://example.com",
		ResultSummary:    types.DeployBenchmarkSummary{TPM: 12345, SuccessRate: 1},
		BenchmarkConfig:  types.DeployBenchmarkConfig{Concurrency: 2, DurationSeconds: 60},
		RequestTemplate:  types.DeployBenchmarkTemplate{APIPath: "/v1/chat/completions", Method: "POST"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.ID)
	require.Equal(t, 12345.0, resp.Summary.TPM)
	require.Equal(t, 2, resp.BenchmarkConfig.Concurrency)
}

func TestShouldSkipBenchmark(t *testing.T) {
	shouldSkip, reason := ShouldSkipBenchmark(types.DeployRequest{
		Endpoint: "http://example.com",
		Task:     string(types.Text2Image),
	})
	require.False(t, shouldSkip)
	require.Empty(t, reason)

	shouldSkip, reason = ShouldSkipBenchmark(types.DeployRequest{
		Endpoint: "http://example.com",
		Task:     string(types.Image2Image),
	})
	require.True(t, shouldSkip)
	require.Contains(t, reason, "unsupported task type")

	shouldSkip, reason = ShouldSkipBenchmark(types.DeployRequest{
		Endpoint: "http://example.com",
		Task:     string(types.Image2Video),
	})
	require.True(t, shouldSkip)
	require.Contains(t, reason, "unsupported task type")
}
