package handler

import (
	"context"
	"log/slog"
)

const (
	chatAttemptPhasePrimary  = "primary"
	chatAttemptPhaseFallback = "fallback"
)

// ChatAttemptFailureEvent describes one failed chat upstream attempt.
type ChatAttemptFailureEvent struct {
	Phase           string
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
}

func shouldReportChatAttemptFailure(statusCode int) bool {
	return statusCode >= 400
}
