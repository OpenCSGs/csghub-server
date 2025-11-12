package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

type CommonResponseWriter interface {
	Header() http.Header
	WriteHeader(int)
	Write([]byte) (int, error)
	Flush()

	WithModeration(rpc.ModerationSvcClient)
	WithLLMTokenCounter(*token.ChatTokenCounter)
	ClearBuffer()
}

var ErrSensitiveContent = errors.New("sensitive content detected")

var _ http.Hijacker = (*ResponseWriterWrapper)(nil)

type ResponseWriterWrapper struct {
	internalWritter    gin.ResponseWriter
	originHeader       http.Header
	modSvcClient       rpc.ModerationSvcClient
	eventStreamDecoder *eventStreamDecoder
	tokenCounter       *token.ChatTokenCounter
	id                 string
}

// Hijack allows the HTTP connection upgrading to a different protocol, such as WebSockets or HTTP/2.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func NewResponseWriterWrapper(internalWritter gin.ResponseWriter, useStream bool) CommonResponseWriter {
	if useStream {
		return newStreamResponseWriter(internalWritter)
	} else {
		return newNonStreamResponseWriter(internalWritter)
	}
}

func newStreamResponseWriter(internalWritter gin.ResponseWriter) *ResponseWriterWrapper {
	id := uuid.New().ID()
	return &ResponseWriterWrapper{
		internalWritter:    internalWritter,
		eventStreamDecoder: &eventStreamDecoder{},
		id:                 fmt.Sprint(id),
	}
}

func (rw *ResponseWriterWrapper) WithModeration(modSvcClient rpc.ModerationSvcClient) {
	rw.modSvcClient = modSvcClient
}

func (rw *ResponseWriterWrapper) WithLLMTokenCounter(counter *token.ChatTokenCounter) {
	rw.tokenCounter = counter
}

func (rw *ResponseWriterWrapper) ClearBuffer() {}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.originHeader = rw.internalWritter.Header().Clone()
	rw.internalWritter.Header().Del("Content-Encoding")
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapper) Write(data []byte) (int, error) {
	return rw.streamWrite(data)
}

