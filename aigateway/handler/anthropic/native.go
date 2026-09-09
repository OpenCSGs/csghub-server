package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/types"
)

// adaptNative prepares the request for native passthrough to an upstream
// that speaks the Anthropic Messages protocol.  It remaps the model name
// and creates a nativeResponseWriter for usage extraction.
func (h *Handler) adaptNative(c *gin.Context, req *types.AnthropicMessagesRequest, p *types.RequestPlan, logCapture LLMLogRecorder) (*types.AdaptResult, error) {
	reqCopy := *req
	reqCopy.Model = p.ModelTarget.ModelName
	if reqCopy.Model == "" {
		reqCopy.Model = req.Model
	}

	body, err := json.Marshal(&reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	writer := newNativeResponseWriter(c.Writer, req.Stream, p.ModelTarget.ModelName, logCapture)
	return &types.AdaptResult{Body: body, Writer: writer}, nil
}


// nativeResponseWriter wraps the gin response writer for native passthrough.
// For streaming responses, it passes SSE events through while extracting usage.
// For non-streaming responses, it buffers the body and extracts usage on Finalize.
type nativeResponseWriter struct {
	ginWriter     gin.ResponseWriter
	stream        bool
	model         string // public model name (what the client sees)
	recorder      LLMLogRecorder
	bodyBuf       bytes.Buffer
	decoder       streamdecoder.Decoder
	inputTokens   int64
	outputTokens  int64
	// Cache token fields for billing/metering.
	cacheReadTokens     int64
	cacheCreationTokens int64
	headerWritten bool
	statusCode    int
	header        http.Header
	// Stream error tracking.
	streamFailed   bool
	streamErrStatus int
	streamErrBody  []byte
	// response ID for recorder chunks (captured from message_start).
	respID string
}

func newNativeResponseWriter(w gin.ResponseWriter, stream bool, model string, recorder LLMLogRecorder) *nativeResponseWriter {
	return &nativeResponseWriter{
		ginWriter: w,
		stream:    stream,
		model:     model,
		recorder:  recorder,
		decoder:   streamdecoder.NewSSE(),
		header:    make(http.Header),
	}
}

func (w *nativeResponseWriter) Write(data []byte) (int, error) {
	if !w.stream {
		// Buffer non-streaming responses for usage extraction and model remap.
		w.bodyBuf.Write(data)
		return len(data), nil
	}
	// Stream: if upstream returned an error status, buffer the error body
	// instead of passing it through as a fake success stream.
	if w.streamFailed {
		w.streamErrBody = append(w.streamErrBody, data...)
		return len(data), nil
	}
	// Decode SSE events to extract usage (message_start.input_tokens,
	// message_delta.output_tokens) while passing data through unchanged.
	events, err := w.decoder.Write(data)
	if err != nil {
		slog.Debug("native stream decoder error", slog.Any("error", err))
	}
	for _, event := range events {
		w.extractUsageFromNativeSSE(event.Type, event.Data)
	}
	return w.ginWriter.Write(data)
}

// extractUsageFromNativeSSE parses Anthropic SSE events to extract token usage
// and feeds the recorder.  message_start carries input_tokens + response ID;
// content_block_delta carries text deltas; message_delta carries output_tokens
// and stop_reason.
func (w *nativeResponseWriter) extractUsageFromNativeSSE(eventType string, data []byte) {
	switch eventType {
	case "message_start":
		var event struct {
			Message struct {
				ID    string `json:"id"`
				Usage struct {
					InputTokens              int `json:"input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &event); err == nil {
			w.inputTokens = int64(event.Message.Usage.InputTokens)
			w.cacheReadTokens = int64(event.Message.Usage.CacheReadInputTokens)
			w.cacheCreationTokens = int64(event.Message.Usage.CacheCreationInputTokens)
			w.respID = event.Message.ID
		}
		if w.recorder != nil {
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				ID:    w.respID,
				Model: w.model,
				Choices: []types.ChatCompletionChunkChoice{{
					Index: 0,
					Delta: types.ChatCompletionChunkChoiceDelta{Role: "assistant"},
				}},
			})
		}
	case "content_block_delta":
		if w.recorder != nil {
			var event struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(data, &event); err == nil && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
					ID:    w.respID,
					Model: w.model,
					Choices: []types.ChatCompletionChunkChoice{{
						Index: 0,
						Delta: types.ChatCompletionChunkChoiceDelta{Content: event.Delta.Text},
					}},
				})
			}
		}
	case "message_delta":
		var event struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &event); err == nil {
			w.outputTokens = int64(event.Usage.OutputTokens)
		}
		if w.recorder != nil {
			finishReason := messagesStopReasonToFinishReason(event.Delta.StopReason)
			w.recorder.AppendCompletionChunk(types.ChatCompletionChunk{
				ID:    w.respID,
				Model: w.model,
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
	}
}

func (w *nativeResponseWriter) WriteHeader(code int) {
	if w.stream {
		if code >= 400 {
			// Upstream error in stream mode: buffer the error body and let
			// Finalize decide how to deliver it.  If no data has been written
			// to the client yet (ginWriter.Written() == false), Finalize will
			// send a proper HTTP error status with a JSON body.  If the SSE
			// stream already started, Finalize will emit an error event into
			// the stream instead.
			slog.Warn("native stream upstream error",
				slog.Int("status", code),
				slog.Bool("ginWritten", w.ginWriter.Written()),
			)
			w.streamFailed = true
			w.streamErrStatus = code
			return
		}
		w.ginWriter.WriteHeader(code)
		return
	}
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
	}
}

func (w *nativeResponseWriter) Header() http.Header {
	if w.stream {
		return w.ginWriter.Header()
	}
	return w.header
}

func (w *nativeResponseWriter) Flush() {
	if w.stream {
		// Don't flush the gin writer when the upstream failed before any
		// data was sent — gin's Flush() calls WriteHeaderNow() which would
		// commit a default 200 status, preventing Finalize from sending a
		// proper HTTP error.
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

func (w *nativeResponseWriter) StatusCode() int {
	if w.stream {
		return w.ginWriter.Status()
	}
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

// Finalize extracts usage from the response body (non-stream) or relies on
// usage already extracted during streaming (message_start/message_delta).
func (w *nativeResponseWriter) Finalize() error {
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
		}
		return nil
	}
	// If upstream returned an error, convert to Anthropic error format.
	if w.statusCode >= 400 {
		writeAnthropicUpstreamError(w.ginWriter, w.statusCode, w.bodyBuf.Bytes())
		return nil
	}
	// Copy buffered headers to gin writer, skipping Content-Length and
	// Content-Encoding since the body may have been decompressed by the proxy.
	for k, vs := range w.header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Content-Encoding") {
			continue
		}
		for _, v := range vs {
			w.ginWriter.Header().Add(k, v)
		}
	}
	// Set content type and write the status code if captured.
	w.ginWriter.Header().Set("Content-Type", "application/json")
	if w.statusCode != 0 {
		w.ginWriter.WriteHeader(w.statusCode)
	} else {
		w.ginWriter.WriteHeader(http.StatusOK)
	}
	// Remap the model field in the response body to the public model name
	// and extract usage. If parsing fails, write the original body as-is.
	bodyBytes := w.bodyBuf.Bytes()
	var resp types.AnthropicMessagesResponse
	if err := json.Unmarshal(bodyBytes, &resp); err == nil {
		w.inputTokens = int64(resp.Usage.InputTokens)
		w.outputTokens = int64(resp.Usage.OutputTokens)
		w.cacheReadTokens = int64(resp.Usage.CacheReadInputTokens)
		w.cacheCreationTokens = int64(resp.Usage.CacheCreationInputTokens)
		resp.Model = w.model
		// Feed the recorder a ChatCompletion built from the Anthropic response.
		if w.recorder != nil {
			w.recorder.Completion(buildChatCompletionFromAnthropicResponse(&resp))
		}
		remapped, err := json.Marshal(&resp)
		if err == nil {
			_, _ = w.ginWriter.Write(remapped)
			return nil
		}
		slog.Warn("failed to remarshal native messages response", slog.Any("error", err))
	}
	// Fallback: write the original body unchanged.  This path is reached
	// when the upstream response is not valid JSON or cannot be remarshaled
	// after model remapping.  Return an api_error instead of leaking a
	// potentially malformed body to the Messages client.
	if w.bodyBuf.Len() > 0 {
		w.ginWriter.Header().Set("Content-Type", "application/json")
		writeAnthropicErrorJSON(w.ginWriter, http.StatusBadGateway, ErrTypeAPI,
			"failed to convert upstream response")
	} else {
		// Force gin to commit the status code (gin's WriteHeader is lazy;
		// it only commits on Write/WriteHeaderNow).
		_, _ = w.ginWriter.Write(nil)
	}
	return nil
}

func (w *nativeResponseWriter) Usage() tokenUsage {
	return tokenUsage{
		PromptTokens:              w.inputTokens,
		CompletionTokens:          w.outputTokens,
		TotalTokens:               w.inputTokens + w.outputTokens,
		CachedPromptTokens:        w.cacheReadTokens,
		CacheCreationPromptTokens: w.cacheCreationTokens,
	}
}

// buildChatCompletionFromAnthropicResponse constructs a types.ChatCompletion
// from an AnthropicMessagesResponse for the LLM log recorder.
func buildChatCompletionFromAnthropicResponse(resp *types.AnthropicMessagesResponse) types.ChatCompletion {
	var finishReason string
	if resp.StopReason != nil {
		finishReason = messagesStopReasonToFinishReason(*resp.StopReason)
	} else {
		finishReason = "stop"
	}
	var content string
	var toolCalls []openai.ChatCompletionMessageToolCallUnion
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if content != "" {
				content += "\n"
			}
			content += block.Text
		case "tool_use":
			args := string(block.Input)
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnion{
				ID:   block.ID,
				Type: "function",
				Function: openai.ChatCompletionMessageFunctionToolCallFunction{
					Name:      block.Name,
					Arguments: args,
				},
			})
		}
	}
	msg := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return types.ChatCompletion{
		ChatCompletion: openai.ChatCompletion{
			ID:    resp.ID,
			Model: resp.Model,
			Choices: []openai.ChatCompletionChoice{{
				Index:        0,
				Message:      msg,
				FinishReason: finishReason,
			}},
			Usage: openai.CompletionUsage{
				PromptTokens:     int64(resp.Usage.InputTokens),
				CompletionTokens: int64(resp.Usage.OutputTokens),
				TotalTokens:      int64(resp.Usage.InputTokens + resp.Usage.OutputTokens),
			},
		},
	}
}

