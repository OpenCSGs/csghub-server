package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	aicomponent "opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
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
			wrapper := NewResponseWriterWrapper(ChatWriterParams{InternalWriter: ctx.Writer, Stream: true, ModerationComponent: mockMod, RequestCtx: context.Background()})
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
			wrapper := NewResponseWriterWrapper(ChatWriterParams{InternalWriter: ctx.Writer, Stream: true, ModerationComponent: mockMod, RequestCtx: context.Background()})
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

func TestResponseWriterWrapper_StreamWrite_FallbackReasoningModeration(t *testing.T) {
	// The real moderation component runs; only the moderation service is
	// stubbed. The stub flags text only when it contains the fallback marker.
	newModeration := func(t *testing.T, seen *[]string) aicomponent.Moderation {
		client := mockrpc.NewMockModerationSvcClient(t)
		client.EXPECT().PassLLMRespCheck(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
				*seen = append(*seen, req.Text)
				return &rpc.CheckResult{IsSensitive: strings.Contains(req.Text, "LEAKED-FALLBACK"), Reason: "toxic"}, nil
			})
		return aicomponent.NewModerationImplWithClient(&config.Config{}, client, nil)
	}
	event := func(delta string) []byte {
		return []byte(`data: {"id":"test-id","choices":[{"index":0,"delta":` + delta + `}]}` + "\n\n")
	}

	t.Run("hit on fallback field blocks the original chunk", func(t *testing.T) {
		var seen []string
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		wrapper := NewResponseWriterWrapper(ChatWriterParams{InternalWriter: ctx.Writer, Stream: true, ModerationComponent: newModeration(t, &seen), RequestCtx: context.Background()})

		_, err := wrapper.Write(event(`{"reasoning_content":"harmless","reasoning":"LEAKED-FALLBACK"}`))

		assert.ErrorIs(t, err, ErrSensitiveContent)
		assert.Len(t, seen, 1)
		assert.Contains(t, seen[0], "LEAKED-FALLBACK")
		body := w.Body.String()
		assert.NotContains(t, body, "LEAKED-FALLBACK")
		assert.NotContains(t, body, "harmless")
		assert.Contains(t, body, `"finish_reason":"sensitive"`)
		assert.Contains(t, body, "data: [DONE]")
	})

	t.Run("hit on fallback field with empty primary blocks the chunk", func(t *testing.T) {
		var seen []string
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		wrapper := NewResponseWriterWrapper(ChatWriterParams{InternalWriter: ctx.Writer, Stream: true, ModerationComponent: newModeration(t, &seen), RequestCtx: context.Background()})

		_, err := wrapper.Write(event(`{"reasoning":"LEAKED-FALLBACK"}`))

		assert.ErrorIs(t, err, ErrSensitiveContent)
		assert.NotContains(t, w.Body.String(), "LEAKED-FALLBACK")
		assert.Contains(t, w.Body.String(), `"finish_reason":"sensitive"`)
	})

	t.Run("moderation disabled leaves the event unchanged", func(t *testing.T) {
		input := event(`{"reasoning_content":"harmless","reasoning":"LEAKED-FALLBACK"}`)
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		wrapper := NewResponseWriterWrapper(ChatWriterParams{InternalWriter: ctx.Writer, Stream: true, RequestCtx: context.Background()})

		n, err := wrapper.Write(input)

		assert.NoError(t, err)
		assert.Equal(t, len(input), n)
		assert.Equal(t, string(input), w.Body.String())
	})
}
