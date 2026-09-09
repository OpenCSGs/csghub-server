package anthropic

import (
	"bytes"
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

// adaptToChat converts the Anthropic Messages request to an OpenAI Chat
// Completions request and creates a toChatResponseWriter that will convert
// the Chat response back to Anthropic Messages format.
func (h *Handler) adaptToChat(c *gin.Context, req *types.AnthropicMessagesRequest, p *types.RequestPlan, logCapture LLMLogRecorder) (*types.AdaptResult, error) {
	// Check adapter capabilities before transforming.
	missing := checkAdapterCapabilities(req, p.UpstreamCap)
	if len(missing) > 0 {
		return nil, fmt.Errorf("unsupported_feature:%s", missing[0])
	}

	// Convert request.
	chatReq, err := messagesToChatRequest(req, p.ModelTarget.ModelName, p.UpstreamCap.Thinking)
	if err != nil {
		return nil, err
	}

	// Marshal the chat request body.
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	// Create the response writer.
	writer := newToChatResponseWriter(c.Writer, req.Stream, req.Model, p.ModelTarget.ModelName, logCapture)
	return &types.AdaptResult{Body: body, Writer: writer}, nil
}

// messagesToChatRequest builds a ChatCompletionRequest-compatible map from
// an Anthropic Messages request.  We use a map[string]any approach for
// flexibility with unknown fields and tool schemas.
func messagesToChatRequest(req *types.AnthropicMessagesRequest, modelName string, includeReasoning bool) (map[string]any, error) {
	messages, err := messagesToChatMessages(req, includeReasoning)
	if err != nil {
		return nil, err
	}
	chatReq := map[string]any{
		"model":    modelName,
		"messages": messages,
		"stream":   req.Stream,
	}
	if req.MaxTokens > 0 {
		chatReq["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		chatReq["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		chatReq["top_p"] = *req.TopP
	}
	if len(req.StopSequences) > 0 {
		chatReq["stop"] = req.StopSequences
	}
	if len(req.Tools) > 0 {
		chatReq["tools"] = messagesToolsToChatTools(req.Tools)
	}
	if len(req.ToolChoice) > 0 {
		converted, err := convertToolChoiceForChat(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		chatReq["tool_choice"] = converted
	}
	// top_k is not a standard OpenAI Chat Completions parameter; it is
	// intentionally dropped (the Responses adapter does the same).
	if req.Metadata != nil && req.Metadata.UserID != "" {
		chatReq["user"] = req.Metadata.UserID
	}
	if req.Stream {
		chatReq["stream_options"] = map[string]any{"include_usage": true}
	}
	// Thinking → reasoning_effort.  When extended thinking is enabled
	// (HasThinking() returns true for "enabled" or "adaptive"), always set
	// reasoning_effort so the upstream reasoning model actually produces
	// reasoning_content.  If BudgetTokens is 0 (e.g. thinking: {type:"enabled"}
	// without an explicit budget), default to "medium".
	//
	// When the client does not set the top-level thinking field, also check
	// extra_body.reasoning_effort as a compatibility fallback — some clients
	// (Litellm, OpenAI SDK) pass reasoning_effort that way.
	effort := ""
	if req.HasThinking() {
		if req.Thinking.BudgetTokens > 0 {
			effort = budgetToEffort(req.Thinking.BudgetTokens)
		}
	} else {
		effort = extraBodyReasoningEffort(req)
	}
	if effort != "" || req.HasThinking() {
		if effort == "" {
			effort = "medium"
		}
		chatReq["reasoning_effort"] = effort
	}

	return chatReq, nil
}

// toChatResponseWriter converts Chat Completions responses (stream and
// non-stream) to Anthropic Messages format on the fly.
type toChatResponseWriter struct {
	ginWriter    gin.ResponseWriter
	stream       bool
	publicModel  string // the model name the client sees
	upstreamModel string
	recorder     LLMLogRecorder
	decoder      streamdecoder.Decoder

	// Non-stream buffering.
	bodyBuf       bytes.Buffer
	statusCode    int
	headerWritten bool
	header        http.Header

	// Stream state machine.
	msgID          string
	started        bool
	textBlockIdx   int
	textStarted    bool
	textDone       bool
	thinkingBlockIdx int
	thinkingStarted  bool
	toolCallStates map[int]*toChatToolCallState
	nextBlockIdx   int
	inputTokens    int64
	outputTokens   int64
	cacheReadTokens int64
	stopReason     string

	// Stream error tracking.
	streamFailed    bool
	streamErrStatus int
	streamErrBody   []byte
}

type toChatToolCallState struct {
	blockIdx int
	id       string
	name     string
	started  bool
}

func newToChatResponseWriter(w gin.ResponseWriter, stream bool, publicModel, upstreamModel string, recorder LLMLogRecorder) *toChatResponseWriter {
	return &toChatResponseWriter{
		ginWriter:      w,
		stream:         stream,
		publicModel:    publicModel,
		upstreamModel:  upstreamModel,
		recorder:       recorder,
		decoder:        streamdecoder.NewSSE(),
		toolCallStates: make(map[int]*toChatToolCallState),
		msgID:          newMessagesResponseID(),
		stopReason:     "end_turn",
		header:         make(http.Header),
	}
}

func (w *toChatResponseWriter) Write(data []byte) (int, error) {
	if !w.stream {
		// Buffer non-streaming responses; Finalize() will convert and write.
		w.bodyBuf.Write(data)
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
		slog.Debug("chat stream decoder error", slog.Any("error", err))
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
		if len(event.Data) == 0 || string(event.Data) == "[DONE]" {
			continue
		}
		w.handleChatStreamChunk(event.Data)
	}
	return len(data), nil
}

func (w *toChatResponseWriter) WriteHeader(code int) {
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

func (w *toChatResponseWriter) Header() http.Header {
	if w.stream {
		return w.ginWriter.Header()
	}
	return w.header
}

func (w *toChatResponseWriter) Flush() {
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

func (w *toChatResponseWriter) StatusCode() int {
	if w.stream {
		return w.ginWriter.Status()
	}
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *toChatResponseWriter) copyHeadersToGin() {
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

func (w *toChatResponseWriter) Finalize() error {
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
			if w.textStarted && !w.textDone {
				writeSSEEventRaw(w.ginWriter, "content_block_stop", map[string]any{
					"type": "content_block_stop", "index": w.textBlockIdx,
				})
			}
// Close any open tool call blocks in deterministic order.
				// toolCallStates is a map — iteration order is random.
				// Sort by blockIdx so content_block_stop events are emitted
				// in the same order as content_block_start events.
				var sortedToolCalls []*toChatToolCallState
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
			// message_delta with stop_reason and usage.
			usageMap := map[string]any{
				"input_tokens":  w.inputTokens,
				"output_tokens": w.outputTokens,
			}
			if w.cacheReadTokens > 0 {
				usageMap["cache_read_input_tokens"] = w.cacheReadTokens
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
	// Non-stream: convert the buffered Chat response to Messages format.
	if w.bodyBuf.Len() == 0 {
		// No body — likely a connection error.
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
		writeAnthropicUpstreamError(w.ginWriter, w.statusCode, w.bodyBuf.Bytes())
		return nil
	}
	// Success path: copy upstream headers (minus Content-Length/Encoding).
	w.copyHeadersToGin()
	resp, err := chatResponseToMessagesResponse(w.bodyBuf.Bytes(), w.publicModel)
	if err != nil {
		// Conversion failure: the upstream returned a 2xx but the body is
		// not a valid Chat response (e.g. HTML, empty, or malformed JSON).
		// Return an Anthropic api_error instead of leaking the raw body.
		w.ginWriter.Header().Set("Content-Type", "application/json")
		writeAnthropicErrorJSON(w.ginWriter, http.StatusBadGateway, ErrTypeAPI,
			"failed to convert upstream response")
		return nil
	}
	w.inputTokens = int64(resp.Usage.InputTokens)
	w.outputTokens = int64(resp.Usage.OutputTokens)
	w.cacheReadTokens = int64(resp.Usage.CacheReadInputTokens)
	// Feed the recorder with the full ChatCompletion before conversion.
	if w.recorder != nil {
		var chatResp types.ChatCompletion
		if jsonErr := json.Unmarshal(w.bodyBuf.Bytes(), &chatResp); jsonErr == nil {
			w.recorder.Completion(chatResp)
		}
	}
	w.ginWriter.Header().Set("Content-Type", "application/json")
	if w.statusCode != 0 {
		w.ginWriter.WriteHeader(w.statusCode)
	} else {
		w.ginWriter.WriteHeader(http.StatusOK)
	}
	output, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, _ = w.ginWriter.Write(output)
	return nil
}

func (w *toChatResponseWriter) Usage() tokenUsage {
	return tokenUsage{
		PromptTokens:              w.inputTokens,
		CompletionTokens:          w.outputTokens,
		TotalTokens:               w.inputTokens + w.outputTokens,
		CachedPromptTokens:        w.cacheReadTokens,
		CacheCreationPromptTokens: 0,
	}
}

// handleChatStreamChunk processes a single Chat SSE chunk and emits
// corresponding Anthropic Messages SSE events.
func (w *toChatResponseWriter) handleChatStreamChunk(data []byte) {
	var chunk struct {
		ID      string `json:"id"`
		Choices []struct {
			Index        int                    `json:"index"`
			Delta        map[string]interface{} `json:"delta"`
			FinishReason *string                `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &chunk); err != nil {
		slog.Debug("chat stream chunk parse error", slog.String("error", err.Error()), slog.String("data", string(data)))
		return
	}

	// Feed the recorder a ChatCompletionChunk built from the parsed data.
	if w.recorder != nil {
		w.recorder.AppendCompletionChunk(buildChunkFromChatStreamData(chunk.ID, chunk.Choices, chunk.Usage))
	}

	// Emit message_start on first chunk.
	if !w.started {
		w.started = true
		writeSSEEventRaw(w.ginWriter, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    w.msgID,
				"type":  "message",
				"role":  "assistant",
				"model": w.publicModel,
				"content": []any{},
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{"input_tokens": 0, "output_tokens": 0},
			},
		})
	}

	// Extract usage if present.
	if chunk.Usage != nil {
		w.inputTokens = chunk.Usage.PromptTokens
		w.outputTokens = chunk.Usage.CompletionTokens
		if chunk.Usage.PromptTokensDetails != nil {
			w.cacheReadTokens = chunk.Usage.PromptTokensDetails.CachedTokens
		}
	}

	if len(chunk.Choices) == 0 {
		return
	}

	choice := chunk.Choices[0]
	delta := choice.Delta

	// Handle finish_reason.  Some upstreams send the final content delta
	// and finish_reason in the same chunk; we record the stop reason but
	// continue processing (content/tool_calls below) so the tail delta is
	// not silently dropped.  The message_stop and message_delta events are
	// emitted later in Finalize().
	if choice.FinishReason != nil {
		w.stopReason = chatFinishReasonToStopReason(*choice.FinishReason)
	}

	// Handle text content.
	if content, ok := delta["content"].(string); ok && content != "" {
		if !w.textStarted {
			w.textStarted = true
			w.textBlockIdx = w.nextBlockIdx
			w.nextBlockIdx++
			writeSSEEventRaw(w.ginWriter, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": w.textBlockIdx,
				"content_block": map[string]any{"type": "text", "text": ""},
			})
		}
		writeSSEEventRaw(w.ginWriter, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": w.textBlockIdx,
			"delta": map[string]any{"type": "text_delta", "text": content},
		})
	}

	// Handle reasoning content — emit as thinking blocks.
	if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
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
			"delta": map[string]any{"type": "thinking_delta", "thinking": reasoning},
		})
	}

	// Handle tool calls.
	if toolCalls, ok := delta["tool_calls"].([]any); ok {
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			idxFloat, _ := tcMap["index"].(float64)
			idx := int(idxFloat)
			state, exists := w.toolCallStates[idx]
			if !exists {
				state = &toChatToolCallState{
					blockIdx: w.nextBlockIdx,
				}
				w.nextBlockIdx++
				w.toolCallStates[idx] = state
			}
			if !state.started {
				state.started = true
				if fn, ok := tcMap["function"].(map[string]any); ok {
					if id, ok := tcMap["id"].(string); ok {
						state.id = id
					}
					if name, ok := fn["name"].(string); ok {
						state.name = name
					}
				}
				writeSSEEventRaw(w.ginWriter, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": state.blockIdx,
					"content_block": map[string]any{
						"type": "tool_use",
						"id":   state.id,
						"name": state.name,
						"input": map[string]any{},
					},
				})
			}
			if fn, ok := tcMap["function"].(map[string]any); ok {
				if args, ok := fn["arguments"].(string); ok && args != "" {
					writeSSEEventRaw(w.ginWriter, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": state.blockIdx,
						"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
					})
				}
			}
		}
	}
}

// buildChunkFromChatStreamData constructs a types.ChatCompletionChunk from
// the loosely-typed parsed SSE data for the LLM log recorder.
func buildChunkFromChatStreamData(id string, choices []struct {
	Index        int                    `json:"index"`
	Delta        map[string]interface{} `json:"delta"`
	FinishReason *string                `json:"finish_reason"`
}, usage *struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}) types.ChatCompletionChunk {
	c := types.ChatCompletionChunk{
		ID: id,
	}
	if len(choices) > 0 {
		ch := choices[0]
		delta := types.ChatCompletionChunkChoiceDelta{}
		if content, ok := ch.Delta["content"].(string); ok {
			delta.Content = content
		}
		if reasoning, ok := ch.Delta["reasoning_content"].(string); ok {
			delta.ReasoningContent = reasoning
		}
		if toolCalls, ok := ch.Delta["tool_calls"].([]any); ok {
			for _, tc := range toolCalls {
				tcMap, ok := tc.(map[string]any)
				if !ok {
					continue
				}
				idxFloat, _ := tcMap["index"].(float64)
				fn, _ := tcMap["function"].(map[string]any)
				fnName, _ := fn["name"].(string)
				fnArgs, _ := fn["arguments"].(string)
				tcID, _ := tcMap["id"].(string)
				delta.ToolCalls = append(delta.ToolCalls, openai.ChatCompletionChunkChoiceDeltaToolCall{
					Index: int64(idxFloat),
					ID:    tcID,
					Type:  "function",
					Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
						Name:      fnName,
						Arguments: fnArgs,
					},
				})
			}
		}
		finishReason := ""
		if ch.FinishReason != nil {
			finishReason = *ch.FinishReason
		}
		c.Choices = []types.ChatCompletionChunkChoice{{
			Index:        int64(ch.Index),
			Delta:        delta,
			FinishReason: finishReason,
		}}
	}
	if usage != nil {
		c.Usage = openai.CompletionUsage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.PromptTokens + usage.CompletionTokens,
		}
	}
	return c
}

// writeSSEEventRaw writes an SSE event directly to the gin writer.
func writeSSEEventRaw(w gin.ResponseWriter, eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// extraBodyReasoningEffort extracts reasoning_effort from extra_body in
// ExtraFields.  This is a compatibility fallback for clients that nest
// reasoning_effort inside extra_body (Litellm, OpenAI SDK pattern).
// Returns "" when not present.
func extraBodyReasoningEffort(req *types.AnthropicMessagesRequest) string {
	if req.ExtraFields == nil {
		return ""
	}
	eb, ok := req.ExtraFields["extra_body"]
	if !ok {
		return ""
	}
	var extraBody struct {
		ReasoningEffort string `json:"reasoning_effort"`
	}
	if err := json.Unmarshal(eb, &extraBody); err != nil {
		return ""
	}
	return extraBody.ReasoningEffort
}
