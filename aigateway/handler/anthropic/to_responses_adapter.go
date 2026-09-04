package anthropic

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/types"
)

// adaptToResponses converts the Anthropic Messages request to an OpenAI
// Responses API request and creates a toResponsesResponseWriter that will
// convert the Responses response back to Anthropic Messages format.
func (h *Handler) adaptToResponses(c *gin.Context, req *types.AnthropicMessagesRequest, p *types.RequestPlan, logCapture LLMLogRecorder) (*types.AdaptResult, error) {
	// Check adapter capabilities before transforming.
	missing := checkAdapterCapabilities(req, p.UpstreamCap)
	if len(missing) > 0 {
		return nil, fmt.Errorf("unsupported_feature:%s", missing[0])
	}

	// Convert request.
	respReq, err := messagesToResponsesRequest(req, p.ModelTarget.ModelName)
	if err != nil {
		return nil, err
	}

	// Marshal the responses request body.
	body, err := json.Marshal(respReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}

	// Create the response writer.
	writer := newToResponsesResponseWriter(c.Writer, req.Stream, req.Model, p.ModelTarget.ModelName, logCapture)
	return &types.AdaptResult{Body: body, Writer: writer}, nil
}

// toResponsesResponseWriter converts Responses API responses (stream and
// non-stream) to Anthropic Messages format on the fly.
type toResponsesResponseWriter struct {
	ginWriter     gin.ResponseWriter
	stream        bool
	publicModel   string
	upstreamModel string
	recorder      LLMLogRecorder
	decoder       streamdecoder.Decoder

	// Non-stream buffering.
	bodyBuf       []byte
	statusCode    int
	headerWritten bool
	header        http.Header

	// Stream state machine.
	msgID          string
	started        bool
	nextBlockIdx   int
	textBlockIdx   int
	textStarted    bool
	thinkingBlockIdx int
	thinkingStarted  bool
	toolCallStates map[string]*toResponsesToolCallState
	inputTokens     int64
	outputTokens    int64
	cacheReadTokens int64
	cacheCreationTokens int64
	stopReason      string

	// Stream error tracking.
	streamFailed    bool
	streamErrStatus int
	streamErrBody   []byte
}

type toResponsesToolCallState struct {
	blockIdx int
	itemID   string
	callID   string
	name     string
	started  bool
}

func newToResponsesResponseWriter(w gin.ResponseWriter, stream bool, publicModel, upstreamModel string, recorder LLMLogRecorder) *toResponsesResponseWriter {
	return &toResponsesResponseWriter{
		ginWriter:      w,
		stream:         stream,
		publicModel:    publicModel,
		upstreamModel:  upstreamModel,
		recorder:       recorder,
		decoder:        streamdecoder.NewSSE(),
		toolCallStates: make(map[string]*toResponsesToolCallState),
		msgID:          newMessagesResponseID(),
		stopReason:     "end_turn",
		header:         make(http.Header),
	}
}

func (w *toResponsesResponseWriter) Write(data []byte) (int, error) {
	if !w.stream {
		// Buffer non-streaming responses; Finalize() will convert and write.
		w.bodyBuf = append(w.bodyBuf, data...)
		return len(data), nil
	}
	// Stream: if upstream returned an error status, buffer the error body.
	if w.streamFailed {
		w.streamErrBody = append(w.streamErrBody, data...)
		return len(data), nil
	}
	// Stream: decode SSE chunks and convert.
	events, err := w.decoder.Write(data)
	if err != nil {
		slog.Debug("responses stream decoder error", slog.Any("error", err))
		return len(data), nil
	}
	for _, event := range events {
		if event.Type == "error" {
			// Upstream sent an SSE error event mid-stream.
			w.streamFailed = true
			w.streamErrStatus = http.StatusInternalServerError
			w.streamErrBody = event.Data
			return len(data), nil
		}
		if event.Type == "response.failed" {
			// Upstream reported a failure event.
			w.streamFailed = true
			w.streamErrStatus = http.StatusInternalServerError
			w.streamErrBody = event.Data
			return len(data), nil
		}
		if len(event.Data) == 0 || string(event.Data) == "[DONE]" {
			continue
		}
		w.handleResponsesStreamEvent(event.Type, event.Data)
	}
	return len(data), nil
}

func (w *toResponsesResponseWriter) WriteHeader(code int) {
	if w.stream {
		if code >= 400 {
			// Upstream error in stream mode: mark as failed.
			w.streamFailed = true
			w.streamErrStatus = code
			return
		}
		w.ginWriter.WriteHeader(code)
		return
	}
	// Non-stream: capture status code, write it in Finalize.
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
	}
}

func (w *toResponsesResponseWriter) Header() http.Header {
	if w.stream {
		return w.ginWriter.Header()
	}
	return w.header
}

