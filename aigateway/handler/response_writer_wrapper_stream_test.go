package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestResponseWriterWrapper_NewResponseWriterWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	rw := newStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))
	if rw == nil {
		t.Fatal("NewResponseWriterWrapper should not return nil")
	}
	if rw.internalWritter != ctx.Writer {
		t.Error("internalWritter should be set correctly")
	}
	if rw.eventStreamDecoder == nil {
		t.Error("eventStreamDecoder should be initialized")
	}
	if rw.id == "" {
		t.Error("id should be set")
	}
}

func TestResponseWriterWrapper_Header_WriteHeader_Flush_ClearBuffer(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	rw := newStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

	rw.WriteHeader(http.StatusOK)
	if w.Code != http.StatusOK {
		t.Error("WriteHeader should set status code correctly")
	}
	rw.Flush()
	rw.ClearBuffer()
}

func TestResponseWriterWrapper_Write_NormalContent(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockMod := component.NewMockModeration(t)
	rw := newStreamResponseWriter(ctx.Writer, mockMod)

	normalChunk := types.ChatCompletionChunk{
		ID:     "test-id",
		Object: "chat.completion.chunk",
		Model:  "test-model",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "Hello, this is a normal content.",
				},
			},
		},
	}

	chunkJSON, _ := json.Marshal(normalChunk)
	streamData := []byte("data: " + string(chunkJSON) + "\n\n")
	expectChunk := types.ChatCompletionChunk{}
	_ = json.Unmarshal(chunkJSON, &expectChunk)
	mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, expectChunk, rw.id).
		Return(&rpc.CheckResult{IsSensitive: false}, nil)
	_, err := rw.Write(streamData)
	if err != nil {
		t.Errorf("Write should not return error for normal content: %v", err)
	}
	if !bytes.Contains(w.Body.Bytes(), streamData) {
		t.Error("Normal content should be written to response")
	}
}

func TestResponseWriterWrapper_Write_DoneMessage(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	rw := newStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

	doneData := []byte("data: [DONE]\n\n")
	_, err := rw.Write(doneData)
	if err != nil {
		t.Errorf("Write should not return error for DONE message: %v", err)
	}

	if !bytes.Contains(w.Body.Bytes(), doneData) {
		t.Error("DONE message should be written to response")
	}
}

func TestResponseWriterWrapper_Write_SensitiveContent(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockMod := component.NewMockModeration(t)
	rw := newStreamResponseWriter(ctx.Writer, mockMod)

	sensitiveChunk := types.ChatCompletionChunk{
		ID:     "test-id",
		Object: "chat.completion.chunk",
		Model:  "test-model",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "sensitive content",
				},
			},
		},
	}

	chunkJSON, _ := json.Marshal(sensitiveChunk)
	streamData := []byte("data: " + string(chunkJSON) + "\n\n")
	expectChunk := types.ChatCompletionChunk{}
	_ = json.Unmarshal(chunkJSON, &expectChunk)
	mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, expectChunk, rw.id).
		Return(&rpc.CheckResult{IsSensitive: true}, nil)
	_, err := rw.Write(streamData)
	if !errors.Is(err, ErrSensitiveContent) {
		t.Errorf("Write should return ErrSensitiveContent for sensitive content, got: %v", err)
	}
	responseBody := w.Body.String()
	if !bytes.Contains([]byte(responseBody), []byte("The message includes inappropriate content")) {
		t.Error("Response should include sensitive content warning")
	}
	if !bytes.Contains([]byte(responseBody), []byte("[DONE]")) {
		t.Error("Response should include DONE message after sensitive content")
	}
}

func TestResponseWriterWrapper_Write_SensitiveReasoningContent(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockMod := component.NewMockModeration(t)
	rw := newStreamResponseWriter(ctx.Writer, mockMod)
	sensitiveChunk := types.ChatCompletionChunk{
		ID:     "test-id",
		Object: "chat.completion.chunk",
		Model:  "test-model",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: types.ChatCompletionChunkChoiceDelta{
					ReasoningContent: "sensitive reasoning",
				},
			},
		},
	}

	chunkJSON, _ := json.Marshal(sensitiveChunk)
	streamData := []byte("data: " + string(chunkJSON) + "\n\n")
	expectChunk := types.ChatCompletionChunk{}
	_ = json.Unmarshal(chunkJSON, &expectChunk)
	mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, expectChunk, rw.id).
		Return(&rpc.CheckResult{IsSensitive: true}, nil)
	_, err := rw.Write(streamData)
	if !errors.Is(err, ErrSensitiveContent) {
		t.Errorf("Write should return ErrSensitiveContent for sensitive reasoning content, got: %v", err)
	}

	responseBody := w.Body.String()
	if !bytes.Contains([]byte(responseBody), []byte("The message includes inappropriate content")) {
		t.Error("Response should include sensitive content warning")
	}
	if !bytes.Contains([]byte(responseBody), []byte("[DONE]")) {
		t.Error("Response should include DONE message after sensitive content")
	}
}

func TestResponseWriterWrapper_Write_ModerationServiceError(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockMod := component.NewMockModeration(t)

	rw := newStreamResponseWriter(ctx.Writer, mockMod)
	testChunk := types.ChatCompletionChunk{
		ID:     "test-id",
		Object: "chat.completion.chunk",
		Model:  "test-model",
		Choices: []types.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "test content",
				},
			},
		},
	}

	chunkJSON, _ := json.Marshal(testChunk)
	streamData := []byte("data: " + string(chunkJSON) + "\n\n")
	expectChunk := types.ChatCompletionChunk{}
	_ = json.Unmarshal(chunkJSON, &expectChunk)
	mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, expectChunk, rw.id).
		Return(nil, errors.New("moderation error"))
	_, err := rw.Write(streamData)
	if err != nil {
		t.Errorf("Write should not return error when moderation service fails: %v", err)
	}

	if !bytes.Contains(w.Body.Bytes(), streamData) {
		t.Error("Original data should be written when moderation service fails")
	}
}

func TestResponseWriterWrapper_Write_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	rw := newStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

	invalidData := []byte("data: invalid json data\n\n")

	_, err := rw.Write(invalidData)
	if err != nil {
		t.Errorf("Write should not return error for invalid JSON: %v", err)
	}

	if !bytes.Contains(w.Body.Bytes(), invalidData) {
		t.Error("Invalid data should be written when JSON parsing fails")
	}
}
