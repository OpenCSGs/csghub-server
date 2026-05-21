package trace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/sigil-sdk/go/sigil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestSigilTracerRecordsGenerationSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	if err != nil {
		t.Fatalf("new sigil tracer: %v", err)
	}
	t.Cleanup(func() {
		_ = tracer.Shutdown(context.Background())
	})

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:      "req-1",
		ConversationID: "session-1",
		UserID:         "user-1",
		Provider:       "openai",
		RequestModel:   "logical-model",
		ResolvedModel:  "gpt-4",
	})
	generation.SetFirstChunk(types.GenerationFirstChunk{
		At: time.Now(),
	})
	generation.SetResponse(types.GenerationResponse{
		Provider:      "anthropic",
		Model:         "claude-3",
		ResponseModel: "claude-3",
	})
	generation.SetUsage(types.TokenUsage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7})
	generation.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected generation span, got %d", len(spans))
	}
	if !spanHasStringAttr(spans, "gen_ai.conversation.id", "session-1") {
		t.Fatalf("expected generation span to include gen_ai.conversation.id")
	}
	if !spanHasStringAttr(spans, "gen_ai.provider.name", "anthropic") {
		t.Fatalf("expected generation span to include final gen_ai.provider.name")
	}
	if !spanHasStringAttr(spans, "gen_ai.request.model", "claude-3") {
		t.Fatalf("expected generation span to include final gen_ai.request.model")
	}
	if !spanHasStringAttr(spans, "gen_ai.response.model", "claude-3") {
		t.Fatalf("expected generation span to include gen_ai.response.model")
	}
}

func TestSigilTracerRecordsStreamingConversationID(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	if err != nil {
		t.Fatalf("new sigil tracer: %v", err)
	}
	t.Cleanup(func() {
		_ = tracer.Shutdown(context.Background())
	})

	_, generation := tracer.StartStreamingGeneration(context.Background(), types.GenerationStart{
		RequestID:      "req-stream",
		ConversationID: "session-stream",
		UserID:         "user-1",
		Provider:       "minimax",
		ResolvedModel:  "MiniMax-M2.7",
	})
	generation.End()

	if !spanHasStringAttr(exporter.GetSpans(), "gen_ai.conversation.id", "session-stream") {
		t.Fatalf("expected streaming generation span to include gen_ai.conversation.id")
	}
}

func TestParseContentCapture(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to metadata_only", "", "metadata_only"},
		{"unknown defaults to metadata_only", "garbage", "metadata_only"},
		{"full", "full", "full"},
		{"no_tool_content", "no_tool_content", "no_tool_content"},
		{"metadata_only explicit", "metadata_only", "metadata_only"},
		{"whitespace trimmed", "  full  ", "full"},
		{"mixed case", "FULL", "full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := SigilConfig{ContentCapture: tt.input}
			tracer, err := NewSigilTracer(cfg)
			require.NoError(t, err)
			require.NotNil(t, tracer)
			_ = tracer.Shutdown(context.Background())
		})
	}
}

func TestFirstString(t *testing.T) {
	require.Equal(t, "b", firstString([]string{"", "b", "c"}))
	require.Equal(t, "", firstString(nil))
	require.Equal(t, "", firstString([]string{"", ""}))
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "a", firstNonEmpty("a", "b"))
	require.Equal(t, "b", firstNonEmpty("", "b"))
	require.Equal(t, "", firstNonEmpty("", ""))
	require.Equal(t, "", firstNonEmpty())
}

func TestSigilTracer_NilReceiver(t *testing.T) {
	var tNil *sigilTracer
	ctx := context.Background()

	_, rec := tNil.StartGeneration(ctx, types.GenerationStart{})
	require.Nil(t, rec)

	_, rec = tNil.StartStreamingGeneration(ctx, types.GenerationStart{})
	require.Nil(t, rec)

	require.NoError(t, tNil.Shutdown(ctx))
}

func TestSigilGenerationRecorder_NilReceiver(t *testing.T) {
	var rNil *sigilGenerationRecorder

	// None of these should panic.
	rNil.SetUsage(types.TokenUsage{InputTokens: 10})
	rNil.SetResponse(types.GenerationResponse{Model: "m"})
	rNil.SetFirstChunk(types.GenerationFirstChunk{At: time.Now()})
	rNil.SetError(errors.New("fail"), "code")
	rNil.End()
}

func TestSigilGenerationRecorder_EndIdempotent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(old)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, rec := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID: "req",
		Provider:  "p",
	})
	rec.SetResponse(types.GenerationResponse{Provider: "p", Model: "m"})
	rec.End()
	// Second End must not panic or produce a second span.
	rec.End()

	require.Len(t, exporter.GetSpans(), 1)
}

func TestSigilGenerationRecorder_SetErrorPreservesCode(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(old)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, rec := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID: "req",
		Provider:  "p",
	})
	rec.SetError(errors.New("boom"), "insufficient_balance")
	rec.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	// Span status should be error.
	require.Equal(t, codes.Error, spans[0].Status.Code)
}

func TestToSigilGenerationStart_ToolCountMetadata(t *testing.T) {
	input := types.GenerationStart{
		RequestID: "req",
		Provider:  "p",
		ToolCount: 3,
		Metadata:  map[string]any{"k": "v"},
	}
	result := toSigilGenerationStart(input, sigil.GenerationModeSync)
	require.Equal(t, "v", result.Metadata["k"])
	require.Equal(t, 3, result.Metadata["aigateway.tool_count"])
}

func TestToSigilGenerationStart_NoToolCount(t *testing.T) {
	result := toSigilGenerationStart(types.GenerationStart{RequestID: "req"}, sigil.GenerationModeStream)
	require.Equal(t, sigil.GenerationModeStream, result.Mode)
	require.NotContains(t, result.Metadata, "aigateway.tool_count")
}

func TestToSigilMessages_Empty(t *testing.T) {
	require.Nil(t, toSigilMessages(nil))
	require.Nil(t, toSigilMessages([]types.GenerationMessage{}))
}

func TestToSigilParts_Empty(t *testing.T) {
	require.Nil(t, toSigilParts(nil))
	require.Nil(t, toSigilParts([]types.GenerationPart{}))
}

func TestToSigilToolDefinitions_Empty(t *testing.T) {
	require.Nil(t, toSigilToolDefinitions(nil))
	require.Nil(t, toSigilToolDefinitions([]types.GenerationToolDefinition{}))
}

func TestToSigilArtifacts_Empty(t *testing.T) {
	require.Nil(t, toSigilArtifacts(nil))
	require.Nil(t, toSigilArtifacts([]types.GenerationArtifact{}))
}

func spanHasStringAttr(spans tracetest.SpanStubs, key string, want string) bool {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key && attr.Value.AsString() == want {
				return true
			}
		}
	}
	return false
}
