package handler

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/token"
)

type ResponseWriterWrapperEmbedding struct {
	internalWritter http.ResponseWriter
	tokenCounter    token.EmbeddingTokenCounter
	statusCode      int
	capture         bytes.Buffer
	captureLimit    int
	truncated       bool
}

const maxEmbeddingResponseCaptureBytes = 4 << 20

func NewResponseWriterWrapperEmbedding(internalWritter http.ResponseWriter, tokenCounter token.EmbeddingTokenCounter) *ResponseWriterWrapperEmbedding {
	return newResponseWriterWrapperEmbedding(internalWritter, tokenCounter, maxEmbeddingResponseCaptureBytes)
}

func newResponseWriterWrapperEmbedding(internalWritter http.ResponseWriter, tokenCounter token.EmbeddingTokenCounter, captureLimit int) *ResponseWriterWrapperEmbedding {
	return &ResponseWriterWrapperEmbedding{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
		captureLimit:    captureLimit,
	}
}

func (rw *ResponseWriterWrapperEmbedding) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapperEmbedding) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapperEmbedding) Write(data []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	rw.captureData(data)
	return rw.internalWritter.Write(data)
}

func (rw *ResponseWriterWrapperEmbedding) CaptureEmbeddingUsage() {
	if rw == nil || rw.tokenCounter == nil || rw.truncated || rw.capture.Len() == 0 {
		return
	}

	embeddingResp := openai.CreateEmbeddingResponse{}
	var jsonData []byte
	// try to unzip data
	data := rw.capture.Bytes()
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err == nil {
		defer gzipReader.Close()
		jsonData, err = io.ReadAll(gzipReader)
		if err != nil {
			slog.Error("failed to read gzipped data", slog.Any("err", err))
			jsonData = data // use original data
		}
	} else {
		jsonData = data
	}

	// unmarshal json data
	if err := json.Unmarshal(jsonData, &embeddingResp); err != nil {
		slog.Error("failed to unmarshal embedding response", slog.Any("err", err))
	} else {
		rw.tokenCounter.Embedding(embeddingResp.Usage)
	}
}

func (rw *ResponseWriterWrapperEmbedding) captureData(data []byte) {
	if rw == nil || rw.truncated || rw.captureLimit <= 0 || len(data) == 0 {
		return
	}
	remaining := rw.captureLimit - rw.capture.Len()
	if remaining <= 0 {
		rw.truncated = true
		return
	}
	if len(data) > remaining {
		rw.capture.Write(data[:remaining])
		rw.truncated = true
		return
	}
	rw.capture.Write(data)
}

func (rw *ResponseWriterWrapperEmbedding) StatusCode() int {
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}
