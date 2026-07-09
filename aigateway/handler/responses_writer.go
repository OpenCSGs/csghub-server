package handler

import (
	"bytes"
	"net/http"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
)

type bufferCommonResponseWriter struct {
	headers    http.Header
	statusCode int
	body       bytes.Buffer
}

func newBufferCommonResponseWriter() *bufferCommonResponseWriter {
	return &bufferCommonResponseWriter{headers: make(http.Header)}
}

func (w *bufferCommonResponseWriter) Header() http.Header {
	return w.headers
}

func (w *bufferCommonResponseWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (w *bufferCommonResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *bufferCommonResponseWriter) Flush() {
}

func (w *bufferCommonResponseWriter) ClearBuffer() {
	w.body.Reset()
}
func (w *bufferCommonResponseWriter) StatusCode() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

type responsesAdapterResponseWriter interface {
	CommonResponseWriter
	Finalize(statusCode int) error
}

func newResponsesAdapterResponseWriter(w gin.ResponseWriter, stream bool, model string, responsesCounter token.ResponsesTokenCounter, moderation component.Moderation, sessionID string, logCapture ...*responsespkg.LLMLogRecorder) responsesAdapterResponseWriter {
	var recorder *responsespkg.LLMLogRecorder
	if len(logCapture) > 0 {
		recorder = logCapture[0]
	}
	if stream {
		return newResponsesAdapterStreamWriter(w, model, responsesCounter, moderation, sessionID, recorder)
	}
	return newResponsesAdapterNonStreamWriter(w, model, responsesCounter, moderation, sessionID, recorder)
}

func newResponsesModerationSessionID() string {
	return uuid.New().String()
}
