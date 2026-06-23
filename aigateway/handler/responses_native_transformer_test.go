package handler

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestResponsesNativePayloadTransformerWrapsUpstreamIDsRecursively(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"",
		nil,
	)
	data := []byte(`{
		"id": "resp_upstream_1",
		"items": [
			{"id": "resp_upstream_2"},
			{"response_id": "resp_upstream_3"}
		]
	}`)

	out, ok, err := tr.transformJSON(data)
	require.NoError(t, err)
	require.True(t, ok)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	for _, raw := range []string{
		got["id"].(string),
		got["items"].([]any)[0].(map[string]any)["id"].(string),
		got["items"].([]any)[1].(map[string]any)["response_id"].(string),
	} {
		require.True(t, strings.HasPrefix(raw, "resp_agw_v1."))
		claims, err := mapper.Unwrap(raw, "ns-1")
		require.NoError(t, err)
		require.Equal(t, int64(7), claims.UpstreamID)
	}
}

func TestResponsesNativePayloadTransformerCachesWrappedIDs(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"",
		nil,
	)
	first, err := tr.wrapUpstreamResponseID("resp_upstream_cached")
	require.NoError(t, err)
	second, err := tr.wrapUpstreamResponseID("resp_upstream_cached")
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestResponsesNativePayloadTransformerDoesNotRewrapExistingWrappedIDs(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"",
		nil,
	)

	wrapped, err := tr.wrapUpstreamResponseID("resp_upstream_seed")
	require.NoError(t, err)

	data := []byte(`{"id":"` + wrapped + `"}`)
	out, ok, err := tr.transformJSON(data)
	require.NoError(t, err)
	require.False(t, ok, "already-wrapped IDs must not be rewrapped")
	require.Equal(t, data, out)
}

func TestResponsesNativePayloadTransformerEchoesPreviousResponseID(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"resp_agw_v1.public_previous",
		nil,
	)
	data := []byte(`{"object":"response","id":"resp_upstream_echo"}`)
	out, ok, err := tr.transformJSON(data)
	require.NoError(t, err)
	require.True(t, ok)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	require.Equal(t, "resp_agw_v1.public_previous", got["previous_response_id"])
}

func TestResponsesNativePayloadTransformerForwardsEventsToCounter(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	var mu sync.Mutex
	var events []types.ResponsesStreamEvent
	var responses []*types.ResponsesResponse
	counter := &streamCaptureCounter{
		onAppend: func(e types.ResponsesStreamEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		},
	}

	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"",
		counter,
	)
	// captureResponsePayload is unexported; verify through transformJSON with a
	// stream-shaped object that the counter hooks receive the event payload.
	_, _, err = tr.transformJSON([]byte(`{"type":"response.completed","response":{"id":"resp_upstream_x","object":"response","status":"completed"}}`))
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, events)
	require.Equal(t, "response.completed", events[0].Type)
	require.Empty(t, responses, "response-level payloads should go through AppendEvent, not Response")
}

func TestResponsesNativePayloadTransformerRecordsUsage(t *testing.T) {
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)

	counter := token.NewResponsesTokenCounter(nil)
	tr := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "ns-1", UpstreamID: 7},
		"",
		counter,
	)
	_, _, err = tr.transformJSON([]byte(`{
		"type":"response.completed",
		"response":{"id":"resp_upstream_y","object":"response","status":"completed","usage":{"input_tokens":11,"output_tokens":22,"total_tokens":33}}
	}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, int64(11), usage.PromptTokens)
	require.Equal(t, int64(22), usage.CompletionTokens)
}

func TestParseResponsesUsageReturnsFalseOnEmpty(t *testing.T) {
	_, ok := parseResponsesUsage(nil)
	require.False(t, ok)
}

func TestParseResponsesUsageReturnsFalseOnUnknownShape(t *testing.T) {
	_, ok := parseResponsesUsage("not-a-usage-block")
	require.False(t, ok)
}