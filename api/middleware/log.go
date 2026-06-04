package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/utils/trace"
)

// Status 499 is a non-standard code introduced by nginx to indicate
// "Client Closed Request" — the client disconnected before the server
// finished processing.
const StatusClientClosedRequest = 499

func Log() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.URL.Path == "/healthz" {
			ctx.Next()
			return
		}

		startTime := time.Now()
		_ = trace.GetOrGenTraceID(ctx)

		// Default to 500 so that panics (which skip the assignment after
		// ctx.Next()) are logged with the correct status. Normal requests
		// overwrite this below once ctx.Next() returns successfully.
		status := http.StatusInternalServerError

		// Use defer to guarantee the log is always emitted, even when:
		//   1. The handler panics (e.g. broken pipe on a disconnected client)
		//      — without defer, the code after ctx.Next() is skipped entirely.
		//   2. The request context is canceled by client disconnection
		//      — slog handlers backed by OTel batch processors may silently
		//        drop records that carry a canceled context.
		defer func() {
			latency := time.Since(startTime).Milliseconds()

			// If the client disconnected (timeout or explicit cancel),
			// override the logged status to 499.
			if ctx.Request.Context().Err() == context.Canceled {
				status = StatusClientClosedRequest
			}

			// Derive a non-canceled context for the log call.
			// context.WithoutCancel preserves all values (trace ID, span, etc.)
			// but removes the cancellation signal, preventing downstream slog
			// handlers (especially OTel's BatchProcessor) from dropping the
			// record.
			logCtx := ctx.Request.Context()
			if logCtx.Err() != nil {
				logCtx = context.WithoutCancel(logCtx)
			}

			slog.InfoContext(logCtx, "http request", slog.String("ip", ctx.ClientIP()),
				slog.String("method", ctx.Request.Method),
				slog.String("start_time", startTime.Format(time.RFC3339)),
				slog.Int("latency(ms)", int(latency)),
				slog.Int("status", status),
				slog.String("current_user", httpbase.GetCurrentUser(ctx)),
				slog.Any("auth_type", httpbase.GetAuthType(ctx)),
				slog.String("url", ctx.Request.URL.RequestURI()),
				slog.String("full_path", ctx.FullPath()),
			)
		}()

		ctx.Next()

		// Only reached when ctx.Next() completes without panicking.
		// Overwrite the default 500 with the actual response status.
		status = ctx.Writer.Status()
	}
}
