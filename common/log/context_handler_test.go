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

func TestContextHandler_WithAttrs(t *testing.T) {
	// Setup buffer to capture logs
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	h := &ContextHandler{Handler: jsonHandler}
	logger := slog.New(h)

	// Create a logger with With() - this should preserve ContextHandler
	logWithAttrs := logger.With("model_name", "gpt-4", "user", "test-user")

	// Setup context with trace ID
	ctx := context.Background()
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
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)

	// Log using the logger created with With()
	logWithAttrs.InfoContext(ctx, "test with attrs", slog.Int("status", 200))

	// Parse result
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify trace_id is present (this is the key test)
	require.Equal(t, traceIDStr, result["trace_id"], "trace_id should be present when using slog.With()")

	// Verify With() attributes are present
	require.Equal(t, "gpt-4", result["model_name"])
	require.Equal(t, "test-user", result["user"])

	// Verify inline attributes
	require.Equal(t, float64(200), result["status"])

	// Verify message
	require.Equal(t, "test with attrs", result["msg"])
}

func TestContextHandler_WithGroup(t *testing.T) {
	// Setup buffer to capture logs
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	h := &ContextHandler{Handler: jsonHandler}
	logger := slog.New(h)

	// Create a logger with WithGroup() - this should preserve ContextHandler
	logWithGroup := logger.WithGroup("request")

	// Setup context with trace ID
	ctx := context.Background()
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
	ctx = trace.ContextWithSpanContext(ctx, spanCtx)

	// Log using the logger created with WithGroup()
	logWithGroup.InfoContext(ctx, "test with group", slog.String("method", "POST"))

	// Parse result
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)

	// Verify group structure
	requestGroup, ok := result["request"].(map[string]interface{})
	require.True(t, ok, "request group should be present")
	require.Equal(t, "POST", requestGroup["method"])

	// trace_id is nested inside the group when using WithGroup()
	// This is expected slog behavior - r.AddAttrs() adds to the current group scope
	require.Equal(t, traceIDStr, requestGroup["trace_id"], "trace_id should be present inside the group when using slog.WithGroup()")

	// Verify message
	require.Equal(t, "test with group", result["msg"])
}
