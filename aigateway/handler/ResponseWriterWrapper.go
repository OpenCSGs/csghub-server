package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
)

var ErrSensitiveContent = errors.New("sensitive content detected")

var _ http.Hijacker = (*ResponseWriterWrapper)(nil)

type ResponseWriterWrapper struct {
	internalWritter    gin.ResponseWriter
	modSvcClient       rpc.ModerationSvcClient
	eventStreamDecoder *eventStreamDecoder
	tokenCounter       LLMTokenCounter
	useStream          bool
}

// Hijack allows the HTTP connection upgrading to a different protocol, such as WebSockets or HTTP/2.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func NewResponseWriterWrapper(internalWritter gin.ResponseWriter, useStream bool) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{
		internalWritter:    internalWritter,
		eventStreamDecoder: &eventStreamDecoder{},
		useStream:          useStream,
	}
}

func (rw *ResponseWriterWrapper) WithModeration(modSvcClient rpc.ModerationSvcClient) *ResponseWriterWrapper {
	rw.modSvcClient = modSvcClient
	return rw
}

func (rw *ResponseWriterWrapper) WithLLMTokenCounter(llmTokenCounter LLMTokenCounter) *ResponseWriterWrapper {
	rw.tokenCounter = llmTokenCounter
	return rw
}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapper) Write(data []byte) (int, error) {
	if !rw.useStream {
		return rw.nonStreamWrite(data)
	}

	return rw.streamWrite(data)
}

func (rw *ResponseWriterWrapper) nonStreamWrite(data []byte) (int, error) {
	completion := openai.ChatCompletion{}
	err := json.Unmarshal(data, &completion)
	if err != nil {
		slog.Error("ResponseWriterWrapper nonStreamWrite unmarshal ChatCompletion error", slog.Any("err", err))
	} else {
		if rw.tokenCounter != nil {
			rw.tokenCounter.Completion(completion)
		}
		// call moderation service
		content := completion.Choices[0].Message.Content
		if rw.modSvcClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			result, err := rw.modSvcClient.PassTextCheck(ctx, string(sensitive.ScenarioLLMResModeration), content)
			cancel()
			if err != nil {
				slog.Error("ResponseWriterWrapper nonStreamWrite failed to call moderation service to check content sensitive", slog.Any("err", err))
			} else {
				if result.IsSensitive {
					slog.Debug("ResponseWriterWrapper nonStreamWrite checkresult is sensitive", slog.Any("content", content), slog.Any("reason", result.Reason))
					completion.Choices[0].Message.Content = "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation."
					completionJson, _ := json.Marshal(completion)
					return rw.internalWritter.Write(completionJson)
				}
			}
		}
	}
	return rw.internalWritter.Write(data)
}

func (rw *ResponseWriterWrapper) streamWrite(data []byte) (int, error) {
	events, _ := rw.eventStreamDecoder.Write(data)
	// unmarshal event data into ChatCompletionChunk and call moderation service
	for _, event := range events {
		if len(event.Data) > 0 {
			if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
				rw.writeInternal(event.Raw)
				return len(data), nil
			}
			// unmarshal event data into ChatCompletionChunk
			var chunk openai.ChatCompletionChunk
			err := json.Unmarshal(event.Data, &chunk)
			if err != nil {
				slog.Error("ResponseWriterWrapper streamWrite unmarshal error", slog.Any("err", err))
				rw.writeInternal(event.Raw)
				continue
			}
			if rw.tokenCounter != nil {
				rw.tokenCounter.AppendCompletionChunk(chunk)
			}
			if chunk.Choices[0].FinishReason != "" {
				rw.writeInternal(event.Raw)
				return len(data), nil
			}
			// call moderation service
			if chunk.Choices[0].Delta.Content == "" {
				rw.writeInternal(event.Raw)
				continue
			}

			if rw.modSvcClient == nil {
				rw.writeInternal(event.Raw)
				continue
			}

			result, err := rw.modStream(chunk.Choices[0].Delta.Content, stringToNumber(chunk.ID))
			if err != nil {
				slog.Error("ResponseWriterWrapper streamWrite modStream err", slog.String("content", chunk.Choices[0].Delta.Content), slog.Any("error", err))
				rw.writeInternal(event.Raw)
				continue
			}
			if result.IsSensitive {
				slog.Debug("ResponseWriterWrapper streamWrite checkresult is sensitive", slog.Any("content", chunk.Choices[0].Delta.Content), slog.Any("reason", result.Reason))
				errorChunk := rw.generateSensitiveResp(chunk)
				errorChunkJson, _ := json.Marshal(errorChunk)
				rw.writeInternal([]byte("data: " + string(errorChunkJson) + "\n\n"))
				return 0, ErrSensitiveContent
			}
			rw.writeInternal(event.Raw)
		} else {
			rw.writeInternal(event.Raw)
		}
	}

	return len(data), nil
}

func (rw *ResponseWriterWrapper) writeInternal(data []byte) {
	slog.Debug("writeInternal", slog.String("data", string(data)))
	_, err := rw.internalWritter.Write(data)
	if err != nil {
		slog.Error("write into internalWritter error:", slog.String("err", err.Error()))
	}
	rw.internalWritter.Flush()
}

// TODO: support different Chunk struct
// func (rw *ResponseWriterWrapper) getData(value []byte) (openai.ChatCompletionChunk, error) {
// 	var cur openai.ChatCompletionChunk
// 	ep := gjson.GetBytes(value, "error")
// 	if ep.Exists() {
// 		return openai.ChatCompletionChunk{}, fmt.Errorf("error while streaming: %v", ep.String())
// 	}
// 	err := json.Unmarshal(value, &cur)
// 	if err != nil {
// 		return openai.ChatCompletionChunk{}, err
// 	}
// 	return cur, nil
// }

func (rw *ResponseWriterWrapper) generateSensitiveResp(cur openai.ChatCompletionChunk) openai.ChatCompletionChunk {
	cur.Choices[0].Delta.Content = "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation."
	return cur
}

/*
	func (rw ResponseWriterWrapper) generateModErrorResp(cur openai.ChatCompletionChunk) openai.ChatCompletionChunk {
		cur.Choices[0].Delta.Content = "moderation server failed"
		return cur
	}
*/
func (rw *ResponseWriterWrapper) Flush() {
	rw.internalWritter.Flush()
}

func (rw *ResponseWriterWrapper) modStream(text, id string) (*rpc.CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := rw.modSvcClient.PassStreamCheck(ctx, string(sensitive.ScenarioLLMResModeration), text, id)
	if err != nil {
		return nil, fmt.Errorf("failed to call moderation service to check content sensitive: %w", err)
	}
	return result, nil
}

func stringToNumber(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
