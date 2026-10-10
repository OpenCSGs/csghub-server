package jev

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// Deps bundles all dependencies required by the jev Handler.
//
// The Handler owns its Execute-phase logic (unified reverse-proxy
// pass-through, response validation and rendering, usage recording,
// metrics, LLM trace, and LLM log publishing).  Each dependency is an
// explicit interface so the Handler has no compile-time dependency on the
// handler package — the handler package provides concrete adapters.
type Deps struct {
	// ProxyExecutor executes a reverse proxy request to the upstream.
	ProxyExecutor ProxyExecutor
	// UsageRecorder records token usage for billing/metering.
	UsageRecorder UsageRecorder
	// AdmissionFinalizer closes the capacity admission lease lifecycle after
	// the upstream response (release + TPM correction with real usage).
	// Optional: nil skips admission finalization (tests).
	AdmissionFinalizer AdmissionFinalizer
	// MetricsRecorder records request-level metrics.
	MetricsRecorder MetricsRecorder
	// ErrorRecorder optionally enriches the request metrics with error
	// details (type + message) for non-2xx outcomes.  Optional: nil skips
	// error enrichment (tests).
	ErrorRecorder ErrorRecorder

	// LLMTracer is optional. When set, the Handler starts an LLM generation
	// trace before invoking and records completion/error after.
	LLMTracer LLMTracer
}

// ProxyExecutor executes a reverse proxy request to the upstream and writes
// the response through the provided writer.
type ProxyExecutor interface {
	ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter)
}

// UsageRecorder records token usage for billing/metering.
type UsageRecorder interface {
	RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, tokenID int64) error
}

// AdmissionFinalizer closes the capacity admission lease after the upstream
// response: it releases the lease and corrects the TPM reservation with the
// real usage. usage == nil means no usable usage (full reservation reclaim).
type AdmissionFinalizer interface {
	FinalizeCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *types.AdmissionUsage)
}

// MetricsRecorder records request-level token usage metrics.
type MetricsRecorder interface {
	RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64)
}

// ErrorRecorder enriches the request metrics with error details for
// non-2xx outcomes.  The error type must be a low-cardinality constant.
type ErrorRecorder interface {
	SetMetricsError(c *gin.Context, errorType, errorMessage string)
}

// LLMTracer starts an LLM generation trace.  The returned GenerationRecorder
// captures usage, response, and error information for the trace.
type LLMTracer interface {
	StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
}

// GenerationRecorder records an LLM generation trace.
type GenerationRecorder interface {
	SetUsage(usage types.TokenUsage)
	SetResponse(response types.GenerationResponse)
	SetFirstChunk(firstChunk types.GenerationFirstChunk)
	SetError(err error, code string)
	End()
}

// postProcessInput holds the data needed for async post-processing after the
// upstream call completes.  It mirrors the anthropic package's shape minus
// the streaming-only fields (System One is a synchronous protocol).
type postProcessInput struct {
	NSUUID          string
	ApiKey          string
	TokenID         int64
	Model           *types.Model
	TargetModelName string
	Usage           *tokenUsage
	Recorder        GenerationRecorder
	// Input and Output carry the trace messages built from the parsed
	// protocol request and response; the trace does not depend on the
	// (chat-shaped) training-log capture.
	Input          []types.GenerationMessage
	Output         []types.GenerationMessage
	ResponseID     string
	StatusCode     int
	AdmissionLease *types.AdmissionLease
}

// tokenUsage is the local usage struct used by the jev package.
type tokenUsage struct {
	PromptTokens       int64
	CompletionTokens   int64
	TotalTokens        int64
	CachedPromptTokens int64
}

func (u *tokenUsage) toUsage() *token.Usage {
	if u == nil {
		return nil
	}
	return &token.Usage{
		PromptTokens:       u.PromptTokens,
		CompletionTokens:   u.CompletionTokens,
		TotalTokens:        u.TotalTokens,
		CachedPromptTokens: u.CachedPromptTokens,
	}
}

// isSuccessfulStatus returns true for HTTP 2xx status codes.
func isSuccessfulStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// traceDeadline bounds the async post-process goroutine's own work.
const traceDeadline = 3 * time.Second

// finishLLMTraceWithError records an error on the generation recorder and
// ends it, so a failed exchange never leaks an open span.
func finishLLMTraceWithError(recorder GenerationRecorder, err error, code string) {
	if recorder == nil || err == nil {
		return
	}
	recorder.SetError(err, code)
	recorder.End()
}
