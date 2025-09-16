package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpPanicsTotal prometheus.Counter
)

func InitMetrics() {
	HttpPanicsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "csghub_http_panics_total",
		Help: "Total number of HTTP panics",
	})
}
