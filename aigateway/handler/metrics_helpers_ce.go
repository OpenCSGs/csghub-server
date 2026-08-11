//go:build !ee && !saas

package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
)

// MetricsRecorderWriter is the small, read-only subset of a response/retry
// writer that the cross-modal RecordMetrics helper needs.  The CE build keeps
// the same shape as EE/SaaS so call sites compile identically.
type MetricsRecorderWriter interface {
	FirstWriteAt() time.Time
	StatusCode() int
}

// RecordMetricsParams bundles every input consumed by RecordMetrics so the signature
// stays stable as we add more fields.  Mirrors the EE/SaaS shape so tests and
// call sites keep compiling on the CE build without a single change.
type RecordMetricsParams struct {
	C              *gin.Context
	Ctx            context.Context
	FinalWrite     MetricsRecorderWriter
	Counter        token.Counter
	ProxyStartTime time.Time
}

// Stubs for ce build — metrics helpers are no-ops when the metrics feature is
// not compiled in.

func SetMetricsModelTarget(c *gin.Context, modelName, provider string, upstreamID int64, isStream bool) {
}

func SetMetricsTTFT(c *gin.Context, ttftMs int64) {
}

func SetMetricsUsageFromCounter(c *gin.Context, ctx context.Context, counter token.Counter) {
}

// RecordMetrics bundles TTFT + token usage recording for all inference
// modalities.  No-op on the CE build.
func RecordMetrics(p RecordMetricsParams) {
}
