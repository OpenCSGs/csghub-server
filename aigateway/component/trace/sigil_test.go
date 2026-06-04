package trace

import (
	"context"
	"encoding/json"
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

func TestSigilTracerPromotesStartMetadataToSpanAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-metadata-start",
		Provider:      "opencsg",
		ResolvedModel: "image-model",
		Metadata: map[string]any{
			TraceMetadataKeyAIGatewayAPI:        "/v1/images/generations",
			TraceMetadataKeyAIGatewayModelID:    "stable-diffusion-xl-base-1.0",
			TraceMetadataKeyGenAIOutputType:     "image",
			TraceMetadataKeyImageSize:           "1024x1024",
			TraceMetadataKeyImageResponseFormat: "url",
			TraceMetadataKeyImageOutputFormat:   "png",
			TraceMetadataKeyImageN:              1,
		},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyAIGatewayAPI, "/v1/images/generations"))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyAIGatewayModelID, "stable-diffusion-xl-base-1.0"))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyGenAIOutputType, "image"))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyImageSize, "1024x1024"))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyImageResponseFormat, "url"))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyImageOutputFormat, "png"))
	require.True(t, spanHasInt64Attr(spans, TraceMetadataKeyImageN, 1))
}

func TestSigilTracerPromotesResponseMetadataBeforeEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-metadata-response",
		Provider:      "opencsg",
		ResolvedModel: "modal-model",
	})
	generation.SetResponse(types.GenerationResponse{
		Provider: "opencsg",
		Model:    "modal-model",
		Metadata: map[string]any{
			TraceMetadataKeyImageOutputCount:     2,
			TraceMetadataKeyAudioDurationSeconds: 9.2,
			TraceMetadataKeyVideoStatus:          "succeeded",
			TraceMetadataKeyVideoSeconds:         int64(5),
		},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.True(t, spanHasInt64Attr(spans, TraceMetadataKeyImageOutputCount, 2))
	require.True(t, spanHasFloat64Attr(spans, TraceMetadataKeyAudioDurationSeconds, 9.2))
	require.True(t, spanHasStringAttr(spans, TraceMetadataKeyVideoStatus, "succeeded"))
	require.True(t, spanHasInt64Attr(spans, TraceMetadataKeyVideoSeconds, 5))
}

func TestSigilTracerSkipsUnsupportedTraceMetadataAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-metadata-filter",
		Provider:      "opencsg",
		ResolvedModel: "modal-model",
		Metadata: map[string]any{
			TraceMetadataKeyVideoID:              "video_123",
			TraceMetadataKeyImageSize:            map[string]any{"width": 1024},
			TraceMetadataKeyImageOutputFormat:    "",
			TraceMetadataKeyAudioDurationSeconds: []float64{9.2},
		},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.False(t, spanHasAttr(spans, TraceMetadataKeyVideoID))
	require.False(t, spanHasAttr(spans, TraceMetadataKeyImageSize))
	require.False(t, spanHasAttr(spans, TraceMetadataKeyImageOutputFormat))
	require.False(t, spanHasAttr(spans, TraceMetadataKeyAudioDurationSeconds))
}

func TestSigilTracerRecordsEmbeddingSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tracer.Shutdown(context.Background())
	})

	dimensions := int64(1536)
	_, embedding := tracer.StartEmbedding(context.Background(), types.EmbeddingStart{
		Provider:       "openai",
		RequestModel:   "embedding-model",
		ResolvedModel:  "text-embedding-3-small",
		Dimensions:     &dimensions,
		EncodingFormat: "float",
	})
	embedding.SetResult(types.EmbeddingResult{
		InputCount:    2,
		InputTokens:   11,
		ResponseModel: "text-embedding-3-small",
		Dimensions:    &dimensions,
	})
	embedding.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "embeddings text-embedding-3-small", spans[0].Name)
	require.True(t, spanHasStringAttr(spans, "gen_ai.operation.name", "embeddings"))
	require.True(t, spanHasStringAttr(spans, "gen_ai.provider.name", "openai"))
	require.True(t, spanHasStringAttr(spans, "gen_ai.request.model", "text-embedding-3-small"))
	require.True(t, spanHasStringAttr(spans, "gen_ai.response.model", "text-embedding-3-small"))
	require.True(t, spanHasInt64Attr(spans, "gen_ai.usage.input_tokens", 11))
	require.True(t, spanHasInt64Attr(spans, "gen_ai.embeddings.input_count", 2))
	require.True(t, spanHasInt64Attr(spans, "gen_ai.embeddings.dimension.count", 1536))
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

	_, embeddingRec := tNil.StartEmbedding(ctx, types.EmbeddingStart{})
	require.Nil(t, embeddingRec)

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

