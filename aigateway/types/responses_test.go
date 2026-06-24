package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesRequestPreservesUnknownFields(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"public-model",
		"input":"hello",
		"stream":true,
		"future_field":{"enabled":true}
	}`), &req)
	require.NoError(t, err)
	require.Equal(t, "public-model", req.Model)
	require.True(t, req.Stream)
	require.NotEmpty(t, req.ExtraFields)

	data, err := json.Marshal(req)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, map[string]any{"enabled": true}, out["future_field"])
}

func TestResponsesRequestCoversOpenAICompatibleFields(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"public-model",
		"input":"hello",
		"instructions":{"type":"developer","content":"be concise"},
		"stream_options":{"include_usage":true},
		"truncation":"auto",
		"user":"user-1",
		"service_tier":"auto",
		"top_logprobs":2,
		"context_management":{"type":"auto"},
		"prompt_cache_key":"cache-key",
		"prompt_cache_retention":"24h",
		"safety_identifier":"safe-user",
		"metadata":{"k":"v"},
		"include":["reasoning.summary"]
	}`), &req)
	require.NoError(t, err)
	require.Equal(t, "user-1", req.User)
	require.Equal(t, "auto", req.ServiceTier)
	require.Equal(t, "cache-key", req.PromptCacheKey)
	require.NotEmpty(t, req.Instructions)
	require.NotEmpty(t, req.StreamOptions)
	require.NotEmpty(t, req.Metadata)
	require.NotEmpty(t, req.Include)
	require.Empty(t, req.ExtraFields)

	data, err := json.Marshal(req)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, "auto", out["service_tier"])
	require.Equal(t, map[string]any{"k": "v"}, out["metadata"])
}

func TestResponsesRequestValidate(t *testing.T) {
	require.Error(t, (ResponsesRequest{}).Validate())
	require.Error(t, (ResponsesRequest{Model: "m"}).Validate())
	require.Error(t, (ResponsesRequest{Model: "m", Input: json.RawMessage(`null`)}).Validate())
	require.NoError(t, (ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)}).Validate())
	require.NoError(t, (ResponsesRequest{Model: "m", PreviousResponseID: "resp_agw_v1.k1.x"}).Validate())
}

func TestResponsesRequestMarshalKnownFieldsWinOverExtraFields(t *testing.T) {
	req := ResponsesRequest{
		Model: "typed-model",
		Input: json.RawMessage(`"hi"`),
		ExtraFields: map[string]json.RawMessage{
			"model":        json.RawMessage(`"extra-model"`),
			"future_field": json.RawMessage(`{"enabled":true}`),
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, "typed-model", out["model"])
	require.Equal(t, map[string]any{"enabled": true}, out["future_field"])
}

func TestResponsesOutputItemMarshalPreservesExtraFields(t *testing.T) {
	item := ResponsesOutputItem{
		ID:   "item_1",
		Type: "message",
		Extra: map[string]any{
			"provider_trace_id": "trace-1",
			"type":              "overridden",
		},
	}

	data, err := json.Marshal(item)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, "message", out["type"])
	require.Equal(t, "trace-1", out["provider_trace_id"])
}

func TestResponsesContentPartOmitsEmptyRefusal(t *testing.T) {
	part := ResponsesContentPart{
		Type: "output_text",
		Text: "hello",
	}

	data, err := json.Marshal(part)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, "output_text", out["type"])
	require.Equal(t, "hello", out["text"])
	require.NotContains(t, out, "refusal")
}
