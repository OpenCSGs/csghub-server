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
	"opencsg.com/csghub-server/builder/rpc"
)

type ResponseWriterWrapperEmbedding struct {
	internalWritter http.ResponseWriter
	modSvcClient    rpc.ModerationSvcClient
	tokenCounter    *token.EmbeddingTokenCounter
}

func NewResponseWriterWrapperEmbedding(internalWritter http.ResponseWriter) *ResponseWriterWrapperEmbedding {
	return &ResponseWriterWrapperEmbedding{
		internalWritter: internalWritter,
	}
}

func (rw *ResponseWriterWrapperEmbedding) WithModeration(modSvcClient rpc.ModerationSvcClient) {
	rw.modSvcClient = modSvcClient
}

func (rw *ResponseWriterWrapperEmbedding) WithTokenCounter(counter *token.EmbeddingTokenCounter) {
	rw.tokenCounter = counter
}

func (rw *ResponseWriterWrapperEmbedding) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapperEmbedding) WriteHeader(statusCode int) {
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapperEmbedding) Write(data []byte) (int, error) {
	embeddingResp := openai.CreateEmbeddingResponse{}
	var jsonData []byte
	// try to unzip data
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
	} else if rw.tokenCounter != nil {
		rw.tokenCounter.Embedding(embeddingResp.Usage)
	}

	return rw.internalWritter.Write(data)
}

//TODO: moderate embedding request and generate sensitive response if needed
// func (rw *ResponseWriterWrapperEmbedding) generateSensitiveResp(originResp openai.CreateEmbeddingResponse) openai.CreateEmbeddingResponse {
// 	newResp := openai.CreateEmbeddingResponse{
// 		Data:   nil,
// 		Model:  originResp.Model,
// 		Object: originResp.Object,
// 		Usage:  originResp.Usage,
// 	}
// 	return newResp
// }
