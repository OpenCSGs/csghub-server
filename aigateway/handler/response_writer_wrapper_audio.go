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
)

type audioResponseWriter interface {
	Header() http.Header
	WriteHeader(int)
	Write([]byte) (int, error)
	Flush()
}

type ResponseWriterWrapperAudio struct {
	internalWritter http.ResponseWriter
	tokenCounter    *token.AudioUsageCounter
	buffer          bytes.Buffer
}

func NewResponseWriterWrapperAudio(internalWritter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) audioResponseWriter {
	return &ResponseWriterWrapperAudio{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
	}
}

func (rw *ResponseWriterWrapperAudio) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapperAudio) WriteHeader(statusCode int) {
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
		Text  string `json:"text"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err == nil {
		if resp.Usage.TotalTokens > 0 || resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
			rw.tokenCounter.SetUsage(token.Usage{
				TotalTokens:      resp.Usage.TotalTokens,
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
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
