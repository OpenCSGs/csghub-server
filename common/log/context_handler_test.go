package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	csgtrace "opencsg.com/csghub-server/common/utils/trace"
)

func TestContextHandler_Handle(t *testing.T) {
	// Setup buffer to capture logs
	var buf bytes.Buffer
	// Use JSONHandler to easily parse output
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		// Remove time to make testing easier or just ignore it in assertion
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	h := &ContextHandler{Handler: jsonHandler}
	logger := slog.New(h)

	// Case 1: Context with TraceID (via OTEL) and SessionID
	ctx := context.Background()

	// Generate a valid TraceID and SpanID for OTEL
	traceIDStr := "4bf92f3577b34da6a3ce929d0e0e4736"
	traceID, err := trace.TraceIDFromHex(traceIDStr)
	require.NoError(t, err)

	spanIDStr := "00f067aa0ba902b7"
	spanID, err := trace.SpanIDFromHex(spanIDStr)
	require.NoError(t, err)

	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	// Inject SpanContext into context
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)

	// Inject SessionID
	sessionID := "sess-12345"
	ctx = csgtrace.SetSessionIDInContext(ctx, sessionID)

	// Log something
	logger.ErrorContext(ctx, "test message")

	// Parse result
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify fields
	require.Equal(t, "test message", result["msg"])
	require.Equal(t, traceIDStr, result["trace_id"])
	require.Equal(t, sessionID, result["xnet_session_id"])

	// Verify source
	source, ok := result["source"].(string)
	require.True(t, ok, "source field should be present and string")

	// Check that source contains the filename
	// Since we are running the test, the source file should be this file.
	require.Contains(t, source, "context_handler_test.go")

	wd, _ := os.Getwd()
	t.Logf("Working Dir: %s", wd)
	t.Logf("Source Logged: %s", source)
}
