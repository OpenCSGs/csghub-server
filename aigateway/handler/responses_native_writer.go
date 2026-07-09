package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

type responsesNativeResponseWriter interface {
	http.ResponseWriter
	Finalize() error
	StatusCode() int
	FirstWriteAt() time.Time
}

func newResponsesNativeResponseWriter(w gin.ResponseWriter, stream bool, transformer *responsesNativePayloadTransformer, moderation component.Moderation, sessionID string) responsesNativeResponseWriter {
	if stream {
		return newResponsesNativeStreamWriter(w, transformer, moderation, sessionID)
	}
	return newResponsesNativeNonStreamWriter(w, transformer, moderation, sessionID)
}

type responsesNativeStreamWriter struct {
	ginWriter    gin.ResponseWriter
	transformer  *responsesNativePayloadTransformer
	decoder      streamdecoder.Decoder
	moderation   component.Moderation
	sessionID    string
	failed       bool
	passthrough  bool
	wroteContent bool
	checkClosed  bool
	statusCode   int
	firstWriteAt time.Time
}

func newResponsesNativeStreamWriter(w gin.ResponseWriter, transformer *responsesNativePayloadTransformer, moderation component.Moderation, sessionID string) *responsesNativeStreamWriter {
	return &responsesNativeStreamWriter{
		ginWriter:   w,
		transformer: transformer,
		decoder:     streamdecoder.NewSSE(),
		moderation:  moderation,
		sessionID:   sessionID,
	}
}

func (w *responsesNativeStreamWriter) Header() http.Header {
	return w.ginWriter.Header()
}

func (w *responsesNativeStreamWriter) WriteHeader(code int) {
	w.statusCode = code
	if isUpstreamHTTPError(code) {
		w.passthrough = true
		w.ginWriter.Header().Del("Content-Length")
		w.ginWriter.WriteHeader(code)
		return
	}
	w.ginWriter.Header().Set("Content-Type", "text/event-stream")
	w.ginWriter.Header().Del("Content-Length")
	w.ginWriter.WriteHeader(code)
}

func (w *responsesNativeStreamWriter) Flush() {
	w.ginWriter.Flush()
}

func (w *responsesNativeStreamWriter) StatusCode() int {
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *responsesNativeStreamWriter) FirstWriteAt() time.Time {
	return w.firstWriteAt
}

func (w *responsesNativeStreamWriter) Finalize() error {
	if w.failed || w.passthrough {
		w.cleanupStreamModeration()
		return nil
	}
	if w.moderation != nil && w.wroteContent && !w.checkClosed {
		ctx, cancel := responsespkg.ModerationContext()
		result, err := w.closeStreamModeration(ctx)
		cancel()
		if err == nil && result != nil && result.IsSensitive {
			w.failed = true
			responsespkg.WriteSensitiveStreamEvent(w.ginWriter)
			return nil
		}
	}
	return nil
}

func (w *responsesNativeStreamWriter) closeStreamModeration(ctx context.Context) (*rpc.CheckResult, error) {
	if w.moderation == nil || w.checkClosed {
		return nil, nil
	}
	w.checkClosed = true
	return w.moderation.CloseStreamCheck(ctx, w.sessionID)
}

func (w *responsesNativeStreamWriter) cleanupStreamModeration() {
	if w.moderation == nil || !w.wroteContent || w.checkClosed {
		return
	}
	ctx, cancel := responsespkg.ModerationContext()
	defer cancel()
	_, _ = w.closeStreamModeration(ctx)
}

func (w *responsesNativeStreamWriter) Write(data []byte) (int, error) {
	if w.failed {
		return len(data), nil
	}
	if w.passthrough {
		n, err := w.ginWriter.Write(data)
		w.ginWriter.Flush()
		return n, err
	}
	// Decoder errors are reserved for future checks such as buffer overflow.
	events, _ := w.decoder.Write(data)
	for _, event := range events {
		if event.Type == "error" {
			w.failed = true
			if _, err := w.ginWriter.Write(event.Raw); err != nil {
				return 0, err
			}
			w.ginWriter.Flush()
			return len(data), nil
		}
		out := event.Raw
		if string(event.Data) == "[DONE]" {
			if w.moderation != nil && w.wroteContent {
				ctx, cancel := responsespkg.ModerationContext()
				result, rerr := w.closeStreamModeration(ctx)
				cancel()
				if rerr == nil && result != nil && result.IsSensitive {
					w.failed = true
					responsespkg.WriteSensitiveStreamEvent(w.ginWriter)
					return len(data), nil
				}
			}
		} else if len(event.Data) > 0 {
			w.wroteContent = true
			if content, ok := responsesStreamEventText(event.Data); ok && w.checkNativeStreamSensitive(content) {
				w.failed = true
				responsespkg.WriteSensitiveStreamEvent(w.ginWriter)
				return len(data), nil
			}
			if rewritten, ok, err := w.transformer.transformJSON(event.Data); err != nil {
				return 0, err
			} else if ok {
				out = bytes.Replace(event.Raw, event.Data, rewritten, 1)
			}
		}
		if _, err := w.ginWriter.Write(out); err != nil {
			return 0, err
		}
		if len(out) > 0 && w.firstWriteAt.IsZero() {
			w.firstWriteAt = time.Now()
		}
		w.ginWriter.Flush()
	}
	return len(data), nil
}