func (w *toResponsesResponseWriter) Flush() {
	if w.stream {
		if w.streamFailed && !w.ginWriter.Written() {
			return
		}
		if f, ok := w.ginWriter.(http.Flusher); ok {
			f.Flush()
		}
	}
	// Non-stream: do nothing. Calling ginWriter.Flush() would trigger
	// WriteHeaderNow() and commit a default 200 status prematurely.
}

func (w *toResponsesResponseWriter) StatusCode() int {
	if w.stream {
		return w.ginWriter.Status()
	}
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *toResponsesResponseWriter) copyHeadersToGin() {
	for k, vs := range w.header {
		// Skip Content-Length and Content-Encoding: the body is transformed
		// during conversion, so the upstream values are invalid and would
		// cause "wrote more than the declared Content-Length" errors.
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Encoding") {
			continue
		}
		for _, v := range vs {
			w.ginWriter.Header().Add(k, v)
		}
	}
}

func (w *toResponsesResponseWriter) Finalize() error {
	if w.stream {
		// If the upstream failed before the SSE stream started (no data
		// written to the client), send a proper HTTP error with a JSON body.
		if w.streamFailed && !w.ginWriter.Written() {
			writeAnthropicUpstreamError(w.ginWriter, w.streamErrStatus, w.streamErrBody)
			return nil
		}
		// If the upstream failed mid-stream (SSE already started), emit an
		// Anthropic error event instead of a success completion.
		if w.streamFailed {
			writeAnthropicStreamError(w.ginWriter, w.streamErrStatus, w.streamErrBody)
			return nil
		}
		// Emit final events if streaming started.
		if w.started {
			// Close any open thinking block.
			if w.thinkingStarted {
				writeSSEEventRaw(w.ginWriter, "content_block_stop", map[string]any{
					"type": "content_block_stop", "index": w.thinkingBlockIdx,
				})
			}
			// Close any open text block.
			if w.textStarted {
				writeSSEEventRaw(w.ginWriter, "content_block_stop", map[string]any{
					"type": "content_block_stop", "index": w.textBlockIdx,
				})
			}
			// Close any open tool call blocks in deterministic order.
			// toolCallStates is a map — iteration order is random.
			// Sort by blockIdx so content_block_stop events are emitted
			// in the same order as content_block_start events.
			var sortedToolCalls []*toResponsesToolCallState
			for _, tc := range w.toolCallStates {
				if tc.started {
					sortedToolCalls = append(sortedToolCalls, tc)
				}
			}
			sort.Slice(sortedToolCalls, func(i, j int) bool {
				return sortedToolCalls[i].blockIdx < sortedToolCalls[j].blockIdx
			})
			for _, tc := range sortedToolCalls {
				writeSSEEventRaw(w.ginWriter, "content_block_stop", map[string]any{
					"type": "content_block_stop", "index": tc.blockIdx,
				})
			}
			usageMap := map[string]any{
				"input_tokens":  w.inputTokens,
				"output_tokens": w.outputTokens,
			}
			if w.cacheReadTokens > 0 {
				usageMap["cache_read_input_tokens"] = w.cacheReadTokens
			}
			if w.cacheCreationTokens > 0 {
				usageMap["cache_creation_input_tokens"] = w.cacheCreationTokens
			}
			writeSSEEventRaw(w.ginWriter, "message_delta", map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{"stop_reason": w.stopReason, "stop_sequence": nil},
				"usage": usageMap,
			})
			writeSSEEventRaw(w.ginWriter, "message_stop", map[string]any{"type": "message_stop"})
		}
		return nil
	}
	// Non-stream: convert the buffered Responses response to Messages format.
	if len(w.bodyBuf) == 0 {
		if w.statusCode >= 400 {
			writeAnthropicUpstreamError(w.ginWriter, w.statusCode, nil)
		} else {
			if w.statusCode != 0 {
				w.ginWriter.WriteHeader(w.statusCode)
			} else {
				w.ginWriter.WriteHeader(http.StatusOK)
			}
			// Force gin to commit the status code (gin's WriteHeader is lazy;
			// it only commits on Write/WriteHeaderNow).
			_, _ = w.ginWriter.Write(nil)
		}
		return nil
	}
	// If upstream returned an error status, convert to Anthropic error format.
	// Do not copy upstream headers for error responses.
	if w.statusCode >= 400 {
		writeAnthropicUpstreamError(w.ginWriter, w.statusCode, w.bodyBuf)
		return nil
	}
	// Success path: copy upstream headers (minus Content-Length/Encoding).
	w.copyHeadersToGin()
	var resp types.ResponsesResponse
	if err := json.Unmarshal(w.bodyBuf, &resp); err != nil {
		// Conversion failure: the upstream returned a 2xx but the body is
		// not a valid Responses response (e.g. HTML, empty, or malformed
		// JSON).  Return an Anthropic api_error instead of leaking the raw
		// body to the Messages client.
		w.ginWriter.Header().Set("Content-Type", "application/json")
		writeAnthropicErrorJSON(w.ginWriter, http.StatusBadGateway, ErrTypeAPI,
			"failed to convert upstream response")
		return nil
	}
	result, err := responsesResponseToMessagesResponse(&resp, w.publicModel)
	if err != nil {
		w.ginWriter.Header().Set("Content-Type", "application/json")
		writeAnthropicErrorJSON(w.ginWriter, http.StatusBadGateway, ErrTypeAPI,
			"failed to convert upstream response")
		return nil
	}
	w.inputTokens = int64(result.Usage.InputTokens)
	w.outputTokens = int64(result.Usage.OutputTokens)
	w.cacheReadTokens = int64(result.Usage.CacheReadInputTokens)
	w.cacheCreationTokens = int64(result.Usage.CacheCreationInputTokens)
	// Feed the recorder with a ChatCompletion built from the Responses response.
	if w.recorder != nil {
		w.recorder.Completion(buildChatCompletionFromResponsesResponse(&resp, w.upstreamModel))
	}
	w.ginWriter.Header().Set("Content-Type", "application/json")
	if w.statusCode != 0 {
		w.ginWriter.WriteHeader(w.statusCode)
	} else {
		w.ginWriter.WriteHeader(http.StatusOK)
	}
	output, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, _ = w.ginWriter.Write(output)
	return nil
}

