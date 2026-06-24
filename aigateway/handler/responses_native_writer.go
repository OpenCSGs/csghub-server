package handler

import (
	"bytes"
	"net/http"

	"github.com/gin-gonic/gin"
)

type responsesNativeResponseWriter interface {
	http.ResponseWriter
	Finalize() error
}

func newResponsesNativeResponseWriter(w gin.ResponseWriter, stream bool, transformer *responsesNativePayloadTransformer) responsesNativeResponseWriter {
	if stream {
		return newResponsesNativeStreamWriter(w, transformer)
	}
	return newResponsesNativeNonStreamWriter(w, transformer)
}

type responsesNativeStreamWriter struct {
	ginWriter   gin.ResponseWriter
	transformer *responsesNativePayloadTransformer
	decoder     eventStreamDecoder
	failed      bool
	passthrough bool
}

func newResponsesNativeStreamWriter(w gin.ResponseWriter, transformer *responsesNativePayloadTransformer) *responsesNativeStreamWriter {
	return &responsesNativeStreamWriter{ginWriter: w, transformer: transformer}
}

func (w *responsesNativeStreamWriter) Header() http.Header {
	return w.ginWriter.Header()
}

func (w *responsesNativeStreamWriter) WriteHeader(code int) {
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

func (w *responsesNativeStreamWriter) Finalize() error {
	return nil
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
	// eventStreamDecoder.Write always returns a nil error today; the second
	// return is kept for future use (e.g. buffer-overflow detection) so the
	// call site already handles it without another edit.
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
		if len(event.Data) > 0 && string(event.Data) != "[DONE]" {
			if rewritten, ok, err := w.transformer.transformJSON(event.Data); err != nil {
				return 0, err
			} else if ok {
				out = bytes.Replace(event.Raw, event.Data, rewritten, 1)
			}
		}
		if _, err := w.ginWriter.Write(out); err != nil {
			return 0, err
		}
		w.ginWriter.Flush()
	}
	return len(data), nil
}

type responsesNativeNonStreamWriter struct {
	ginWriter   gin.ResponseWriter
	transformer *responsesNativePayloadTransformer
	buffer      bytes.Buffer
}

func newResponsesNativeNonStreamWriter(w gin.ResponseWriter, transformer *responsesNativePayloadTransformer) *responsesNativeNonStreamWriter {
	return &responsesNativeNonStreamWriter{ginWriter: w, transformer: transformer}
}

func (w *responsesNativeNonStreamWriter) Header() http.Header {
	return w.ginWriter.Header()
}

func (w *responsesNativeNonStreamWriter) WriteHeader(code int) {
	w.ginWriter.Header().Del("Content-Length")
	w.ginWriter.WriteHeader(code)
}

func (w *responsesNativeNonStreamWriter) Flush() {
}

func (w *responsesNativeNonStreamWriter) ClearBuffer() {
	w.buffer.Reset()
}

func (w *responsesNativeNonStreamWriter) Write(data []byte) (int, error) {
	return w.buffer.Write(data)
}

func (w *responsesNativeNonStreamWriter) Finalize() error {
	data := w.buffer.Bytes()
	if len(data) == 0 {
		return nil
	}
	if rewritten, ok, err := w.transformer.transformJSON(data); err != nil {
		return err
	} else if ok {
		data = rewritten
	}
	_, err := w.ginWriter.Write(data)
	return err
}
