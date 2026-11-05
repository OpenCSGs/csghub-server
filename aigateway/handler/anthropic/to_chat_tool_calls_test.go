package anthropic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const toolCallsRequestBody = `{
	"model": "test-model",
	"max_tokens": 1024,
	"stream": %t,
	"messages": [{"role": "user", "content": "Weather in Paris and Tokyo?"}],
	"tools": [{"name": "get_weather", "description": "Get weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]
}`

func chatChunk(delta, finishReason string) string {
	return fmt.Sprintf(`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":%s,"finish_reason":%s}]}`, delta, finishReason)
}

func toolCallDelta(fields ...string) string {
	return `{"tool_calls":[` + strings.Join(fields, ",") + `]}`
}

func wholeToolCall(id, city string) string {
	return fmt.Sprintf(`{"id":%q,"type":"function","function":{"name":"get_weather","arguments":%q}}`, id, `{"city":"`+city+`"}`)
}

type sseEvent struct {
	Event string
	Data  map[string]any
}

func parseSSEEvents(t *testing.T, body string) []sseEvent {
	t.Helper()
	var events []sseEvent
	for _, block := range strings.Split(body, "\n\n") {
		var ev sseEvent
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev.Data))
			}
		}
		if ev.Event != "" {
			events = append(events, ev)
		}
	}
	return events
}

type streamedToolUse struct {
	Index int
	ID    string
	Name  string
	Input string
}

// streamedToolUses returns the tool_use blocks of a Messages stream in start
// order, with their concatenated partial_json, plus the final stop_reason.
func streamedToolUses(t *testing.T, body string) ([]streamedToolUse, string) {
	t.Helper()
	var blocks []*streamedToolUse
	byIndex := map[int]*streamedToolUse{}
	stopReason := ""
	for _, ev := range parseSSEEvents(t, body) {
		switch ev.Event {
		case "content_block_start":
			block, _ := ev.Data["content_block"].(map[string]any)
			if block["type"] != "tool_use" {
				continue
			}
			idx := int(ev.Data["index"].(float64))
			tu := &streamedToolUse{Index: idx, ID: block["id"].(string), Name: block["name"].(string)}
			blocks = append(blocks, tu)
			byIndex[idx] = tu
		case "content_block_delta":
			delta, _ := ev.Data["delta"].(map[string]any)
			if delta["type"] != "input_json_delta" {
				continue
			}
			tu := byIndex[int(ev.Data["index"].(float64))]
			require.NotNil(t, tu, "input_json_delta for a block that was never started")
			tu.Input += delta["partial_json"].(string)
		case "message_delta":
			delta, _ := ev.Data["delta"].(map[string]any)
			stopReason, _ = delta["stop_reason"].(string)
		}
	}
	var out []streamedToolUse
	for _, b := range blocks {
		out = append(out, *b)
	}
	return out, stopReason
}

func streamToolCalls(t *testing.T, chunks ...string) ([]streamedToolUse, string) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, fmt.Sprintf(toolCallsRequestBody, true), planner)
	require.Equal(t, 200, w.Code)
	return streamedToolUses(t, w.Body.String())
}

func TestChatResponseToMessagesResponseStopReasonFromContent(t *testing.T) {
	const toolCall = `{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}`
	tests := []struct {
		name         string
		finishReason string
		toolCalls    string
		expected     string
	}{
		{"stop with tool calls", "stop", toolCall, "tool_use"},
		{"tool_calls with tool calls", "tool_calls", toolCall, "tool_use"},
		{"length wins over tool calls", "length", toolCall, "max_tokens"},
		{"stop without tool calls", "stop", "", "end_turn"},
		{"length without tool calls", "length", "", "max_tokens"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatJSON := fmt.Sprintf(`{
				"id": "chatcmpl-1", "object": "chat.completion", "model": "gemini",
				"choices": [{"index": 0,
					"message": {"role": "assistant", "content": "", "tool_calls": [%s]},
					"finish_reason": %q}],
				"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
			}`, tt.toolCalls, tt.finishReason)
			resp, err := chatResponseToMessagesResponse([]byte(chatJSON), "claude-3")
			require.NoError(t, err)
			require.NotNil(t, resp.StopReason)
			assert.Equal(t, tt.expected, *resp.StopReason)
		})
	}
}

