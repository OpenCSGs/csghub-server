package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpPanicsTotal prometheus.Counter

	WebhookRequestsTotal   prometheus.Counter
	WebhookRequestDuration *prometheus.HistogramVec
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
}
