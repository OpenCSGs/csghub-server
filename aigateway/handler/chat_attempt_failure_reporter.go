package handler

import (
	"context"
	"fmt"
	"log/slog"
	"opencsg.com/csghub-server/aigateway/types"
)

const (
	chatAttemptPhasePrimary  = "primary"
	chatAttemptPhaseFallback = "fallback"
)

// ChatAttemptFailureEvent describes one failed chat upstream attempt.
type ChatAttemptFailureEvent struct {
	Phase           string
	UpstreamID      int64
	ModelID         string
	ModelName       string
	Provider        string
	Endpoint        string
	Target          string
	SessionKeyHash  string
	StatusCode      int
	Retryable       bool
	FallbackAttempt int
}

// ChatAttemptFailureReporter handles failed chat upstream attempts.
type ChatAttemptFailureReporter interface {
	ReportChatAttemptFailure(ctx context.Context, event ChatAttemptFailureEvent) error
}

type noopChatAttemptFailureReporter struct{}

func (n noopChatAttemptFailureReporter) ReportChatAttemptFailure(_ context.Context, _ ChatAttemptFailureEvent) error {
	return nil
}

// SetChatAttemptFailureReporter replaces the chat attempt failure reporter.
// Passing nil resets reporter to the built-in no-op implementation.
func (h *OpenAIHandlerImpl) SetChatAttemptFailureReporter(reporter ChatAttemptFailureReporter) {
	if reporter == nil {
		h.chatAttemptFailureReporter = noopChatAttemptFailureReporter{}
		return
	}
	h.chatAttemptFailureReporter = reporter
}

func (h *OpenAIHandlerImpl) reportChatAttemptFailure(ctx context.Context, event ChatAttemptFailureEvent) {
	if h == nil || h.chatAttemptFailureReporter == nil {
		return
	}
	if err := h.chatAttemptFailureReporter.ReportChatAttemptFailure(ctx, event); err != nil {
		slog.WarnContext(ctx, "failed to report chat attempt failure", slog.Any("error", err), slog.Any("event", event))
	}
	if h.availabilityManager != nil && types.ShouldAttemptFailureStatus(event.StatusCode) {
		recordErr := h.availabilityManager.RecordRequestResult(
			ctx,
			event.UpstreamID,
			event.ModelID,
			false,
			fmt.Errorf("HTTP %d: %s", event.StatusCode, event.Phase),
		)
		if recordErr != nil {
			slog.WarnContext(ctx, "failed to record failure to circuit breaker",
				"error", recordErr,
				"provider", event.Provider,
				"model", event.ModelName,
				"endpoint", event.Endpoint)
		}
	}
}

// reportChatAttemptSuccess reports a successful chat upstream attempt to the
// circuit breaker so it can transition HalfOpen->Closed when appropriate.
func (h *OpenAIHandlerImpl) reportChatAttemptSuccess(ctx context.Context, upstreamID int64, modelID string) {
	if h == nil || h.availabilityManager == nil {
		return
	}
	if err := h.availabilityManager.RecordRequestResult(ctx, upstreamID, modelID, true, nil); err != nil {
		slog.WarnContext(ctx, "failed to record success to circuit breaker",
			"error", err,
			"upstream_id", upstreamID,
			"model_id", modelID)
	}
}

type chatAttemptReportParams struct {
	UpstreamID      int64
	Phase           string
	RequestModelID  string
	ModelName       string
	Provider        string
	Endpoint        string
	Target          string
	SessionKeyHash  string
	StatusCode      int
	Retryable       bool
	FallbackAttempt int
	Model           *types.Model
}

func (h *OpenAIHandlerImpl) reportChatAttemptResult(ctx context.Context, p chatAttemptReportParams) {
	recordChatAttemptMetrics(p)

	bgCtx := context.WithoutCancel(ctx)
	if types.ShouldAttemptFailureStatus(p.StatusCode) {
		event := ChatAttemptFailureEvent{
			UpstreamID:      p.UpstreamID,
			Phase:           p.Phase,
			ModelID:         resolveFailureEventModelID(p.RequestModelID, p.Model),
			ModelName:       p.ModelName,
			Provider:        p.Provider,
			Endpoint:        p.Endpoint,
			Target:          p.Target,
			SessionKeyHash:  p.SessionKeyHash,
			StatusCode:      p.StatusCode,
			Retryable:       p.Retryable,
			FallbackAttempt: p.FallbackAttempt,
		}
		go h.reportChatAttemptFailure(bgCtx, event)
	} else {
		upstreamID := p.UpstreamID
		modelID := resolveFailureEventModelID(p.RequestModelID, p.Model)
		go h.reportChatAttemptSuccess(bgCtx, upstreamID, modelID)
	}
}
