package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
)

type nonStreamResponseWriter struct {
	internalWritter gin.ResponseWriter
	modSvcClient    rpc.ModerationSvcClient
	tokenCounter    *token.ChatTokenCounter
	buffer          bytes.Buffer
	hasProcessed    bool
}

func newNonStreamResponseWriter(internalWritter gin.ResponseWriter) *nonStreamResponseWriter {
	return &nonStreamResponseWriter{
		internalWritter: internalWritter,
		hasProcessed:    false,
	}
}

func (nsw *nonStreamResponseWriter) WithModeration(modSvcClient rpc.ModerationSvcClient) {
	nsw.modSvcClient = modSvcClient
}

func (nsw *nonStreamResponseWriter) WithLLMTokenCounter(counter *token.ChatTokenCounter) {
	nsw.tokenCounter = counter
}

func (nsw *nonStreamResponseWriter) Header() http.Header {
	return nsw.internalWritter.Header()
}

func (nsw *nonStreamResponseWriter) WriteHeader(statusCode int) {
	nsw.internalWritter.WriteHeader(statusCode)
}

func (nsw *nonStreamResponseWriter) Write(data []byte) (int, error) {
	return nsw.nonStreamWrite(data)
}

func (nsw *nonStreamResponseWriter) Flush() {
	nsw.internalWritter.Flush()
}

// nonStreamWrite processes non-streaming response data with content moderation and token counting
func (nsw *nonStreamResponseWriter) nonStreamWrite(originData []byte) (int, error) {
	// Step 1: Store original length and accumulate data in buffer
	originLen := len(originData)
	nsw.buffer.Write(originData)
	slog.Debug("write into buffer", slog.Any("buffer length", nsw.buffer.Len()))

	// Step 2: Try to decode data based on content encoding header
	originEncodingHeader := nsw.internalWritter.Header().Get("Content-Encoding")
	data, err := compress.Decode(originEncodingHeader, nsw.buffer.Bytes())
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		slog.Debug("NonStreamResponseWriter buffer decode attempt failed, waiting for more data",
			slog.String("encoding header", originEncodingHeader),
			slog.String("decoded data", string(data)),
			slog.Any("error", err))
		return originLen, nil
	}

	slog.Debug("buffer decoded", slog.String("decode data", string(data)))

	// Step 3: Parse JSON data into completion struct
	var completion types.ChatCompletion
	err = json.Unmarshal(data, &completion)
	if err != nil {
		slog.Debug("NonStreamResponseWriter nonStreamWrite unmarshal attempt failed, waiting for more data",
			slog.String("decoded data", string(data)),
			slog.Any("error", err))
		return originLen, nil
	}

	slog.Debug("JSON unmarshal", slog.Any("data", completion))

	// Step 4: Count tokens if token counter is available
	if nsw.tokenCounter != nil {
		nsw.tokenCounter.Completion(completion)
	}

	// Step 5: Handle case with empty choices array
	if len(completion.Choices) == 0 {
		return originLen, nsw.writeToInternal(originData)
	}

	// Step 6: Perform content moderation if service is available
	content := completion.Choices[0].Message.Content
	if nsw.modSvcClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := nsw.modSvcClient.PassTextCheck(ctx, string(sensitive.ScenarioChatDetection), content)
		cancel()

		if err != nil {
			slog.Error("NonStreamResponseWriter nonStreamWrite failed to call moderation service", slog.Any("err", err))
			// Continue with original content if moderation service fails
		} else if result.IsSensitive {
			// Replace sensitive content with block message
			slog.Debug("NonStreamResponseWriter nonStreamWrite checkresult is sensitive",
				slog.Any("content", content),
				slog.Any("reason", result.Reason))
			completion.Choices[0].Message.Content = "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation."

			// Re-encode modified completion
			modifiedData, _ := json.Marshal(completion)
			compressedData, _ := compress.Encode(originEncodingHeader, modifiedData)
			return originLen, nsw.writeToInternal(compressedData)
		}
	}

	// Step 7: Write original data to internal writer
	return originLen, nsw.writeToInternal(nsw.buffer.Bytes())
}

// writeToInternal encapsulates writing to the internal writer with error logging and buffer cleanup
func (nsw *nonStreamResponseWriter) writeToInternal(data []byte) error {
	_, err := nsw.internalWritter.Write(data)
	// Clear buffer to free memory after successful write
	nsw.buffer.Reset()
	if err != nil {
		slog.Error("NonStreamResponseWriter failed to write to internal writer", slog.Any("err", err))
	}
	return err
}

func (nsw *nonStreamResponseWriter) ClearBuffer() {
	data := nsw.buffer.Bytes()
	if len(data) > 0 {
		_ = nsw.writeToInternal(data)
	}
}