func (w *toResponsesResponseWriter) Usage() tokenUsage {
	return tokenUsage{
		PromptTokens:              w.inputTokens,
		CompletionTokens:          w.outputTokens,
		TotalTokens:               w.inputTokens + w.outputTokens,
		CachedPromptTokens:        w.cacheReadTokens,
		CacheCreationPromptTokens: w.cacheCreationTokens,
	}
}

// handleResponsesStreamEvent processes a single Responses SSE event and emits
// corresponding Anthropic Messages SSE events.
func (w *toResponsesResponseWriter) handleResponsesStreamEvent(eventType string, data []byte) {
	switch eventType {
	case "response.created", "response.in_progress":
		if !w.started {
			w.started = true
			writeSSEEventRaw(w.ginWriter, "message_start", map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":            w.msgID,
					"type":          "message",
					"role":          "assistant",
					"model":         w.publicModel,
					"content":       []any{},
					"stop_reason":   nil,
					"stop_sequence": nil,
					"usage":         map[string]any{"input_tokens": 0, "output_tokens": 0},
				},
			})
		}

	case "response.output_item.added":
		var event struct {
			Item struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				CallID string `json:"call_id"`
				Name   string `json:"name"`
			} `json:"item"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Debug("responses stream output_item.added parse error", slog.String("error", err.Error()))
			return
		}
		switch event.Item.Type {
		case "message":
			// Text block will be started by content_part.added / output_text.delta.
		case "function_call":
			state := &toResponsesToolCallState{
				blockIdx: w.nextBlockIdx,
				itemID:   event.Item.ID,
				callID:   event.Item.CallID,
				name:     event.Item.Name,
				started:  true,
			}
			w.nextBlockIdx++
			// Index by item_id so function_call_arguments.delta can find it.
			w.toolCallStates[event.Item.ID] = state
			writeSSEEventRaw(w.ginWriter, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": state.blockIdx,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    state.callID,
					"name":  state.name,
					"input": map[string]any{},
				},
			})
		}

	case "response.reasoning_text.delta":
		var event struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Debug("responses stream reasoning_text.delta parse error", slog.String("error", err.Error()))
			return
		}
		if event.Delta == "" {
			return
		}
		if w.recorder != nil {
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				Model: w.upstreamModel,
				Choices: []types.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: types.ChatCompletionChunkChoiceDelta{ReasoningContent: event.Delta},
				}},
			})
		}
		if !w.thinkingStarted {
			w.thinkingStarted = true
			w.thinkingBlockIdx = w.nextBlockIdx
			w.nextBlockIdx++
			writeSSEEventRaw(w.ginWriter, "content_block_start", map[string]any{
				"type":          "content_block_start",
				"index":         w.thinkingBlockIdx,
				"content_block": map[string]any{"type": "thinking", "thinking": ""},
			})
		}
		writeSSEEventRaw(w.ginWriter, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": w.thinkingBlockIdx,
			"delta": map[string]any{"type": "thinking_delta", "thinking": event.Delta},
		})

	case "response.output_text.delta":
		var event struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Debug("responses stream output_text.delta parse error", slog.String("error", err.Error()))
			return
		}
		if event.Delta == "" {
			return
		}
		if w.recorder != nil {
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				Model: w.upstreamModel,
				Choices: []types.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: types.ChatCompletionChunkChoiceDelta{Content: event.Delta},
				}},
			})
		}
		if !w.textStarted {
			w.textStarted = true
			w.textBlockIdx = w.nextBlockIdx
			w.nextBlockIdx++
			writeSSEEventRaw(w.ginWriter, "content_block_start", map[string]any{
				"type":          "content_block_start",
				"index":         w.textBlockIdx,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
		}
		writeSSEEventRaw(w.ginWriter, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": w.textBlockIdx,
			"delta": map[string]any{"type": "text_delta", "text": event.Delta},
		})

	case "response.function_call_arguments.delta":
		var event struct {
			ItemID string `json:"item_id"`
			Delta  string `json:"delta"`
		}
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Debug("responses stream function_call_arguments.delta parse error", slog.String("error", err.Error()))
			return
		}
		if w.recorder != nil && event.Delta != "" {
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				Model: w.upstreamModel,
				Choices: []types.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: types.ChatCompletionChunkChoiceDelta{
						ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{{
							Index: 0,
							Type:  "function",
							Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
								Arguments: event.Delta,
							},
						}},
					},
				}},
			})
		}
		if state, ok := w.toolCallStates[event.ItemID]; ok && event.Delta != "" {
			writeSSEEventRaw(w.ginWriter, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": state.blockIdx,
				"delta": map[string]any{"type": "input_json_delta", "partial_json": event.Delta},
			})
		}

	case "response.completed":
		var event struct {
			Response struct {
				Status string `json:"status"`
				Usage  *struct {
					InputTokens  int64 `json:"input_tokens"`
					OutputTokens int64 `json:"output_tokens"`
					InputTokensDetails *struct {
						CachedTokens         int64 `json:"cached_tokens"`
						CachedCreationTokens int64 `json:"cached_creation_tokens"`
					} `json:"input_tokens_details"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal(data, &event); err == nil {
			if event.Response.Usage != nil {
				w.inputTokens = event.Response.Usage.InputTokens
				w.outputTokens = event.Response.Usage.OutputTokens
				if event.Response.Usage.InputTokensDetails != nil {
					w.cacheReadTokens = event.Response.Usage.InputTokensDetails.CachedTokens
					w.cacheCreationTokens = event.Response.Usage.InputTokensDetails.CachedCreationTokens
				}
			}
			w.stopReason = responsesStatusToStopReason(event.Response.Status)
		}
		if w.recorder != nil {
			finishReason := messagesStopReasonToFinishReason(w.stopReason)
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				Model: w.upstreamModel,
				Choices: []types.ChatCompletionChunkChoice{{
					Index:        0,
					FinishReason: finishReason,
				}},
				Usage: openai.CompletionUsage{
					PromptTokens:     w.inputTokens,
					CompletionTokens: w.outputTokens,
					TotalTokens:      w.inputTokens + w.outputTokens,
				},
			})
		}
		// The Finalize() method will emit message_delta and message_stop.
	}
}

