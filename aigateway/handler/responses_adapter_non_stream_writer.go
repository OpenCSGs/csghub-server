package handler

import (
	"context"
	"encoding/json"
	"net/http"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type responsesAdapterNonStreamWriter struct {
	*bufferCommonResponseWriter
	ginWriter        gin.ResponseWriter
	model            string
	responsesCounter token.ResponsesTokenCounter
	moderation       component.Moderation
	sessionID        string
	logCapture       *responsespkg.LLMLogRecorder
}

func newResponsesAdapterNonStreamWriter(w gin.ResponseWriter, model string, responsesCounter token.ResponsesTokenCounter, moderation component.Moderation, sessionID string, logCapture ...*responsespkg.LLMLogRecorder) *responsesAdapterNonStreamWriter {
	var recorder *responsespkg.LLMLogRecorder
	if len(logCapture) > 0 {
		recorder = logCapture[0]
	}
	return &responsesAdapterNonStreamWriter{
		bufferCommonResponseWriter: newBufferCommonResponseWriter(),
		ginWriter:                  w,
		model:                      model,
		responsesCounter:           responsesCounter,
		moderation:                 moderation,
		sessionID:                  sessionID,
		logCapture:                 recorder,
	}
}

func (w *responsesAdapterNonStreamWriter) Finalize(statusCode int) error {
	if statusCode >= http.StatusBadRequest {
		copyHeader(w.ginWriter.Header(), w.Header())
		w.ginWriter.WriteHeader(statusCode)
		_, err := w.ginWriter.Write(w.body.Bytes())
		return err
	}

	chatBody, err := decodeResponsesAdapterChatBody(w.bufferCommonResponseWriter)
	if err != nil {
		return err
	}
	ctx, cancel := responsespkg.ModerationContext()
	sensitive := w.checkNonStreamSensitive(ctx, chatBody)
	cancel()
	if sensitive {
		return nil
	}
	resp, err := chatResponseToResponses(chatBody, w.model)
	if err != nil {
		return err
	}
	if w.responsesCounter != nil {
		w.responsesCounter.Response(resp)
	}
	if w.logCapture != nil {
		w.logCapture.CaptureResponse(resp)
		var chat types.ChatCompletion
		if err := json.Unmarshal(chatBody, &chat); err == nil {
			for _, choice := range chat.Choices {
				if choice.FinishReason != "" {
					w.logCapture.CaptureFinishReason(string(choice.FinishReason))
				}
			}
		}
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	w.ginWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.ginWriter.WriteHeader(http.StatusOK)
	_, err = w.ginWriter.Write(body)
	return err
}

// checkNonStreamSensitive inspects the upstream chat completion body and,
// when sensitive content is detected, writes the canned blocked Responses
// JSON response in place of the normal payload. Returns true if the writer
// has already emitted the canned response and the caller should stop.
func (w *responsesAdapterNonStreamWriter) checkNonStreamSensitive(ctx context.Context, chatBody []byte) bool {
	if w.moderation == nil {
		return false
	}
	var chat types.ChatCompletion
	if err := json.Unmarshal(chatBody, &chat); err != nil {
		return false
	}
	if len(chat.Choices) == 0 {
		return false
	}
	content := chatCompletionModerationText(chat)
	if strings.TrimSpace(content) == "" {
		return false
	}
	result, err := w.moderation.CheckText(ctx, types.TextModerationRequest{
		Content: content,
		Key:     w.sessionID,
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeNonStream,
	})
	if err != nil || result == nil {
		return false
	}
	if !result.IsSensitive {
		return false
	}
	w.ginWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.ginWriter.WriteHeader(http.StatusOK)
	canned := types.ResponsesResponse{
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: responsespkg.BlockedMessage,
			}},
		}},
	}
	body, err := json.Marshal(canned)
	if err != nil {
		return false
	}
	_, _ = w.ginWriter.Write(body)
	return true
}

func chatCompletionModerationText(chat types.ChatCompletion) string {
	var b strings.Builder
	for _, choice := range chat.Choices {
		msg := choice.Message
		writeModerationText(&b, msg.Content)
		writeModerationText(&b, msg.Refusal)
		writeLegacyFunctionCallModerationText(&b, msg)
		for _, call := range msg.ToolCalls {
			writeModerationText(&b, call.Function.Name)
			writeModerationText(&b, call.Function.Arguments)
			writeModerationText(&b, call.Custom.Name)
			writeModerationText(&b, call.Custom.Input)
		}
	}
	return b.String()
}

func writeLegacyFunctionCallModerationText(b *strings.Builder, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	var raw struct {
		FunctionCall *struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function_call"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.FunctionCall == nil {
		return
	}
	writeModerationText(b, raw.FunctionCall.Name)
	writeModerationText(b, raw.FunctionCall.Arguments)
}
