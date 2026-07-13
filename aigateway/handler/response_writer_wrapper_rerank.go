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

// ResponseWriterWrapperRerank captures the rerank response body to extract
// token usage. Rerank usage is prompt-token based, so it reuses the
// embedding token counter for metering.
type ResponseWriterWrapperRerank struct {
	internalWritter http.ResponseWriter
	tokenCounter    token.EmbeddingTokenCounter
	statusCode      int
	capture         bytes.Buffer
	captureLimit    int
	truncated       bool
}

const maxRerankResponseCaptureBytes = 4 << 20

// rerankUsageResponse matches the usage block returned by vllm and llama.cpp
// rerank endpoints. TEI returns no usage, in which case the token counter
// falls back to tokenizing the input.
type rerankUsageResponse struct {
	Usage struct {
		PromptTokens int64 `json:"prompt_tokens"`
		TotalTokens  int64 `json:"total_tokens"`
	} `json:"usage"`
}

func NewResponseWriterWrapperRerank(internalWritter http.ResponseWriter, tokenCounter token.EmbeddingTokenCounter) *ResponseWriterWrapperRerank {
	return &ResponseWriterWrapperRerank{
		internalWritter: internalWritter,
		tokenCounter:    tokenCounter,
		captureLimit:    maxRerankResponseCaptureBytes,
	}
}

func (rw *ResponseWriterWrapperRerank) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapperRerank) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapperRerank) Write(data []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	rw.captureData(data)
	return rw.internalWritter.Write(data)
}

func (rw *ResponseWriterWrapperRerank) CaptureRerankUsage() {
	if rw == nil || rw.tokenCounter == nil || rw.truncated || rw.capture.Len() == 0 {
		return
	}

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

	var rerankResp rerankUsageResponse
	if err := json.Unmarshal(jsonData, &rerankResp); err != nil {
		slog.Error("failed to unmarshal rerank response", slog.Any("err", err))
		return
	}
	totalTokens := rerankResp.Usage.TotalTokens
	promptTokens := rerankResp.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = totalTokens
	}
	if totalTokens == 0 && promptTokens == 0 {
		// no usage info in response (e.g. TEI), leave counter to tokenizer fallback
		return
	}
	rw.tokenCounter.Embedding(openai.CreateEmbeddingResponseUsage{
		PromptTokens: promptTokens,
		TotalTokens:  totalTokens,
	})
}

func (rw *ResponseWriterWrapperRerank) captureData(data []byte) {
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

func (rw *ResponseWriterWrapperRerank) StatusCode() int {
	if rw == nil || rw.statusCode == 0 {
		return http.StatusOK
	}
	return rw.statusCode
}
