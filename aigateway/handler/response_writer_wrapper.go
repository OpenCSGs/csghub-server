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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type CommonResponseWriter interface {
	Header() http.Header
	WriteHeader(int)
	Write([]byte) (int, error)
	Flush()
	ClearBuffer()
}

var ErrSensitiveContent = errors.New("sensitive content detected")

var _ http.Hijacker = (*ResponseWriterWrapper)(nil)

type ResponseWriterWrapper struct {
	internalWritter     gin.ResponseWriter
	moderationComponent component.Moderation
	eventStreamDecoder  *eventStreamDecoder
	tokenCounter        token.ChatTokenCounter
	id                  string
}

// Hijack allows the HTTP connection upgrading to a different protocol, such as WebSockets or HTTP/2.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func NewResponseWriterWrapper(internalWritter gin.ResponseWriter, useStream bool, moderationComponent component.Moderation, tokenCounter token.ChatTokenCounter) CommonResponseWriter {
	if useStream {
		return newStreamResponseWriter(internalWritter, moderationComponent, tokenCounter)
	} else {
		return newNonStreamResponseWriter(internalWritter, moderationComponent, tokenCounter)
	}
}

func newStreamResponseWriter(internalWritter gin.ResponseWriter, moderationComponent component.Moderation, tokenCounter token.ChatTokenCounter) *ResponseWriterWrapper {
	id := uuid.New().ID()
	return &ResponseWriterWrapper{
		internalWritter:     internalWritter,
		moderationComponent: moderationComponent,
		tokenCounter:        tokenCounter,
		eventStreamDecoder:  &eventStreamDecoder{},
		id:                  fmt.Sprint(id),
	}
}

func (rw *ResponseWriterWrapper) ClearBuffer() {}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapper) Write(data []byte) (int, error) {
	return rw.streamWrite(data)
}

func (rw *ResponseWriterWrapper) streamWrite(data []byte) (int, error) {
	events, _ := rw.eventStreamDecoder.Write(data)
	// unmarshal event data into ChatCompletionChunk and call moderation service
	for _, event := range events {
		if len(event.Data) <= 0 {
			continue
		}
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
		// call moderation service
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := rw.moderationComponent.CheckChatStreamResponse(ctx, chunk, rw.id)
		if err != nil {
			slog.Error("ResponseWriterWrapper streamWrite checkChatResponse error", slog.Any("err", err))
			rw.writeInternal(event.Raw)
			continue
		}
		if result.IsSensitive {
			slog.Debug("ResponseWriterWrapper streamWrite checkresult is sensitive",
				slog.Any("content", chunk),
				slog.Any("reason", result.Reason))
			chunk = rw.generateSensitiveRespForContent(chunk)
			chunkJson, _ := json.Marshal(chunk)
			rw.writeInternal([]byte("data: " + string(chunkJson) + "\n\n"))
			rw.writeInternal([]byte("data: [DONE]\n\n"))
			return 0, ErrSensitiveContent
		}
		rw.writeInternal(event.Raw)
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

func generateInsufficientBalanceResp(frontendURL string) types.ChatCompletionChunk {
	rechargeURL := fmt.Sprintf("%s/settings/recharge-payment", frontendURL)
	message := fmt.Sprintf(
		"**Insufficient balance**\n\n👉 [Recharge your account](%s) to continue.",
		rechargeURL,
	)
	newChunk := types.ChatCompletionChunk{
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: message,
				},
				FinishReason: "insufficient_balance",
				Index:        0,
			},
		},
	}
	return newChunk
}

func (rw *ResponseWriterWrapper) Flush() {
	rw.internalWritter.Flush()
}
