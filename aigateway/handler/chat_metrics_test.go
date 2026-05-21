package handler

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	prom "opencsg.com/csghub-server/builder/prometheus"
)

func TestRecordChatAttemptMetrics(t *testing.T) {
	withTestChatMetrics(t)

	recordChatAttemptMetrics(chatAttemptReportParams{
		Phase:      chatAttemptPhaseFallback,
		Provider:   "openai",
		ModelName:  "gpt-4o",
		StatusCode: 503,
		Retryable:  true,
	})

	require.Equal(t, float64(1), counterValue(t, prom.AIGatewayChatUpstreamAttemptTotal.WithLabelValues(
		chatAttemptPhaseFallback,
		"openai",
		"gpt-4o",
		"5xx",
		"true",
	)))
}

func TestChatAttemptStatusClass(t *testing.T) {
	require.Equal(t, "2xx", chatAttemptStatusClass(200))
	require.Equal(t, "3xx", chatAttemptStatusClass(302))
	require.Equal(t, "4xx", chatAttemptStatusClass(429))
	require.Equal(t, "5xx", chatAttemptStatusClass(503))
	require.Equal(t, "unknown", chatAttemptStatusClass(0))
	require.Equal(t, "other", chatAttemptStatusClass(700))
}

func withTestChatMetrics(t *testing.T) {
	t.Helper()
	oldAttemptTotal := prom.AIGatewayChatUpstreamAttemptTotal
	t.Cleanup(func() {
		prom.AIGatewayChatUpstreamAttemptTotal = oldAttemptTotal
	})

	prom.AIGatewayChatUpstreamAttemptTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_aigateway_chat_upstream_attempt_total",
		Help: "Test chat upstream attempts",
	}, []string{"phase", "provider", "model_name", "status_class", "retryable"})
}

func counterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	metric := &dto.Metric{}
	require.NoError(t, counter.Write(metric))
	return metric.GetCounter().GetValue()
}