func TestSigilEmbeddingRecorder_NilReceiver(t *testing.T) {
	var rNil *sigilEmbeddingRecorder

	// None of these should panic.
	rNil.SetResult(types.EmbeddingResult{InputTokens: 10})
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
	result := toSigilGenerationStart(input, sigil.GenerationModeSync, sigil.ContentCaptureModeMetadataOnly)
	require.Equal(t, "v", result.Metadata["k"])
	require.Equal(t, 3, result.Metadata["aigateway.tool_count"])
}

func TestToSigilGenerationStart_NoToolCount(t *testing.T) {
	result := toSigilGenerationStart(types.GenerationStart{RequestID: "req"}, sigil.GenerationModeStream, sigil.ContentCaptureModeMetadataOnly)
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

func TestMarshalMessagesToCompactJSON(t *testing.T) {
	t.Run("nil messages", func(t *testing.T) {
		require.Equal(t, "", marshalMessagesToCompactJSON(nil, 0))
	})
	t.Run("empty slice", func(t *testing.T) {
		require.Equal(t, "", marshalMessagesToCompactJSON([]types.GenerationMessage{}, 0))
	})
	t.Run("single message", func(t *testing.T) {
		jsonStr := marshalMessagesToCompactJSON([]types.GenerationMessage{
			{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "hello"}}},
		}, 0)
		require.Contains(t, jsonStr, `"role":"user"`)
		require.Contains(t, jsonStr, `"text":"hello"`)
	})
	t.Run("assistant with thinking", func(t *testing.T) {
		jsonStr := marshalMessagesToCompactJSON([]types.GenerationMessage{
			{
				Role: "assistant",
				Parts: []types.GenerationPart{
					{Kind: "thinking", Thinking: "step by step"},
					{Kind: "text", Text: "the answer"},
				},
			},
		}, 0)
		require.Contains(t, jsonStr, `"role":"assistant"`)
		require.Contains(t, jsonStr, `"thinking":"step by step"`)
		require.Contains(t, jsonStr, `"text":"the answer"`)
	})
	t.Run("truncates text-like fields", func(t *testing.T) {
		jsonStr := marshalMessagesToCompactJSON([]types.GenerationMessage{
			{
				Role: "assistant",
				Parts: []types.GenerationPart{
					{Kind: "thinking", Thinking: "abcdef"},
					{Kind: "text", Text: "123456"},
					{
						Kind: "tool_result",
						ToolResult: &types.GenerationToolResult{
							Name:        "lookup",
							Content:     "secret-value",
							ContentJSON: json.RawMessage(`{"keep":"complete"}`),
						},
					},
				},
			},
		}, 5)
		require.JSONEq(t, `[{"role":"assistant","parts":[{"kind":"thinking","thinking":"ab..."},{"kind":"text","text":"12..."},{"kind":"tool_result","tool_result":{"name":"lookup","content":"se...","content_json":{"keep":"complete"}}}]}]`, jsonStr)
	})
}

func TestTruncateContent(t *testing.T) {
	require.Equal(t, "hello", truncateContent("hello", 5))
	require.Equal(t, "he...", truncateContent("hello!", 5))
	require.Equal(t, "世...", truncateContent("世界您好啊", 4))
	require.Equal(t, "hello", truncateContent("hello", 0))
	require.Equal(t, "hello", truncateContent("hello", -1))
	require.Equal(t, "hel", truncateContent("hello", 3))
	require.Equal(t, "he", truncateContent("hello", 2))
}

