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
	rpc "opencsg.com/csghub-server/builder/rpc"
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
	recorder            component.LLMLogRecorder
	id                  string
}

// Hijack allows the HTTP connection upgrading to a different protocol, such as WebSockets or HTTP/2.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func NewResponseWriterWrapper(internalWritter gin.ResponseWriter, useStream bool, moderationComponent component.Moderation, tokenCounter token.ChatTokenCounter, recorder component.LLMLogRecorder) CommonResponseWriter {
	if useStream {
		return newStreamResponseWriter(internalWritter, moderationComponent, tokenCounter, recorder)
	} else {
		return newNonStreamResponseWriter(internalWritter, moderationComponent, tokenCounter, recorder)
	}
}

func newStreamResponseWriter(internalWritter gin.ResponseWriter, moderationComponent component.Moderation, tokenCounter token.ChatTokenCounter, recorder component.LLMLogRecorder) *ResponseWriterWrapper {
	id := uuid.New().ID()
	return &ResponseWriterWrapper{
		internalWritter:     internalWritter,
		moderationComponent: moderationComponent,
		tokenCounter:        tokenCounter,
		eventStreamDecoder:  &eventStreamDecoder{},
		recorder:            recorder,
		id:                  fmt.Sprint(id),
	}
}

func (rw *ResponseWriterWrapper) ClearBuffer() {}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.internalWritter.Header().Del("Content-Length")
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
			// trigger async check for sensitive content on remaining buffer
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := rw.closeStreamCheck(ctx, rw.id)
			if err != nil {
				slog.Error("ResponseWriterWrapper streamWrite closeStreamCheck error", slog.Any("err", err))
				rw.writeInternal(event.Raw)
				continue
			}
			if res != nil && res.IsSensitive {
				return rw.handleSensitiveResult(res, event.Raw, types.ChatCompletionChunk{})
			}

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
		if rw.recorder != nil {
			rw.recorder.AppendCompletionChunk(chunk)
		}
		// call moderation service
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := rw.checkChatStreamResponse(ctx, chunk, rw.id)
		if err != nil {
			slog.Error("ResponseWriterWrapper streamWrite checkChatStreamResponse error", slog.Any("err", err))
			rw.writeInternal(event.Raw)
			continue
		}
		if result != nil && result.IsSensitive {
			return rw.handleSensitiveResult(result, event.Raw, chunk)
		}
		rw.writeInternal(event.Raw)
	}

	return len(data), nil
}

func (rw *ResponseWriterWrapper) closeStreamCheck(ctx context.Context, id string) (*rpc.CheckResult, error) {
	if rw.moderationComponent == nil {
		return nil, nil
	}
	return rw.moderationComponent.CloseStreamCheck(ctx, id)
}

func (rw *ResponseWriterWrapper) checkChatStreamResponse(ctx context.Context, chunk types.ChatCompletionChunk, id string) (*rpc.CheckResult, error) {
	if rw.moderationComponent == nil {
		return nil, nil
	}
	return rw.moderationComponent.CheckChatStreamResponse(ctx, chunk, id)
}

func (rw *ResponseWriterWrapper) handleSensitiveResult(result *rpc.CheckResult, rawData []byte, chunk types.ChatCompletionChunk) (int, error) {
	slog.Debug("ResponseWriterWrapper streamWrite checkresult is sensitive",
		slog.Any("content", chunk),
		slog.Any("reason", result.Reason))
	chunk = rw.generateSensitiveRespForContent(chunk)
	chunkJson, _ := json.Marshal(chunk)
	rw.writeInternal([]byte("data: " + string(chunkJson) + "\n\n"))
	rw.writeInternal([]byte("data: [DONE]\n\n"))
	return 0, ErrSensitiveContent
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
	var index int64 = 0
	if len(curChunk.Choices) > 0 {
		index = curChunk.Choices[0].Index
	}
	newChunk := types.ChatCompletionChunk{
		ID:    curChunk.ID,
		Model: curChunk.Model,
		Choices: []types.ChatCompletionChunkChoice{
			{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "The message includes inappropriate content and has been blocked. We appreciate your understanding and cooperation.",
				},
				FinishReason: "sensitive",
				Index:        index,
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

func handleSensitiveResponse(c *gin.Context, stream bool, checkResult *rpc.CheckResult) {
	slog.DebugContext(
		c.Request.Context(),
		"sensitive content detected",
		slog.String("reason", checkResult.Reason),
	)

	resp := generateSensitiveRespForPrompt()
	if stream {
		writeSensitiveStreamResponse(c, resp)
		return
	}
	writeSensitiveJSONResponse(c, resp)
}

func writeSensitiveStreamResponse(c *gin.Context, resp any) {
	errorChunkJson, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "marshal error:", slog.String("err", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
	_, err = c.Writer.Write([]byte("data: " + string(errorChunkJson) + "\n\n" + "[DONE]"))
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "write into resp error:", slog.String("err", err.Error()))
	}
	c.Writer.Flush()
}

func writeSensitiveJSONResponse(c *gin.Context, resp any) {
	c.JSON(http.StatusOK, resp)
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
