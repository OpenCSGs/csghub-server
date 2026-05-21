package handler

import (
	"strconv"
	"strings"

	prom "opencsg.com/csghub-server/builder/prometheus"
)

func recordChatAttemptMetrics(p chatAttemptReportParams) {
	phase := normalizeMetricLabel(p.Phase)
	provider := normalizeMetricLabel(p.Provider)
	modelName := normalizeMetricLabel(p.ModelName)
	statusClass := chatAttemptStatusClass(p.StatusCode)
	retryable := strconv.FormatBool(p.Retryable)

	if prom.AIGatewayChatUpstreamAttemptTotal != nil {
		prom.AIGatewayChatUpstreamAttemptTotal.WithLabelValues(
			phase,
			provider,
			modelName,
			statusClass,
			retryable,
		).Inc()
	}
}

func chatAttemptStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	case statusCode == 0:
		return "unknown"
	default:
		return "other"
	}
}

func normalizeMetricLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
