package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestResponseWriterWrapper_StreamWrite(t *testing.T) {
	chunk1 := openai.ChatCompletionChunk{
		ID: "test-id",
		Choices: []openai.ChatCompletionChunkChoice{
			{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "  valid content  " + "\t\n"}},
		},
	}
	chunk1JSON, _ := json.Marshal(chunk1)
	chunk2 := openai.ChatCompletionChunk{
		ID: "test-id",
		Choices: []openai.ChatCompletionChunkChoice{
			{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "    " + "\t\n"}},
		},
	}
	chunk2JSON, _ := json.Marshal(chunk2)
	tests := []struct {
		name     string
		inputs   []byte
		wantResp string
	}{
		{
			name:     "valid content with whiteSapce",
			inputs:   []byte("data: " + string(chunk1JSON) + "\n\n"),
			wantResp: "data: " + string(chunk1JSON) + "\n\n",
		},
		{
			name:     "only whiteSapce",
			inputs:   []byte("data: " + string(chunk2JSON) + "\n\n"),
			wantResp: "data: " + string(chunk2JSON) + "\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			mockMod := component.NewMockModeration(t)
			wrapper := NewResponseWriterWrapper(ctx.Writer, true, mockMod, nil, nil)
			// only check response decode, not check moderation
			mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, mock.Anything, mock.Anything).
				Return(&rpc.CheckResult{IsSensitive: false}, nil)
			n, err := wrapper.Write(tt.inputs)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.inputs), n)
			assert.Equal(t, w.Body.String(), string(tt.wantResp))
		})
	}
}

func TestGenerateInsufficientBalanceResp(t *testing.T) {
	frontendURL := "http://localhost:8080"
	chunk := generateInsufficientBalanceResp(frontendURL)
	assert.Len(t, chunk.Choices, 1)
	assert.Equal(t, "insufficient_balance", chunk.Choices[0].FinishReason)
	assert.Contains(t, chunk.Choices[0].Delta.Content, "**Insufficient balance**")
	assert.Contains(t, chunk.Choices[0].Delta.Content, frontendURL+"/settings/recharge-payment")
}

func TestWriteSensitiveStreamResponse(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	resp := generateSensitiveRespForPrompt()

	writeSensitiveStreamResponse(ctx, resp)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "data: {")
	assert.Contains(t, body, "sensitive")
	assert.Contains(t, body, "[DONE]")
}

func TestWriteSensitiveJSONResponse(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	resp := generateSensitiveRespForPrompt()

	writeSensitiveJSONResponse(ctx, resp)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "sensitive")
	var parsedResp openai.ChatCompletionChunk
	err := json.Unmarshal([]byte(body), &parsedResp)
	assert.NoError(t, err)
	assert.Equal(t, "sensitive", parsedResp.Choices[0].FinishReason)
}

func TestHandleSensitiveResponse(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/", nil)

		checkResult := &rpc.CheckResult{Reason: "test reason"}
		handleSensitiveResponse(ctx, true, checkResult)

		assert.Equal(t, 200, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "data: {")
		assert.Contains(t, body, "sensitive")
		assert.Contains(t, body, "[DONE]")
	})

	t.Run("json", func(t *testing.T) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest("GET", "/", nil)

		checkResult := &rpc.CheckResult{Reason: "test reason"}
		handleSensitiveResponse(ctx, false, checkResult)

		assert.Equal(t, 200, w.Code)
		body := w.Body.String()
		var parsedResp openai.ChatCompletionChunk
		err := json.Unmarshal([]byte(body), &parsedResp)
		assert.NoError(t, err)
		assert.Equal(t, "sensitive", parsedResp.Choices[0].FinishReason)
	})
}

func TestResponseWriterWrapper_StreamWrite_WithWhiteSpace(t *testing.T) {
	chunk := openai.ChatCompletionChunk{
		ID: "test-id",
		Choices: []openai.ChatCompletionChunkChoice{
			{Delta: openai.ChatCompletionChunkChoiceDelta{Content: "    " + "\t\n"}},
		},
	}
	chunkJSON, _ := json.Marshal(chunk)
	tests := []struct {
		name     string
		inputs   []byte
		wantResp string
	}{
		{
			name:     "only whiteSapce",
			inputs:   []byte("data: " + string(chunkJSON) + "\n\n"),
			wantResp: "data: " + string(chunkJSON) + "\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			mockMod := component.NewMockModeration(t)
			wrapper := NewResponseWriterWrapper(ctx.Writer, true, mockMod, nil, nil)
			// only check response decode, not check moderation
			mockMod.EXPECT().CheckChatStreamResponse(mock.Anything, mock.Anything, mock.Anything).
				Return(&rpc.CheckResult{IsSensitive: false}, nil)
			n, err := wrapper.Write(tt.inputs)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.inputs), n)
			assert.Equal(t, w.Body.String(), string(tt.wantResp))
		})
	}
}
