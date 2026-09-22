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

	// AIGateway capacity admission metrics. The admission totals counter is
	// the single source for admission_requests_total /
	// admission_accepted_total / admission_queued_total /
	// admission_rejected_total / admission_rejection_reason (all derivable
	// from the decision/reason label dimensions).
	AIGatewayAdmissionTotal           *prometheus.CounterVec
	AIGatewayAdmissionDecisionLatency *prometheus.HistogramVec
	AIGatewayAdmissionRedisErrors     *prometheus.CounterVec

	// AIGateway upstream capacity state metrics, refreshed from the
	// admission check / observation path. Observational only: gauges reflect
	// the state at the last check (request-path freshness varies; the
	// selected upstream's values include its own reservation).
	AIGatewayUpstreamCapacityCurrent *prometheus.GaugeVec
	// Count of admission checks where the candidate was blocked, per
	// dimension — the primary saturation alarm signal.
	AIGatewayUpstreamCapacityBlockedTotal *prometheus.CounterVec

	// AIGateway admission reservation queue metrics. Queue depth is the
	// number of VALID tickets in the Redis queue after expired-ticket
	// cleanup (not the number of waiting HTTP requests: disconnected or
	// crashed waiters are removed by cleanup). The wait histogram is
	// labeled by outcome (admitted/timeout/disconnected/reroute).
	AIGatewayAdmissionQueueDepth  *prometheus.GaugeVec
	AIGatewayAdmissionQueueWait   *prometheus.HistogramVec
	AIGatewayAdmissionQueueEvents *prometheus.CounterVec
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
		Name: "csghub_aigateway_request_duration_ms",
		Help: "AIGateway request total latency in milliseconds",
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
		Name: "csghub_aigateway_ttft_ms",
		Help: "AIGateway time to first token in milliseconds (streaming only)",
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

	// AIGateway capacity admission decisions.
	// Labels: decision (admit/reject/fail_open), reason (empty for admit,
	// otherwise concurrency_exceeded/rpm_exceeded/tpm_exceeded/
	// capacity_exceeded), model, provider.
	// decision=fail_open counts requests admitted because Redis was
	// unavailable — keep it out of the "accepted" series when monitoring
	// protection health.
	AIGatewayAdmissionTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_admission_requests_total",
		Help: "Total AIGateway capacity admission decisions (accepted/queued/rejected derivable from the decision label; rejection reason from the reason label)",
	}, []string{"decision", "reason", "model", "provider"})

	// AIGateway admission decision latency (single Redis round trip).
	AIGatewayAdmissionDecisionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "csghub_aigateway_admission_decision_latency_ms",
		Help:    "AIGateway capacity admission decision latency in milliseconds",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000},
	}, []string{"model"})

	// AIGateway admission Redis operation failures (per operation:
	// check/finalize/renew). These degrade to fail-open behavior.
	AIGatewayAdmissionRedisErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_admission_redis_errors_total",
		Help: "Total AIGateway capacity admission Redis operation errors",
	}, []string{"operation"})

	// AIGateway upstream capacity state (post expired-lease cleanup).
	// Labels: model, upstream_id, dimension (concurrency/rpm/tpm).
	// Values are the Redis-side counters as of the last observation; the
	// selected upstream of an admitted request reports post-acquire values.
	AIGatewayUpstreamCapacityCurrent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_upstream_capacity_current",
		Help: "Current upstream capacity usage observed by admission (observational, refreshed at check/observation time)",
	}, []string{"model", "upstream_id", "dimension"})

	// AIGateway upstream capacity blocked events per dimension — a nonzero
	// rate means the upstream saturates that dimension.
	AIGatewayUpstreamCapacityBlockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_upstream_capacity_blocked_total",
		Help: "Total admission checks where an upstream was blocked on a capacity dimension",
	}, []string{"model", "upstream_id", "dimension"})

	// AIGateway admission reservation queue depth (valid tickets after
	// expired-ticket cleanup, per upstream).
	AIGatewayAdmissionQueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "csghub_aigateway_admission_queue_depth",
		Help: "Current number of valid tickets waiting in an upstream's admission reservation queue (observational)",
	}, []string{"model", "upstream_id"})

	// AIGateway admission queue wait duration in milliseconds, labeled by
	// outcome: admitted (promoted to a lease), timeout (QueueWait exceeded,
	// HTTP 408), disconnected (client gone), reroute (upstream unavailable).
	AIGatewayAdmissionQueueWait = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csghub_aigateway_admission_queue_wait_ms",
		Help: "Time a request spent waiting in the admission reservation queue, by outcome",
		Buckets: []float64{
			10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 20000, 30000, 60000, 120000, 300000,
		},
	}, []string{"model", "outcome"})

	// AIGateway admission reservation queue lifecycle events:
	// enqueued / promoted / queue_full / queue_timeout / queue_disconnect /
	// queue_reroute / claim_lost / cancel.
	AIGatewayAdmissionQueueEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "csghub_aigateway_admission_queue_events_total",
		Help: "Total admission reservation queue lifecycle events",
	}, []string{"model", "event"})
}
