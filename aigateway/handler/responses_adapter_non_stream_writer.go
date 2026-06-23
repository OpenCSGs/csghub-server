package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
)

type responsesAdapterNonStreamWriter struct {
	*bufferCommonResponseWriter
	ginWriter        gin.ResponseWriter
	model            string
	responsesCounter token.ResponsesTokenCounter
}

func newResponsesAdapterNonStreamWriter(w gin.ResponseWriter, model string, responsesCounter token.ResponsesTokenCounter) *responsesAdapterNonStreamWriter {
	return &responsesAdapterNonStreamWriter{
		bufferCommonResponseWriter: newBufferCommonResponseWriter(),
		ginWriter:                  w,
		model:                      model,
		responsesCounter:           responsesCounter,
	}
}

func (w *responsesAdapterNonStreamWriter) Finalize(statusCode int) error {
	if statusCode >= http.StatusBadRequest {
		copyHeader(w.ginWriter.Header(), w.Header())
		w.ginWriter.WriteHeader(statusCode)
		_, err := w.ginWriter.Write(w.body.Bytes())
		return err
	}

	chatBody, err := decodeResponsesAdapterChatBody(w.bufferCommonResponseWriter)
	if err != nil {
		return err
	}
	resp, err := chatResponseToResponses(chatBody, w.model)
	if err != nil {
		return err
	}
	if w.responsesCounter != nil {
		w.responsesCounter.Response(resp)
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	w.ginWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.ginWriter.WriteHeader(http.StatusOK)
	_, err = w.ginWriter.Write(body)
	return err
}