type responsesNativeNonStreamWriter struct {
	ginWriter    gin.ResponseWriter
	transformer  *responsesNativePayloadTransformer
	buffer       bytes.Buffer
	moderation   component.Moderation
	sessionID    string
	statusCode   int
	firstWriteAt time.Time
}

func newResponsesNativeNonStreamWriter(w gin.ResponseWriter, transformer *responsesNativePayloadTransformer, moderation component.Moderation, sessionID string) *responsesNativeNonStreamWriter {
	return &responsesNativeNonStreamWriter{ginWriter: w, transformer: transformer, moderation: moderation, sessionID: sessionID}
}

func (w *responsesNativeNonStreamWriter) Header() http.Header {
	return w.ginWriter.Header()
}

func (w *responsesNativeNonStreamWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ginWriter.Header().Del("Content-Length")
}

func (w *responsesNativeNonStreamWriter) Flush() {
}

func (w *responsesNativeNonStreamWriter) ClearBuffer() {
	w.buffer.Reset()
}

func (w *responsesNativeNonStreamWriter) StatusCode() int {
	if w.statusCode != 0 {
		return w.statusCode
	}
	return http.StatusOK
}

func (w *responsesNativeNonStreamWriter) FirstWriteAt() time.Time {
	return w.firstWriteAt
}

func (w *responsesNativeNonStreamWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *responsesNativeNonStreamWriter) Finalize() error {
	data := w.buffer.Bytes()
	if len(data) == 0 {
		return nil
	}
	if w.checkNativeNonStreamSensitive(data) {
		return nil
	}
	if rewritten, ok, err := w.transformer.transformJSON(data); err != nil {
		return err
	} else if ok {
		data = rewritten
	}
	w.ginWriter.WriteHeader(w.StatusCode())
	_, err := w.ginWriter.Write(data)
	if len(data) > 0 && w.firstWriteAt.IsZero() {
		w.firstWriteAt = time.Now()
	}
	return err
}

// checkNativeStreamSensitive runs the per-check sensitive check on extracted
// upstream event text. Nil-safe — returns false when no moderation component
// is wired.
func (w *responsesNativeStreamWriter) checkNativeStreamSensitive(content string) bool {
	if w.moderation == nil || strings.TrimSpace(content) == "" {
		return false
	}
	ctx, cancel := responsespkg.ModerationContext()
	result, err := w.moderation.CheckText(ctx, types.TextModerationRequest{
		Content: content,
		Key:     w.sessionID,
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeStream,
	})
	cancel()
	if err != nil || result == nil {
		return false
	}
	return result.IsSensitive
}

// checkNativeNonStreamSensitive inspects the assembled upstream Responses
// response and, when sensitive content is detected, writes the canned
// blocked JSON in place of the normal payload.
func (w *responsesNativeNonStreamWriter) checkNativeNonStreamSensitive(data []byte) bool {
	if w.moderation == nil {
		return false
	}
	var resp types.ResponsesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false
	}
	content := resp.OutputText
	if content == "" {
		content = responsesOutputModerationText(resp.Output)
	}
	if strings.TrimSpace(content) == "" {
		return false
	}
	ctx, cancel := responsespkg.ModerationContext()
	result, err := w.moderation.CheckText(ctx, types.TextModerationRequest{
		Content: content,
		Key:     w.sessionID,
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeNonStream,
	})
	cancel()
	if err != nil || result == nil {
		return false
	}
	if !result.IsSensitive {
		return false
	}
	canned := types.ResponsesResponse{
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: responsespkg.BlockedMessage,
			}},
		}},
	}
	body, err := json.Marshal(canned)
	if err != nil {
		return false
	}
	w.ginWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.ginWriter.WriteHeader(http.StatusOK)
	_, _ = w.ginWriter.Write(body)
	return true
}

// responsesStreamEventText extracts a single text field from a Responses SSE
// event payload for sensitive-content moderation. Returns ok=true when the
// payload contains at least one of: a `delta` string, a `response.output_text`
// assembly, or a refusal string.
func responsesStreamEventText(data []byte) (string, bool) {
	var ev types.ResponsesStreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return "", false
	}
	if text := strings.TrimSpace(ev.Delta); text != "" {
		return text, true
	}
	if ev.Response != nil {
		if text := strings.TrimSpace(ev.Response.OutputText); text != "" {
			return text, true
		}
		if text := strings.TrimSpace(responsesOutputModerationText(ev.Response.Output)); text != "" {
			return text, true
		}
	}
	if ev.Item != nil {
		if text := strings.TrimSpace(responsesOutputModerationText([]types.ResponsesOutputItem{*ev.Item})); text != "" {
			return text, true
		}
	}
	if len(ev.Part) > 0 {
		var part types.ResponsesContentPart
		if err := json.Unmarshal(ev.Part, &part); err == nil {
			if text := responsesContentPartModerationText(part); strings.TrimSpace(text) != "" {
				return text, true
			}
		}
	}
	return "", false
}

func responsesOutputModerationText(items []types.ResponsesOutputItem) string {
	var b strings.Builder
	for _, item := range items {
		writeModerationText(&b, item.Name)
		writeModerationText(&b, item.Arguments)
		for _, part := range item.Content {
			writeModerationText(&b, responsesContentPartModerationText(part))
		}
	}
	return b.String()
}

func responsesContentPartModerationText(part types.ResponsesContentPart) string {
	switch part.Type {
	case "output_text", "text":
		return part.Text
	case "refusal":
		return part.Refusal
	default:
		if strings.TrimSpace(part.Text) != "" {
			return part.Text
		}
		return part.Refusal
	}
}