func TestLimitInputMessages(t *testing.T) {
	messages := []types.GenerationMessage{
		{Role: "system", Parts: []types.GenerationPart{{Kind: "text", Text: "system"}}},
		{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "first"}}},
		{Role: "assistant", Parts: []types.GenerationPart{{Kind: "text", Text: "second"}}},
		{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "latest"}}},
		{Role: "tool", Parts: []types.GenerationPart{{Kind: "text", Text: "tool result"}}},
	}

	got := limitInputMessages(messages, 1)

	require.Len(t, got, 2)
	require.Equal(t, "system", got[0].Role)
	require.Equal(t, "system", got[0].Parts[0].Text)
	require.Equal(t, "user", got[1].Role)
	require.Equal(t, "latest", got[1].Parts[0].Text)
	require.NotContains(t, got, messages[4])
	require.Equal(t, messages, limitInputMessages(messages, 0))
}

func TestLimitInputMessagesWithoutSystem(t *testing.T) {
	messages := []types.GenerationMessage{
		{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "first"}}},
		{Role: "assistant", Parts: []types.GenerationPart{{Kind: "text", Text: "second"}}},
		{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "latest"}}},
		{Role: "tool", Parts: []types.GenerationPart{{Kind: "text", Text: "tool result"}}},
	}

	got := limitInputMessages(messages, 1)

	require.Len(t, got, 1)
	require.Equal(t, "user", got[0].Role)
	require.Equal(t, "latest", got[0].Parts[0].Text)
}

func TestCompactToolParametersOmitsVerboseFields(t *testing.T) {
	parameters, ok := compactToolParameters(json.RawMessage(`{
		"type": "object",
		"description": "abcdef",
		"title": "verbose title",
		"examples": [{"query": "large example"}],
		"properties": {
			"query": {
				"type": "string",
				"description": "uvwxyz",
				"default": "large default",
				"enum": ["long-enum-value"]
			}
		},
		"required": ["query"]
	}`), 5)
	require.True(t, ok)

	data, err := json.Marshal(parameters)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`, string(data))
}

func TestCompactToolParametersPreservesItemsAndAdditionalProperties(t *testing.T) {
	parameters, ok := compactToolParameters(json.RawMessage(`{
		"type": "object",
		"properties": {
			"tags": {
				"type": "array",
				"items": {
					"type": "string",
					"description": "abcdef"
				}
			},
			"meta": {
				"type": "object",
				"additionalProperties": {
					"type": "string",
					"description": "uvwxyz"
				}
			}
		}
	}`), 5)
	require.True(t, ok)

	data, err := json.Marshal(parameters)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"object","properties":{"tags":{"type":"array","items":{"type":"string"}},"meta":{"type":"object","additionalProperties":{"type":"string"}}}}`, string(data))
}

func TestSigilTracer_SetsMessageAttributesWhenContentCaptureFull(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "full"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-msg",
		Provider:      "p",
		ResolvedModel: "m",
	})
	generation.SetResponse(types.GenerationResponse{
		Provider: "p",
		Model:    "m",
		Input: []types.GenerationMessage{
			{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "hi"}}},
		},
		Output: []types.GenerationMessage{
			{Role: "assistant", Parts: []types.GenerationPart{{Kind: "text", Text: "Hello!"}}},
		},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.True(t, spanHasAttr(spans, "gen_ai.input.messages"),
		"expected gen_ai.input.messages attribute to be present")
	require.True(t, spanHasAttr(spans, "gen_ai.output.messages"),
		"expected gen_ai.output.messages attribute to be present")
}

