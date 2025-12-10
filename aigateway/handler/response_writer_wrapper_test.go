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
			wrapper := NewResponseWriterWrapper(ctx.Writer, true, mockMod, nil)
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
			wrapper := NewResponseWriterWrapper(ctx.Writer, true, mockMod, nil)
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
