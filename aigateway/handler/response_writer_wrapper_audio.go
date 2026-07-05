package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/token"
	commontypes "opencsg.com/csghub-server/common/types"
)

type audioResponseWriter interface {
	Header() http.Header
	WriteHeader(int)
	Write([]byte) (int, error)
	Flush()
	StatusCode() int
	DurationSeconds() (float64, bool)
}

func NewResponseWriterWrapperAudio(internalWritter http.ResponseWriter, tokenCounter *token.AudioUsageCounter, useStream bool) audioResponseWriter {
	if useStream {
		return newStreamAudioResponseWriter(internalWritter, tokenCounter)
	}
	return newNonStreamAudioResponseWriter(internalWritter, tokenCounter)
}

type nonStreamAudioResponseWriter struct {
	internalWritter http.ResponseWriter
	tokenCounter    *token.AudioUsageCounter
	buffer          bytes.Buffer
	statusCode      int
	durationSeconds float64
	hasDuration     bool
}

func newNonStreamAudioResponseWriter(internalWritter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) *nonStreamAudioResponseWriter {
	return &nonStreamAudioResponseWriter{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
		statusCode:      http.StatusOK,
	}
}

func (rw *nonStreamAudioResponseWriter) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *nonStreamAudioResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *nonStreamAudioResponseWriter) Write(data []byte) (int, error) {
	rw.captureText(data)
	return rw.internalWritter.Write(data)
}

