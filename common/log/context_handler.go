package log

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/common/utils/trace"
)

// ContextHandler is a slog.Handler that adds trace ID and session ID to every log record.
type ContextHandler struct {
	slog.Handler
}

// Handle adds the trace ID and session ID to the log record before passing it to the underlying handler.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceID, _ := trace.GetTraceIDFromContext(ctx); traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}
	if sessionID := trace.GetSessionIDFromContext(ctx); sessionID != "" {
		r.AddAttrs(slog.String("xnet_session_id", sessionID))
	}
	return h.Handler.Handle(ctx, r)
}
