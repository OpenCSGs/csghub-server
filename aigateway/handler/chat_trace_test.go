package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestExtractChatSessionIDPrecedence(t *testing.T) {
	headers := http.Header{}
	headers.Set(sessionHeaderConvID, "conversation")
	headers.Set(sessionHeaderSessionID, "session")
	headers.Set(sessionHeaderClaudeCode, "claude")

	if got := extractChatSessionID(headers); got != "claude" {
		t.Fatalf("expected claude session id, got %q", got)
	}

	headers.Del(sessionHeaderClaudeCode)
	if got := extractChatSessionID(headers); got != "session" {
		t.Fatalf("expected x-session id, got %q", got)
	}

	headers.Del(sessionHeaderSessionID)
	if got := extractChatSessionID(headers); got != "conversation" {
		t.Fatalf("expected conversation id, got %q", got)
	}
}

func TestExtractChatSessionIDTruncatesLongValue(t *testing.T) {
	longValue := strings.Repeat("a", maxSessionKeyLength+10)
	headers := http.Header{}
	headers.Set(sessionHeaderSessionID, longValue)

	got := extractChatSessionID(headers)
	if len(got) != maxSessionKeyLength {
		t.Fatalf("expected truncated session length %d, got %d", maxSessionKeyLength, len(got))
	}
}

func TestExtractChatSessionIDMissing(t *testing.T) {
	if got := extractChatSessionID(http.Header{}); got != "" {
		t.Fatalf("expected empty session id, got %q", got)
	}
}

func TestChatTraceToolsAndToolChoice(t *testing.T) {
	var req ChatCompletionRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "deepseek-v4-flash",
		"messages": [{"role": "user", "content": "weather"}],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get weather",
				"parameters": {
					"type": "object",
					"properties": {
						"city": {"type": "string"}
					},
					"required": ["city"]
				}
			}
		}],
		"tool_choice": "auto"
	}`), &req))

	tools := chatTraceTools(&req)
	require.Len(t, tools, 1)
	require.Equal(t, "get_weather", tools[0].Name)
	require.Equal(t, "function", tools[0].Type)
	require.Equal(t, "Get weather", tools[0].Description)
	require.JSONEq(t, `{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`, string(tools[0].InputSchema))

	toolChoice := chatTraceToolChoice(&req)
	require.NotNil(t, toolChoice)
	require.Equal(t, "auto", *toolChoice)
}

func TestPreflightTraceRecordsEarlyError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	ctx, preflight := startPreflightTrace(context.Background(), preflightTraceStart{
		API:       "/v1/chat/completions",
		RequestID: "req-1",
		UserID:    "user-1",
	})
	require.NotNil(t, ctx)
	preflight.RecordError(errors.New("model resolve failed"), "model_resolve")

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "aigateway.request.preflight", spans[0].Name)
	require.Equal(t, "Error", spans[0].Status.Code.String())
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.api", "/v1/chat/completions")
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.request.id", "req-1")
	requireSpanAttrValue(t, spans[0].Attributes, "user.id", "user-1")
	requireSpanAttrValue(t, spans[0].Attributes, "error.type", "model_resolve")
	requireSpanAttrValue(t, spans[0].Attributes, "error.category", "gateway_error")
}

func TestPreflightTraceRecordsResolvedTargetModel(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(oldProvider)
		_ = provider.Shutdown(context.Background())
	})

	_, preflight := startPreflightTrace(context.Background(), preflightTraceStart{
		API:       "/v1/chat/completions",
		RequestID: "req-1",
		UserID:    "user-1",
	})
	preflight.SetTargetModel("requested-model", &resolvedModelTarget{
		Model:          &types.Model{BaseModel: types.BaseModel{ID: "internal-model"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "deepseek"}},
		ModelName:      "resolved-model",
		AttemptTargets: []commontypes.UpstreamConfig{{ID: 2}, {ID: 3}},
	})
	preflight.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.request.model", "requested-model")
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.model.id", "internal-model")
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.target.provider", "deepseek")
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.target.model", "resolved-model")
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.target.has_fallbacks", true)
	requireSpanAttrValue(t, spans[0].Attributes, "aigateway.target.fallback_count", int64(2))
}

func requireSpanAttrValue(t *testing.T, attrs []attribute.KeyValue, key string, expected any) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			switch value := expected.(type) {
			case string:
				require.Equal(t, value, attr.Value.AsString())
			case bool:
				require.Equal(t, value, attr.Value.AsBool())
			case int64:
				require.Equal(t, value, attr.Value.AsInt64())
			default:
				t.Fatalf("unsupported expected value type %T", expected)
			}
			return
		}
	}
	t.Fatalf("missing span attribute %s", key)
}
