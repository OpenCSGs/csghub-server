package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	rpcmock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
)

func setupTest(t *testing.T) (*OpenAIHandlerImpl, *mockcomp.MockOpenAIComponent, *apicomp.MockRepoComponent, *gin.Context, *httptest.ResponseRecorder) {
	mockOpenAI := mockcomp.NewMockOpenAIComponent(t)
	mockRepo := apicomp.NewMockRepoComponent(t)
	modSvcClient := rpcmock.NewMockModerationSvcClient(t)
	handler := NewOpenAIHandler(mockOpenAI, mockRepo, modSvcClient)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	// Set test user
	httpbase.SetCurrentUser(c, "testuser")

	return handler.(*OpenAIHandlerImpl), mockOpenAI, mockRepo, c, w
}

func TestOpenAIHandler_ListModels(t *testing.T) {

	t.Run("successful case", func(t *testing.T) {
		handler, mockOpenAI, _, c, w := setupTest(t)
		models := []types.Model{
			{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
		}
		mockOpenAI.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Equal(t, models, response.Data)
	})
}

func TestOpenAIHandler_GetModel(t *testing.T) {

	t.Run("model found", func(t *testing.T) {
		handler, mockOpenAI, _, c, w := setupTest(t)
		model := &types.Model{
			ID:      "model1:svc1",
			Object:  "model",
			OwnedBy: "testuser",
		}
		c.Params = []gin.Param{{Key: "model", Value: "model1:svc1"}}
		mockOpenAI.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)

		handler.GetModel(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.Model
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, model.ID, response.ID)
	})

	t.Run("model not found", func(t *testing.T) {
		handler, mockOpenAI, _, c, w := setupTest(t)
		c.Params = []gin.Param{{Key: "model", Value: "nonexistent:svc"}}
		mockOpenAI.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		handler.GetModel(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestOpenAIHandler_Chat(t *testing.T) {

	t.Run("invalid request body", func(t *testing.T) {
		handler, _, _, c, w := setupTest(t)
		c.Request.Method = http.MethodPost
		c.Request.Body = http.NoBody

		handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not found", func(t *testing.T) {
		handler, mockOpenAI, _, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "nonexistent:svc",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mockOpenAI.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not running", func(t *testing.T) {
		handler, mockOpenAI, _, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			ID:      "model1:svc1",
			Object:  "model",
			OwnedBy: "testuser",
		}
		mockOpenAI.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)

		handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