func TestToChatStreamStopWithToolCallsReportsToolUse(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(`{"role":"assistant"}`, `null`),
		chatChunk(toolCallDelta(wholeToolCall("call_paris", "Paris")), `"stop"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 1)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
}

func TestToChatStreamLengthWinsOverToolCalls(t *testing.T) {
	_, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(wholeToolCall("call_paris", "Paris")), `"length"`),
	)

	assert.Equal(t, "max_tokens", stopReason)
}

func TestToChatStreamStopWithoutToolCallsReportsEndTurn(t *testing.T) {
	_, stopReason := streamToolCalls(t,
		chatChunk(`{"role":"assistant","content":"Hello"}`, `null`),
		chatChunk(`{}`, `"stop"`),
	)

	assert.Equal(t, "end_turn", stopReason)
}

// Gemini's OpenAI-compatible endpoint sends each parallel call whole, without
// tool_calls[].index.
func TestToChatStreamParallelCallsWithoutIndexInOneChunk(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(wholeToolCall("call_paris", "Paris"), wholeToolCall("call_tokyo", "Tokyo")), `"stop"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 2)
	assert.Equal(t, 0, blocks[0].Index)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
	assert.Equal(t, 1, blocks[1].Index)
	assert.Equal(t, "call_tokyo", blocks[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, blocks[1].Input)
}

func TestToChatStreamParallelCallsWithoutIndexInSeparateChunks(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(wholeToolCall("call_paris", "Paris")), `null`),
		chatChunk(toolCallDelta(wholeToolCall("call_tokyo", "Tokyo")), `null`),
		chatChunk(`{}`, `"stop"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 2)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
	assert.Equal(t, "call_tokyo", blocks[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, blocks[1].Input)
	assert.Less(t, blocks[0].Index, blocks[1].Index)
}

func TestToChatStreamFragmentsWithoutIndexOrIDContinueLastCall(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(`{"id":"call_paris","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}`), `null`),
		chatChunk(toolCallDelta(`{"function":{"arguments":"\"Par"}}`), `null`),
		chatChunk(toolCallDelta(`{"function":{"arguments":"is\"}"}}`), `null`),
		chatChunk(toolCallDelta(wholeToolCall("call_tokyo", "Tokyo")), `null`),
		chatChunk(`{}`, `"stop"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 2)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
	assert.Equal(t, "call_tokyo", blocks[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, blocks[1].Input)
}

func TestToChatStreamIndexedCallsAreUnchanged(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(`{"index":0,"id":"call_paris","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}`), `null`),
		chatChunk(toolCallDelta(`{"index":1,"id":"call_tokyo","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}`), `null`),
		chatChunk(toolCallDelta(`{"index":0,"function":{"arguments":"\"Paris\"}"}}`), `null`),
		chatChunk(toolCallDelta(`{"index":1,"function":{"arguments":"\"Tokyo\"}"}}`), `null`),
		chatChunk(`{}`, `"tool_calls"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 2)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
	assert.Equal(t, "call_tokyo", blocks[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, blocks[1].Input)
}

// Some upstreams repeat the id on every fragment of one call.
func TestToChatStreamFragmentsRepeatingTheIDContinueLastCall(t *testing.T) {
	blocks, stopReason := streamToolCalls(t,
		chatChunk(toolCallDelta(`{"id":"call_paris","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}`), `null`),
		chatChunk(toolCallDelta(`{"id":"call_paris","type":"function","function":{"arguments":"\"Paris\"}"}}`), `null`),
		chatChunk(toolCallDelta(wholeToolCall("call_tokyo", "Tokyo")), `null`),
		chatChunk(`{}`, `"stop"`),
	)

	assert.Equal(t, "tool_use", stopReason)
	require.Len(t, blocks, 2)
	assert.Equal(t, "call_paris", blocks[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, blocks[0].Input)
	assert.Equal(t, "call_tokyo", blocks[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, blocks[1].Input)
}