func TestSigilTracer_SetsToolDefinitionsAndResponseAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "full"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	startedAt := time.Now()
	_, generation := tracer.StartStreamingGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-tools",
		Provider:      "p",
		ResolvedModel: "m",
		StartedAt:     startedAt,
		Tools: []types.GenerationToolDefinition{{
			Name:        "get_weather",
			Description: "Get weather",
			Type:        "function",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		}},
	})
	generation.SetFirstChunk(types.GenerationFirstChunk{At: startedAt.Add(250 * time.Millisecond)})
	generation.SetResponse(types.GenerationResponse{
		Provider:      "p",
		Model:         "m",
		ResponseID:    "chatcmpl-1",
		FinishReasons: []string{"tool_calls"},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.True(t, spanHasStringAttr(spans, "gen_ai.response.id", "chatcmpl-1"))
	require.True(t, spanHasStringSliceAttr(spans, "gen_ai.response.finish_reasons", []string{"tool_calls"}))
	require.True(t, spanHasFloat64Attr(spans, "gen_ai.response.time_to_first_chunk", 0.25))
	toolDefinitions, ok := spanStringAttr(spans, "gen_ai.tool.definitions")
	require.True(t, ok)
	require.JSONEq(t, `[{"name":"get_weather","type":"function","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]`, toolDefinitions)
}

func TestSigilTracer_TruncatesMessageAndToolDefinitionContent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "full", MaxContentLength: 5, MaxInputUserMessages: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-truncate",
		Provider:      "p",
		ResolvedModel: "m",
		Tools: []types.GenerationToolDefinition{{
			Name:        "lookup",
			Description: "abcdef",
			Type:        "function",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"abcdef"}}}`),
		}},
	})
	generation.SetResponse(types.GenerationResponse{
		Provider: "p",
		Model:    "m",
		Input: []types.GenerationMessage{
			{
				Role: "system",
				Parts: []types.GenerationPart{
					{Kind: "text", Text: "system-message"},
				},
			},
			{
				Role: "user",
				Parts: []types.GenerationPart{
					{Kind: "text", Text: "first-turn"},
				},
			},
			{
				Role: "user",
				Parts: []types.GenerationPart{
					{Kind: "text", Text: "123456"},
				},
			},
			{
				Role: "tool",
				Parts: []types.GenerationPart{
					{Kind: "text", Text: "tool-result"},
				},
			},
		},
		Output: []types.GenerationMessage{{
			Role: "assistant",
			Parts: []types.GenerationPart{
				{Kind: "thinking", Thinking: "abcdef"},
				{Kind: "text", Text: "uvwxyz"},
				{
					Kind: "tool_result",
					ToolResult: &types.GenerationToolResult{
						Name:        "lookup",
						Content:     "content-value",
						ContentJSON: json.RawMessage(`{"keep":"complete"}`),
					},
				},
			},
		}},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	inputMessages, ok := spanStringAttr(spans, "gen_ai.input.messages")
	require.True(t, ok)
	require.JSONEq(t, `[{"role":"system","parts":[{"kind":"text","text":"sy..."}]},{"role":"user","parts":[{"kind":"text","text":"12..."}]}]`, inputMessages)
	require.NotContains(t, inputMessages, "first-turn")
	require.NotContains(t, inputMessages, "tool-result")

	outputMessages, ok := spanStringAttr(spans, "gen_ai.output.messages")
	require.True(t, ok)
	require.JSONEq(t, `[{"role":"assistant","parts":[{"kind":"thinking","thinking":"ab..."},{"kind":"text","text":"uv..."},{"kind":"tool_result","tool_result":{"name":"lookup","content":"co...","content_json":{"keep":"complete"}}}]}]`, outputMessages)

	toolDefinitions, ok := spanStringAttr(spans, "gen_ai.tool.definitions")
	require.True(t, ok)
	require.JSONEq(t, `[{"name":"lookup","type":"function","parameters":{"type":"object","properties":{"query":{"type":"string"}}}}]`, toolDefinitions)

	require.False(t, spanHasAttr(spans, "sigil.gen_ai.input.messages.truncated"))
	require.False(t, spanHasAttr(spans, "sigil.gen_ai.output.messages.truncated"))
	require.False(t, spanHasAttr(spans, "sigil.gen_ai.tool.definitions.truncated"))
	require.False(t, spanHasAttr(spans, "sigil.gen_ai.input.messages.original_size"))
	require.False(t, spanHasAttr(spans, "sigil.gen_ai.output.messages.original_size"))
	require.False(t, spanHasAttr(spans, "sigil.gen_ai.tool.definitions.original_size"))
}

func TestSigilTracer_SkipsMessageAttributesWhenContentCaptureMetadataOnly(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "metadata_only"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-nomsg",
		Provider:      "p",
		ResolvedModel: "m",
	})
	generation.SetResponse(types.GenerationResponse{
		Provider: "p",
		Model:    "m",
		Input:    []types.GenerationMessage{{Role: "user", Parts: []types.GenerationPart{{Kind: "text", Text: "hi"}}}},
		Output:   []types.GenerationMessage{{Role: "assistant", Parts: []types.GenerationPart{{Kind: "text", Text: "hi"}}}},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.False(t, spanHasAttr(spans, "gen_ai.input.messages"))
	require.False(t, spanHasAttr(spans, "gen_ai.output.messages"))
}

func TestSigilTracer_SetsMessageAttributesWithoutToolContentWhenContentCaptureNoToolContent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	tracer, err := NewSigilTracer(SigilConfig{ContentCapture: "no_tool_content"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	_, generation := tracer.StartGeneration(context.Background(), types.GenerationStart{
		RequestID:     "req-notool",
		Provider:      "p",
		ResolvedModel: "m",
	})
	generation.SetResponse(types.GenerationResponse{
		Provider: "p",
		Model:    "m",
		Input: []types.GenerationMessage{{
			Role: "user",
			Parts: []types.GenerationPart{
				{Kind: "text", Text: "hi"},
				{
					Kind: "tool_result",
					ToolResult: &types.GenerationToolResult{
						ToolCallID:  "call-1",
						Name:        "lookup",
						IsError:     true,
						Content:     "secret result",
						ContentJSON: json.RawMessage(`{"secret":true}`),
					},
				},
			},
		}},
		Output: []types.GenerationMessage{{
			Role: "assistant",
			Parts: []types.GenerationPart{
				{Kind: "text", Text: "hi"},
				{
					Kind: "tool_call",
					ToolCall: &types.GenerationToolCall{
						ID:        "call-1",
						Name:      "lookup",
						InputJSON: json.RawMessage(`{"secret":true}`),
					},
				},
			},
		}},
	})
	generation.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	inputMessages, ok := spanStringAttr(spans, "gen_ai.input.messages")
	require.True(t, ok)
	require.Contains(t, inputMessages, `"text":"hi"`)
	require.Contains(t, inputMessages, `"tool_call_id":"call-1"`)
	require.Contains(t, inputMessages, `"name":"lookup"`)
	require.NotContains(t, inputMessages, "secret result")
	require.NotContains(t, inputMessages, "content_json")

	outputMessages, ok := spanStringAttr(spans, "gen_ai.output.messages")
	require.True(t, ok)
	require.Contains(t, outputMessages, `"text":"hi"`)
	require.Contains(t, outputMessages, `"id":"call-1"`)
	require.Contains(t, outputMessages, `"name":"lookup"`)
	require.NotContains(t, outputMessages, "input_json")
	require.NotContains(t, outputMessages, "secret")
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

func spanStringAttr(spans tracetest.SpanStubs, key string) (string, bool) {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key {
				return attr.Value.AsString(), true
			}
		}
	}
	return "", false
}

func spanHasInt64Attr(spans tracetest.SpanStubs, key string, want int64) bool {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key && attr.Value.AsInt64() == want {
				return true
			}
		}
	}
	return false
}

func spanHasFloat64Attr(spans tracetest.SpanStubs, key string, want float64) bool {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key && attr.Value.AsFloat64() == want {
				return true
			}
		}
	}
	return false
}

func spanHasStringSliceAttr(spans tracetest.SpanStubs, key string, want []string) bool {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key {
				got := attr.Value.AsStringSlice()
				if len(got) != len(want) {
					return false
				}
				for i := range got {
					if got[i] != want[i] {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func spanHasAttr(spans tracetest.SpanStubs, key string) bool {
	for _, span := range spans {
		for _, attr := range span.Attributes {
			if string(attr.Key) == key {
				return true
			}
		}
	}
	return false
}
