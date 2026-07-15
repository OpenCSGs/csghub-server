package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/token"
	commontypes "opencsg.com/csghub-server/common/types"
)

const speechAudioDurationHeader = "Audio-Duration-Seconds"

type speechResponseWriter interface {
	Header() http.Header
	WriteHeader(int)
	Write([]byte) (int, error)
	Flush()
	StatusCode() int
}

// NewResponseWriterWrapperSpeech wraps the response of an OpenAI-compatible
// text-to-speech request (POST /v1/audio/speech). Binary audio responses and
// raw audio streams pass through unchanged; SSE responses (stream=true or
// stream_format="sse") are decoded to capture token usage from the terminal
// speech.audio.done event.
func NewResponseWriterWrapperSpeech(internalWriter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) speechResponseWriter {
	return &speechAudioResponseWriter{
		internalWriter: internalWriter,
		tokenCounter:   tokenCounter,
		statusCode:     http.StatusOK,
	}
}

type speechAudioResponseWriter struct {
	internalWriter http.ResponseWriter
	tokenCounter   *token.AudioUsageCounter
	decoder        streamdecoder.Decoder
	statusCode     int
}

func (rw *speechAudioResponseWriter) Header() http.Header {
	return rw.internalWriter.Header()
}

func (rw *speechAudioResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	if rw.tokenCounter != nil {
		duration, err := strconv.ParseFloat(strings.TrimSpace(rw.internalWriter.Header().Get(speechAudioDurationHeader)), 64)
		if err == nil {
			rw.tokenCounter.Duration(duration)
		}
	}
	ct := rw.internalWriter.Header().Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		rw.decoder = streamdecoder.NewSSE()
		rw.internalWriter.Header().Del("Content-Length")
	}
	rw.internalWriter.WriteHeader(statusCode)
}

func (rw *speechAudioResponseWriter) Write(data []byte) (int, error) {
	if rw.decoder == nil {
		// Binary audio or raw audio stream: usage falls back to the counter's
		// input-text based accounting set by the handler.
		return rw.internalWriter.Write(data)
	}

	originLen := len(data)
	events, err := rw.decoder.Write(data)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		rw.captureUsage(event.Data)
		if _, err := rw.internalWriter.Write(event.Raw); err != nil {
			return 0, err
		}
		rw.Flush()
	}
	return originLen, nil
}

func (rw *speechAudioResponseWriter) captureUsage(payload []byte) {
	if rw.tokenCounter == nil {
		return
	}
	var event struct {
		Type  string `json:"type"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return
	}
	if event.Type != "speech.audio.done" {
		return
	}
	if event.Usage.TotalTokens == 0 && event.Usage.InputTokens == 0 && event.Usage.OutputTokens == 0 {
		return
	}
	rw.tokenCounter.SetUsage(token.Usage{
		TotalTokens:      event.Usage.TotalTokens,
		PromptTokens:     event.Usage.InputTokens,
		CompletionTokens: event.Usage.OutputTokens,
		DataType:         string(commontypes.DataTypeAudio),
		CompletionRC:     1,
	})
}

func (rw *speechAudioResponseWriter) Flush() {
	if flusher, ok := rw.internalWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *speechAudioResponseWriter) StatusCode() int {
	// Match net/http behavior: a body write without WriteHeader implies 200 OK.
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}

// NewResponseWriterWrapperSpeechBatch wraps the JSON response of a batch
// text-to-speech request (POST /v1/audio/speech/batch) and sums the usage of
// all successful items for billing.
func NewResponseWriterWrapperSpeechBatch(internalWriter http.ResponseWriter, tokenCounter *token.AudioUsageCounter) speechResponseWriter {
	return &speechBatchResponseWriter{
		internalWriter: internalWriter,
		tokenCounter:   tokenCounter,
		statusCode:     http.StatusOK,
	}
}

type speechBatchResponseWriter struct {
	internalWriter http.ResponseWriter
	tokenCounter   *token.AudioUsageCounter
	buffer         bytes.Buffer
	statusCode     int
}

func (rw *speechBatchResponseWriter) Header() http.Header {
	return rw.internalWriter.Header()
}

func (rw *speechBatchResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWriter.WriteHeader(statusCode)
}

func (rw *speechBatchResponseWriter) Write(data []byte) (int, error) {
	rw.captureUsage(data)
	return rw.internalWriter.Write(data)
}

// captureUsage accumulates the response body and, once it parses as the batch
// response JSON, sums per-item usage of successful results.
func (rw *speechBatchResponseWriter) captureUsage(data []byte) {
	if rw.tokenCounter == nil || len(data) == 0 {
		return
	}
	rw.buffer.Write(data)

	var resp struct {
		Results []struct {
			Status string `json:"status"`
			Usage  *struct {
				InputTokens  int64   `json:"input_tokens"`
				OutputTokens int64   `json:"output_tokens"`
				TotalTokens  int64   `json:"total_tokens"`
				Seconds      float64 `json:"seconds"`
			} `json:"usage"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rw.buffer.Bytes(), &resp); err != nil {
		return
	}
	usage := token.Usage{
		DataType: string(commontypes.DataTypeAudio),
	}
	for _, result := range resp.Results {
		if result.Status != "success" || result.Usage == nil {
			continue
		}
		usage.PromptTokens += result.Usage.InputTokens
		usage.CompletionTokens += result.Usage.OutputTokens
		usage.TotalTokens += result.Usage.TotalTokens
		usage.Duration += result.Usage.Seconds
		usage.CompletionRC++
	}
	if usage.CompletionRC == 0 {
		return
	}
	rw.tokenCounter.SetUsage(usage)
}

func (rw *speechBatchResponseWriter) Flush() {
	if flusher, ok := rw.internalWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *speechBatchResponseWriter) StatusCode() int {
	// Match net/http behavior: a body write without WriteHeader implies 200 OK.
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}
