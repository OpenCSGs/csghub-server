package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

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

type ResponseWriterWrapperAudio struct {
	internalWritter http.ResponseWriter
	tokenCounter    *token.AudioUsageCounter
	buffer          bytes.Buffer
	statusCode      int
	durationSeconds float64
	hasDuration     bool
}

func NewResponseWriterWrapperAudio(internalWritter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) audioResponseWriter {
	return &ResponseWriterWrapperAudio{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
		statusCode:      http.StatusOK,
	}
}

func (rw *ResponseWriterWrapperAudio) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapperAudio) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapperAudio) Write(data []byte) (int, error) {
	rw.captureText(data)
	return rw.internalWritter.Write(data)
}

func (rw *ResponseWriterWrapperAudio) Flush() {
	if flusher, ok := rw.internalWritter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *ResponseWriterWrapperAudio) captureText(data []byte) {
	if rw.tokenCounter == nil || len(data) == 0 {
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

func (rw *ResponseWriterWrapperAudio) captureDuration(candidates ...*float64) {
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

func (rw *ResponseWriterWrapperAudio) StatusCode() int {
	// Match net/http behavior: a body write without WriteHeader implies 200 OK.
	// Audio proxying is synchronous; trace completion reads this only after
	// ReverseProxy.ServeHTTP returns and wrapper writes have completed.
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

func (rw *ResponseWriterWrapperAudio) DurationSeconds() (float64, bool) {
	if rw == nil || !rw.hasDuration {
		return 0, false
	}
	return rw.durationSeconds, true
}