func (rw *ResponseWriterWrapper) streamWrite(data []byte) (int, error) {
	events, _ := rw.eventStreamDecoder.Write(data)
	// unmarshal event data into ChatCompletionChunk and call moderation service
	for _, event := range events {
		if len(event.Data) > 0 {
			if string(event.Data) == "[DONE]" {
				rw.writeInternal(event.Raw)
				return len(data), nil
			}
			// unmarshal event data into ChatCompletionChunk
			var chunk types.ChatCompletionChunk
			err := json.Unmarshal(event.Data, &chunk)
			if err != nil {
				slog.Error("ResponseWriterWrapper streamWrite unmarshal error", slog.Any("err", err))
				rw.writeInternal(event.Raw)
				continue
			}
			if rw.tokenCounter != nil {
				rw.tokenCounter.AppendCompletionChunk(chunk)
			}
			// skip moderation service for white space content and no moderation service
			if rw.skipModration(chunk) {
				rw.writeInternal(event.Raw)
				continue
			}
			if strings.TrimSpace(chunk.Choices[0].Delta.Content) != "" {
				// moderate on content
				result, err := rw.modStream(chunk.Choices[0].Delta.Content, rw.id)
				if err != nil {
					slog.Error("ResponseWriterWrapper streamWrite modStream err", slog.String("content", chunk.Choices[0].Delta.Content), slog.Any("error", err))
					rw.writeInternal(event.Raw)
					continue
				}
				if result.IsSensitive {
					slog.Debug("ResponseWriterWrapper streamWrite checkresult is sensitive", slog.Any("content", chunk.Choices[0].Delta.Content), slog.Any("reason", result.Reason))
					errorChunk := rw.generateSensitiveRespForContent(chunk)
					errorChunkJson, _ := json.Marshal(errorChunk)
					rw.writeInternal([]byte("data: " + string(errorChunkJson) + "\n\n"))
					rw.writeInternal([]byte("data: [DONE]\n\n"))
					return 0, ErrSensitiveContent
				}
				rw.writeInternal(event.Raw)
			} else if strings.TrimSpace(chunk.Choices[0].Delta.ReasoningContent) != "" {
				// moderate on reasoning content
				result, err := rw.modStream(chunk.Choices[0].Delta.ReasoningContent, rw.id)
				if err != nil {
					slog.Error("ResponseWriterWrapper streamWrite modStream err", slog.String("content", chunk.Choices[0].Delta.ReasoningContent), slog.Any("error", err))
					rw.writeInternal(event.Raw)
					continue
				}
				if result.IsSensitive {
					slog.Debug("ResponseWriterWrapper streamWrite checkresult is sensitive", slog.Any("content", chunk.Choices[0].Delta.ReasoningContent), slog.Any("reason", result.Reason))
					errorChunk := rw.generateSensitiveRespForReasoningContent(chunk)
					errorChunkJson, _ := json.Marshal(errorChunk)
					rw.writeInternal([]byte("data: " + string(errorChunkJson) + "\n\n"))
					rw.writeInternal([]byte("data: [DONE]\n\n"))
					return 0, ErrSensitiveContent
				}
				rw.writeInternal(event.Raw)
			} else {
				slog.Error("Unknown data struct",
					slog.Any("raw data", event.Raw),
					slog.Any("unmarshal chunk", chunk))
				rw.writeInternal(event.Raw)
			}
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

func (rw *ResponseWriterWrapper) generateSensitiveRespForContent(curChunk types.ChatCompletionChunk) types.ChatCompletionChunk {
	newChunk := types.ChatCompletionChunk{
		ID:    curChunk.ID,
		Model: curChunk.Model,
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation.",
				},
				FinishReason: "sensitive",
				Index:        curChunk.Choices[0].Index,
			},
		},
		SystemFingerprint: curChunk.SystemFingerprint,
		Object:            curChunk.Object,
		Usage:             curChunk.Usage,
	}
	return newChunk
}

func generateSensitiveRespForPrompt() types.ChatCompletionChunk {
	newChunk := types.ChatCompletionChunk{
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "The prompt includes inappropriate content and has been blocked. We appreciate your understanding and cooperation.",
				},
				FinishReason: "sensitive",
				Index:        0,
			},
		},
	}
	return newChunk
}

func (rw *ResponseWriterWrapper) generateSensitiveRespForReasoningContent(curChunk types.ChatCompletionChunk) types.ChatCompletionChunk {
	newChunk := types.ChatCompletionChunk{
		ID:    curChunk.ID,
		Model: curChunk.Model,
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					ReasoningContent: "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation.",
				},
				FinishReason: "sensitive",
				Index:        curChunk.Choices[0].Index,
			},
		},
		SystemFingerprint: curChunk.SystemFingerprint,
		Object:            curChunk.Object,
		Usage:             curChunk.Usage,
	}
	return newChunk
}

func (rw *ResponseWriterWrapper) Flush() {
	rw.internalWritter.Flush()
}

func (rw *ResponseWriterWrapper) modStream(text, id string) (*rpc.CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := rw.modSvcClient.PassLLMRespCheck(ctx, text, id)
	if err != nil {
		return nil, fmt.Errorf("failed to call moderation service to check content sensitive: %w", err)
	}
	return result, nil
}

func (rw *ResponseWriterWrapper) skipModration(chunk types.ChatCompletionChunk) bool {
	if rw.modSvcClient == nil {
		return true
	}
	if len(chunk.Choices) == 0 {
		return true
	}
	if strings.TrimSpace(chunk.Choices[0].Delta.Content) == "" && strings.TrimSpace(chunk.Choices[0].Delta.ReasoningContent) == "" {
		return true
	}
	return false
}
