package proxy

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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go"
	"github.com/tidwall/gjson"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
)

var ErrSensitiveContent = errors.New("sensitive content detected")

var _ http.Hijacker = (*ResponseWriterWrapper)(nil)

type ResponseWriterWrapper struct {
	internalWritter gin.ResponseWriter
	modSvcClient    rpc.ModerationSvcClient
	acceptType      string
}

// Hijack allows the HTTP connection upgrading to a different protocol, such as WebSockets or HTTP/2.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func NewResponseWriterWrapper(internalWritter gin.ResponseWriter, acceptType string) *ResponseWriterWrapper {
	slog.Debug("generate Wrapper", slog.String("content-type", acceptType))
	return &ResponseWriterWrapper{
		internalWritter: internalWritter,
		acceptType:      acceptType,
	}
}

func (rw *ResponseWriterWrapper) WithModeration(modSvcClient rpc.ModerationSvcClient) *ResponseWriterWrapper {
	rw.modSvcClient = modSvcClient
	return rw
}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *ResponseWriterWrapper) Write(data []byte) (int, error) {
	// contentType := rw.Header().Get("Content-Type")
	// if !strings.HasPrefix(contentType, "text/event-stream") {
	if rw.acceptType != "text/event-stream" {
		return rw.internalWritter.Write(data)
	}
	byteDatas := bytes.Split(data, []byte("\n\n"))
	for _, byteData := range byteDatas {
		if len(byteData) == 0 {
			continue
		}
		name, value, _ := bytes.Cut(byteData, []byte(":"))
		value = []byte(strings.TrimLeft(string(value), " "))
		switch string(name) {
		case "data":
			if bytes.HasPrefix(value, []byte("[DONE]")) {
				// In this case we don't break because we still want to iterate through the full stream.
				slog.Debug("get done", slog.String("content", string(byteData)))
				rw.writeInternal(append(byteData, '\n', '\n'))
				return len(data), nil
			}
			cur, err := rw.getData(value)
			if err != nil {
				slog.Debug("get content error", slog.Any("err", err))
				rw.writeInternal(append(byteData, '\n', '\n'))
				continue
			}
			if cur.Choices[0].Delta.Content == "" {
				rw.writeInternal(append(byteData, '\n', '\n'))
				continue
			}
			result, err := rw.modStream(cur.Choices[0].Delta.Content, stringToNumber(cur.ID))
			if err != nil {
				slog.Error("modStream err", slog.String("content", cur.Choices[0].Delta.Content), slog.Any("error", err))
				rw.writeInternal(append(byteData, '\n', '\n'))
				return len(data), nil
			}
			if result.IsSensitive {
				slog.Debug("checkresult is sensitive", slog.Any("content", cur.Choices[0].Delta.Content), slog.Any("reason", result.Reason))
				errorChunk := rw.generateSensitiveResp(cur)
				errorChunkJson, _ := json.Marshal(errorChunk)
				rw.writeInternal([]byte("data: " + string(errorChunkJson) + "\n\n"))
				rw.writeInternal([]byte("data: [DONE]\n\n"))
				return 0, ErrSensitiveContent
			}
			rw.writeInternal(append(byteData, '\n', '\n'))
			continue
		case "event":
			rw.writeInternal(append(byteData, '\n', '\n'))
		default:
			rw.writeInternal(append(byteData, '\n', '\n'))
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
func (rw *ResponseWriterWrapper) getData(value []byte) (openai.ChatCompletionChunk, error) {
	var cur openai.ChatCompletionChunk
	ep := gjson.GetBytes(value, "error")
	if ep.Exists() {
		return openai.ChatCompletionChunk{}, fmt.Errorf("error while streaming: %v", ep.String())
	}
	err := json.Unmarshal(value, &cur)
	if err != nil {
		return openai.ChatCompletionChunk{}, err
	}
	return cur, nil
}

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
