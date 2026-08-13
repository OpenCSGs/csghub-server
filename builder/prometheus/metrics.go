package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpPanicsTotal prometheus.Counter

	WebhookRequestsTotal   prometheus.Counter
	WebhookRequestDuration *prometheus.HistogramVec

	ClusterHeartbeatLastTimestamp *prometheus.GaugeVec

	// AIGateway upstream health metrics
	AIGatewayUpstreamHealthState *prometheus.GaugeVec
	// AIGateway upstream circuit breaker metrics
	AIGatewayUpstreamCircuitState *prometheus.GaugeVec
	// AIGateway upstream health check latency
	AIGatewayUpstreamHealthLatency *prometheus.GaugeVec
	// AIGateway chat upstream attempt count
	AIGatewayChatUpstreamAttemptTotal *prometheus.CounterVec

	// AIGateway request metrics (dashboard)
	AIGatewayRequestTotal    *prometheus.CounterVec
	AIGatewayRequestDuration *prometheus.HistogramVec
	AIGatewayTTFT            *prometheus.HistogramVec
	AIGatewayTokensTotal     *prometheus.CounterVec
	AIGatewayActiveRequests  prometheus.Gauge
)

func InitMetrics() {
	HttpPanicsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "csghub_http_panics_total",
		Help: "Total number of HTTP panics",
	})

	WebhookRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "csghub_webhook_requests_total",
		Help: "Total number of webhook requests from runner server",
	})

	WebhookRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "csghub_webhook_request_duration_seconds",
		Help:    "Duration of webhook requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint", "status"})

	ClusterHeartbeatLastTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_cluster_heartbeat_last_timestamp_seconds",
		Help: "Timestamp of the last cluster heartbeat received",
	}, []string{"cluster_id", "region"})

	// AIGateway upstream health state gauge
	// Labels: upstream_id, model_name, provider, state
	AIGatewayUpstreamHealthState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_upstream_health_state",
		Help: "Health state of aigateway upstreams (0=unhealthy, 1=degraded, 2=healthy)",
	}, []string{"upstream_id", "model_name", "provider", "state"})

	// AIGateway upstream circuit state gauge
	// Labels: upstream_id, model_name, provider, circuit_state
	AIGatewayUpstreamCircuitState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_upstream_circuit_state",
		Help: "Circuit breaker state of aigateway upstreams (0=open, 1=half_open, 2=closed)",
	}, []string{"upstream_id", "model_name", "provider", "circuit_state"})

	// AIGateway upstream health check latency
	AIGatewayUpstreamHealthLatency = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_upstream_health_latency_ms",
		Help: "Last health check latency in milliseconds for aigateway upstreams",
	}, []string{"upstream_id", "url"})

	// AIGateway chat upstream attempt count.
	AIGatewayChatUpstreamAttemptTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_chat_upstream_attempt_total",
		Help: "Total number of AIGateway chat upstream attempts",
	}, []string{"phase", "provider", "model_name", "status_class", "retryable"})

	// AIGateway request total (dashboard KPI: total requests / success rate).
	// Labels: model, provider, status_class (2xx/4xx/5xx), is_stream, error_type.
	// error_type is empty for successful requests; for failures it identifies
	// the error category (e.g. "rate_limit", "timeout", "upstream_error").
	AIGatewayRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_request_total",
		Help: "Total number of AIGateway requests",
	}, []string{"model", "provider", "status_class", "is_stream", "error_type"})

	// AIGateway request duration in milliseconds (dashboard: total latency P50/P90).
	// Labels: model, provider, is_stream.
	// Buckets are chosen with extra density in the 500–5000 ms range where most
	// inference requests land, so histogram_quantile() produces accurate P50/P90
	// values.  Long-tail buckets (2m–1h) cover slow batch/long-generation requests.
	AIGatewayRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "csghub_aigateway_request_duration_ms",
		Help:    "AIGateway request total latency in milliseconds",
		Buckets: []float64{
			50, 100, 200, 300, 500, 750, 1000, 1500, 2000, 3000, 5000, 10000, 30000, 60000,
			120000, 300000, 600000, 1200000, 1800000, 3600000, // 2m, 5m, 10m, 20m, 30m, 1h
		},
	}, []string{"model", "provider", "is_stream"})

	// AIGateway TTFT (time to first token) in milliseconds (dashboard: TTFT P50/P90).
	// Labels: model, provider, is_stream.
	// TTFT is more latency-sensitive than total duration, so extra density is
	// added in the 600–4000 ms range where streaming first-token typically lands.
	AIGatewayTTFT = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "csghub_aigateway_ttft_ms",
		Help:    "AIGateway time to first token in milliseconds (streaming only)",
		Buckets: []float64{
			50, 100, 200, 300, 500, 600, 700, 800, 900, 1000, 1200, 1500, 2000, 2500, 3000, 4000, 5000, 10000, 30000, 60000,
		},
	}, []string{"model", "provider", "is_stream"})

	// AIGateway token consumption (dashboard: total token usage).
	// Labels: model, provider, token_type (prompt/completion/cached/cache_creation).
	AIGatewayTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_tokens_total",
		Help: "Total AIGateway token consumption",
	}, []string{"model", "provider", "token_type"})

	// AIGateway active requests (dashboard: real-time concurrency).
	AIGatewayActiveRequests = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "csghub_aigateway_active_requests",
		Help: "Number of active AIGateway requests being processed",
	})
}
