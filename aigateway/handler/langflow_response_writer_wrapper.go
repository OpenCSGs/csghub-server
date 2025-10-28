package handler

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type LangflowResponseWriterWrapper struct {
	internalWritter    gin.ResponseWriter
	useStream          bool
	agentComponent     component.AgentComponent
	eventStreamDecoder *langflowEventStreamDecoder
}

var _ http.Hijacker = (*LangflowResponseWriterWrapper)(nil)

func NewLangflowResponseWriterWrapper(internalWritter gin.ResponseWriter, useStream bool, agentComponent component.AgentComponent) *LangflowResponseWriterWrapper {
	return &LangflowResponseWriterWrapper{
		internalWritter:    internalWritter,
		useStream:          useStream,
		agentComponent:     agentComponent,
		eventStreamDecoder: &langflowEventStreamDecoder{},
	}
}

func (rw *LangflowResponseWriterWrapper) Header() http.Header {
	return rw.internalWritter.Header()
}

func (rw *LangflowResponseWriterWrapper) WriteHeader(statusCode int) {
	rw.internalWritter.WriteHeader(statusCode)
}

func (rw *LangflowResponseWriterWrapper) Write(data []byte) (int, error) {
	if !rw.useStream {
		return rw.nonStreamWrite(data)
	}
	return rw.streamWrite(data)
}

func (rw *LangflowResponseWriterWrapper) Flush() {
	rw.internalWritter.Flush()
}

func (rw *LangflowResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return rw.internalWritter.Hijack()
}

func (rw *LangflowResponseWriterWrapper) nonStreamWrite(data []byte) (int, error) {
	var resp types.RunLangflowAgentInstanceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		slog.Error("Failed to unmarshal RunLangflowAgentInstanceResponse",
			slog.Any("err", err),
			slog.String("data", string(data)),
		)
		// Still write raw data downstream for client visibility
		return rw.internalWritter.Write(data)
	}

	// Validate and extract message safely
	outputs := resp.Outputs
	message := extractLangflowMessage(outputs)
	if message != "" {
		slog.Debug("Langflow nonStreamWrite message", slog.String("message", message))
		rw.recordSessionHistory(resp.SessionID, true, message)
	}
	return rw.internalWritter.Write(data)
}

func (rw *LangflowResponseWriterWrapper) streamWrite(data []byte) (int, error) {
	events, _ := rw.eventStreamDecoder.Write(data)
	for _, event := range events {
		slog.Debug("event", slog.String("event", event.Event.Event), slog.String("data", string(event.Event.Data)))
		switch event.Event.Event {
		case "token", "add_message":
			rw.writeInternal(event.Raw)
		case "end":
			var endData types.LangflowEndData
			if err := json.Unmarshal(event.Event.Data, &endData); err != nil {
				slog.Error("Failed to unmarshal LangflowEndData", slog.Any("err", err))
				rw.writeInternal(event.Raw)
				continue
			}
			outputs := endData.Result.Outputs
			msg := extractLangflowMessage(outputs)
			if msg != "" {
				slog.Debug("Langflow final message", slog.String("message", msg))
				rw.recordSessionHistory(endData.Result.SessionID, false, msg)
			} else {
				slog.Warn("Langflow end event has no text message", slog.Any("response", endData))
			}
			rw.writeInternal(event.Raw)
		default:
			slog.Warn("LangflowResponseWriterWrapper: unknown event type", slog.String("event", event.Event.Event))
			rw.writeInternal(event.Raw)
		}
	}
	return len(data), nil
}

func (rw *LangflowResponseWriterWrapper) writeInternal(data []byte) {
	_, err := rw.internalWritter.Write(data)
	if err != nil {
		slog.Error("write into internalWritter error:", slog.String("err", err.Error()))
	}
	rw.internalWritter.Flush()
}

func (rw *LangflowResponseWriterWrapper) recordSessionHistory(_ string, _ bool, _ string) {
}

func extractLangflowMessage(outputs []types.LangflowOutputs) string {
	if len(outputs) == 0 || len(outputs[0].Outputs) == 0 || outputs[0].Outputs[0].Results == nil {
		return ""
	}
	return outputs[0].Outputs[0].Results.Message.Text
}
