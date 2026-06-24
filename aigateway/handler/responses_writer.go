package handler

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
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

func newResponsesAdapterResponseWriter(w gin.ResponseWriter, stream bool, model string, responsesCounter token.ResponsesTokenCounter) responsesAdapterResponseWriter {
	if stream {
		return newResponsesAdapterStreamWriter(w, model, responsesCounter)
	}
	return newResponsesAdapterNonStreamWriter(w, model, responsesCounter)
}