// buildChatCompletionFromResponsesResponse constructs a types.ChatCompletion
// from a ResponsesResponse for the LLM log recorder.
func buildChatCompletionFromResponsesResponse(resp *types.ResponsesResponse, model string) types.ChatCompletion {
	var content string
	var toolCalls []openai.ChatCompletionMessageToolCallUnion
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					if content != "" {
						content += "\n"
					}
					content += part.Text
				case "refusal":
					if content != "" {
						content += "\n"
					}
					content += part.Refusal
				}
			}
		case "function_call":
			args := item.Arguments
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnion{
				ID:   item.CallID,
				Type: "function",
				Function: openai.ChatCompletionMessageFunctionToolCallFunction{
					Name:      item.Name,
					Arguments: args,
				},
			})
		}
	}

	finishReason := responsesStatusToFinishReason(resp.Status)
	msg := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	var promptTokens, completionTokens int64
	if resp.Usage != nil {
		promptTokens = resp.Usage.InputTokens
		completionTokens = resp.Usage.OutputTokens
	}

	return types.ChatCompletion{
		ChatCompletion: openai.ChatCompletion{
			ID:    resp.ID,
			Model: model,
			Choices: []openai.ChatCompletionChoice{{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			}},
			Usage: openai.CompletionUsage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			},
		},
	}
}

// responsesStatusToFinishReason maps a Responses API status to an OpenAI
// Chat finish_reason for the LLM log recorder.
func responsesStatusToFinishReason(status string) string {
	switch status {
	case "completed":
		return "stop"
	case "incomplete":
		return "length"
	default:
		return "stop"
	}
}
