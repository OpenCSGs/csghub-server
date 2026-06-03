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
}