func (rw *nonStreamAudioResponseWriter) Flush() {
	if flusher, ok := rw.internalWritter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *nonStreamAudioResponseWriter) captureText(data []byte) {
	if rw == nil || rw.tokenCounter == nil || len(data) == 0 {
		return
	}

	rw.buffer.Write(data)
	body := rw.buffer.Bytes()
	if gzipReader, err := gzip.NewReader(bytes.NewReader(body)); err == nil {
		defer gzipReader.Close()
		unzipped, readErr := io.ReadAll(gzipReader)
		if readErr != nil {
			slog.Error("failed to read gzipped audio transcription response", slog.Any("err", readErr))
		} else {
			body = unzipped
		}
	}

	var resp struct {
		Text     string   `json:"text"`
		Duration *float64 `json:"duration"`
		Usage    struct {
			PromptTokens     int64    `json:"prompt_tokens"`
			CompletionTokens int64    `json:"completion_tokens"`
			TotalTokens      int64    `json:"total_tokens"`
			Seconds          *float64 `json:"seconds"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err == nil {
		rw.captureDuration(resp.Usage.Seconds, resp.Duration)
		if resp.Usage.TotalTokens > 0 || resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
			duration, _ := rw.DurationSeconds()
			rw.tokenCounter.SetUsage(token.Usage{
				TotalTokens:      resp.Usage.TotalTokens,
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				DataType:         string(commontypes.DataTypeAudio),
				Duration:         duration,
				CompletionRC:     1,
				CompletionDesc:   resp.Text,
			})
			return
		}
		if resp.Text != "" {
			rw.tokenCounter.Text(resp.Text)
			return
		}
	}

	if json.Valid(body) {
		return
	}

	contentType := strings.ToLower(rw.Header().Get("Content-Type"))
	if strings.Contains(contentType, "text/") || strings.Contains(contentType, "application/octet-stream") || contentType == "" {
		text := strings.TrimSpace(string(body))
		if text != "" {
			rw.tokenCounter.Text(text)
		}
	}
}

func (rw *nonStreamAudioResponseWriter) captureDuration(candidates ...*float64) {
	for _, candidate := range candidates {
		if candidate != nil && *candidate > 0 {
			rw.durationSeconds = *candidate
			rw.hasDuration = true
			if rw.tokenCounter != nil {
				rw.tokenCounter.Duration(*candidate)
			}
			return
		}
	}
}

func (rw *nonStreamAudioResponseWriter) StatusCode() int {
	// Match net/http behavior: a body write without WriteHeader implies 200 OK.
	// Audio proxying is synchronous; trace completion reads this only after
	// ReverseProxy.ServeHTTP returns and wrapper writes have completed.
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func (rw *nonStreamAudioResponseWriter) DurationSeconds() (float64, bool) {
	if rw == nil || !rw.hasDuration {
		return 0, false
	}
	return rw.durationSeconds, true
}

type streamAudioResponseWriter struct {
	internalWritter http.ResponseWriter
	tokenCounter    *token.AudioUsageCounter
	decoder         streamdecoder.Decoder
	statusCode      int
	durationSeconds float64
	hasDuration     bool
	streamStarted   bool
	accumulatedText string
}

func newStreamAudioResponseWriter(internalWritter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) *streamAudioResponseWriter {
	return &streamAudioResponseWriter{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
		statusCode:      http.StatusOK,
	}
}

func (rw *streamAudioResponseWriter) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *streamAudioResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWritter.Header().Del("Content-Length")

	ct := rw.internalWritter.Header().Get("Content-Type")
	switch {
	case strings.Contains(ct, "text/event-stream"):
		rw.decoder = streamdecoder.NewSSE()
	case strings.Contains(ct, "application/x-ndjson"):
		rw.decoder = streamdecoder.NewNDJSON()
	}

	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *streamAudioResponseWriter) Write(data []byte) (int, error) {
	originLen := len(data)

	if shouldPassthroughUpstreamError(rw.statusCode, rw.streamStarted) {
		return rw.internalWritter.Write(data)
	}

	if rw.decoder == nil {
		// Unknown stream formats are passed through unchanged; usage can only be
		// captured for stream formats with a selected decoder.
		rw.streamStarted = true
		return rw.internalWritter.Write(data)
	}

	events, err := rw.decoder.Write(data)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if err := rw.handleStreamEvent(event); err != nil {
			return 0, err
		}
	}
	return originLen, nil
}

func (rw *streamAudioResponseWriter) handleStreamEvent(event *streamdecoder.Event) error {
	if rw.decoder.Format() == streamdecoder.FormatSSE && string(event.Data) == "[DONE]" {
		rw.streamStarted = true
		if _, err := rw.internalWritter.Write(event.Raw); err != nil {
			return err
		}
		if flusher, ok := rw.internalWritter.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}
	return rw.handleStreamPayload(event.Data, event.Raw)
}

func (rw *streamAudioResponseWriter) handleStreamPayload(payload, raw []byte) error {
	rw.streamStarted = true

	var chunk struct {
		Text     string   `json:"text"`
		Duration *float64 `json:"duration"`
	}
	if err := json.Unmarshal(payload, &chunk); err != nil {
		slog.Warn("streamAudioResponseWriter: failed to parse stream payload",
			slog.Any("error", err),
			slog.Int("accumulated_text_len", len(rw.accumulatedText)))
		if _, writeErr := rw.internalWritter.Write(raw); writeErr != nil {
			return writeErr
		}
		if flusher, ok := rw.internalWritter.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}

	rw.captureDuration(chunk.Duration)

	if chunk.Text != "" {
		rw.accumulatedText += chunk.Text
		if rw.tokenCounter != nil {
			rw.tokenCounter.Text(rw.accumulatedText)
		}
	}

	if _, err := rw.internalWritter.Write(raw); err != nil {
		return err
	}
	if flusher, ok := rw.internalWritter.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (rw *streamAudioResponseWriter) captureDuration(candidates ...*float64) {
	for _, candidate := range candidates {
		if candidate != nil && *candidate > 0 {
			rw.durationSeconds = *candidate
			rw.hasDuration = true
			if rw.tokenCounter != nil {
				rw.tokenCounter.Duration(*candidate)
			}
			return
		}
	}
}

func (rw *streamAudioResponseWriter) Flush() {
	if flusher, ok := rw.internalWritter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *streamAudioResponseWriter) StatusCode() int {
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func (rw *streamAudioResponseWriter) DurationSeconds() (float64, bool) {
	if rw == nil {
		return 0, false
	}
	if rw.hasDuration {
		return rw.durationSeconds, true
	}
	return 0, false
}
