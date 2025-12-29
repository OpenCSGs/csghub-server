package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
)

type testerOpenAIHandler struct {
	*testutil.GinTester
	mocks struct {
		openAIComp          *mockcomp.MockOpenAIComponent
		moderationComp      *mockcomp.MockModeration
		repoComp            *apicomp.MockRepoComponent
		mockClsComp         *apicomp.MockClusterComponent
		tokenCounterFactory *mocktoken.MockCounterFactory
	}

	handler *OpenAIHandlerImpl
}

func setupTest(t *testing.T) (*testerOpenAIHandler, *gin.Context, *httptest.ResponseRecorder) {
	mockOpenAI := mockcomp.NewMockOpenAIComponent(t)
	mockRepo := apicomp.NewMockRepoComponent(t)
	mockModeration := mockcomp.NewMockModeration(t)
	mockClsComp := apicomp.NewMockClusterComponent(t)
	mockTokenCounterFactory := mocktoken.NewMockCounterFactory(t)
	handler := newOpenAIHandler(mockOpenAI, mockRepo, mockModeration, mockClsComp, mockTokenCounterFactory)

	// Set test user
	tester := &testerOpenAIHandler{
		GinTester: testutil.NewGinTester(),
		handler:   handler,
	}
	w := tester.GinTester.Response()
	c := tester.GinTester.Gctx()
	httpbase.SetCurrentUser(c, "testuser")
	httpbase.SetCurrentUserUUID(c, "testuuid")
	tester.mocks.moderationComp = mockModeration
	tester.mocks.openAIComp = mockOpenAI
	tester.mocks.repoComp = mockRepo
	tester.mocks.mockClsComp = mockClsComp
	tester.mocks.tokenCounterFactory = mockTokenCounterFactory
	return tester, c, w
}

func TestOpenAIHandler_ListModels(t *testing.T) {

	t.Run("successful case", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "model1:svc1",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Equal(t, models, response.Data)
	})

	t.Run("fuzzy search with matching model_id", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "gpt-4:svc1",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
			{
				BaseModel: types.BaseModel{
					ID:      "claude-3:svc2",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)
		tester.WithQuery("model_id", "gpt")
		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 1)
		assert.Equal(t, "gpt-4:svc1", response.Data[0].ID)
	})

	t.Run("fuzzy search case insensitive", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "GPT-4:svc1",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		c.Request.URL, _ = url.Parse("/v1/models?model_id=gpt")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Len(t, response.Data, 1)
		assert.Equal(t, "GPT-4:svc1", response.Data[0].ID)
	})

	t.Run("fuzzy search with no matches", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "gpt-4:svc1",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		c.Request.URL, _ = url.Parse("/v1/models?model_id=nonexistent")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 0)
	})

	t.Run("no search query returns all models", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "model1:svc1",
					Object:  "model",
					OwnedBy: "testuser",
				},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		c.Request.URL, _ = url.Parse("/v1/models")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Equal(t, models, response.Data)
	})

	t.Run("pagination with default parameters", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model2:svc2", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model3:svc3", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 3)
		assert.Equal(t, 3, response.TotalCount)
		assert.False(t, response.HasMore)
		assert.Equal(t, "model1:svc1", *response.FirstID)
		assert.Equal(t, "model3:svc3", *response.LastID)
	})

	t.Run("pagination with per_page parameter", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model2:svc2", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model3:svc3", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model4:svc4", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model5:svc5", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("per", "2")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)
		assert.Equal(t, 5, response.TotalCount)
		assert.True(t, response.HasMore)
		assert.Equal(t, "model1:svc1", *response.FirstID)
		assert.Equal(t, "model2:svc2", *response.LastID)
	})

	t.Run("pagination with page and per_page parameters", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model2:svc2", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model3:svc3", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model4:svc4", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model5:svc5", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("page", "2").WithQuery("per", "2")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)
		assert.Equal(t, 5, response.TotalCount)
		assert.True(t, response.HasMore)
		assert.Equal(t, "model3:svc3", *response.FirstID)
		assert.Equal(t, "model4:svc4", *response.LastID)
	})

	t.Run("pagination last page", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model2:svc2", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model3:svc3", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model4:svc4", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model5:svc5", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("page", "3").WithQuery("per", "2")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 1)
		assert.Equal(t, 5, response.TotalCount)
		assert.False(t, response.HasMore)
		assert.Equal(t, "model5:svc5", *response.FirstID)
		assert.Equal(t, "model5:svc5", *response.LastID)
	})

	t.Run("pagination with page beyond available data", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "model2:svc2", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("page", "3").WithQuery("per", "2")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 0)
		assert.Equal(t, 2, response.TotalCount)
		assert.False(t, response.HasMore)
		assert.Nil(t, response.FirstID)
		assert.Nil(t, response.LastID)
	})

	t.Run("pagination with per_page limit capped at 100", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := make([]types.Model, 150)
		for i := 0; i < 150; i++ {
			models[i] = types.Model{
				BaseModel: types.BaseModel{
					ID:      fmt.Sprintf("model%d:svc%d", i+1, i+1),
					Object:  "model",
					OwnedBy: "testuser",
				},
			}
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("per", "200")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 100) // Should be capped at 100
		assert.Equal(t, 150, response.TotalCount)
		assert.True(t, response.HasMore)
	})

	t.Run("pagination with search and per_page", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "gpt-4:svc1", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "gpt-3.5:svc2", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "claude-3:svc3", Object: "model", OwnedBy: "testuser"}},
			{BaseModel: types.BaseModel{ID: "gpt-3.5-turbo:svc4", Object: "model", OwnedBy: "testuser"}},
		}
		tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

		tester.WithQuery("model_id", "gpt").WithQuery("per", "2")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)         // Only gpt models, limited to 2
		assert.Equal(t, 3, response.TotalCount) // 3 gpt models total
		assert.True(t, response.HasMore)
		assert.Equal(t, "gpt-4:svc1", *response.FirstID)
		assert.Equal(t, "gpt-3.5:svc2", *response.LastID)
	})
}

func TestOpenAIHandler_ListModels_OpenaiSDK(t *testing.T) {
	// Setup test with mock data
	tester, _, _ := setupTest(t)

	// Prepare mock models
	models := []types.Model{
		{
			BaseModel: types.BaseModel{
				ID:      "gpt-4:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
		},
		{
			BaseModel: types.BaseModel{
				ID:      "gpt-3.5-turbo:svc2",
				Object:  "model",
				OwnedBy: "testuser",
			},
		},
	}

	// Set up mock expectation
	tester.mocks.openAIComp.EXPECT().GetAvailableModels(mock.Anything, "testuser").Return(models, nil)

	// Create gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add middleware to set current user (similar to how it's done in the actual router)
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentUserUUID(c, "testuuid")
		c.Next()
	})

	// Set up the route
	router.GET("/v1/models", tester.handler.ListModels)

	// Start test server
	server := httptest.NewServer(router)
	defer server.Close()

	// Create OpenAI client with the test server URL
	client := openai.NewClient(option.WithAPIKey("test-api-key"), option.WithBaseURL(server.URL+"/v1"))

	// Call the ListModels endpoint using OpenAI SDK
	ctx := context.Background()
	modelList, err := client.Models.List(ctx)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, modelList)
	assert.Equal(t, "list", modelList.Object)
	assert.Len(t, modelList.Data, 2)
	// Verify model IDs
	modelIDs := make([]string, len(modelList.Data))
	for i, model := range modelList.Data {
		modelIDs[i] = model.ID
	}
	assert.Contains(t, modelIDs, "gpt-4:svc1")
	assert.Contains(t, modelIDs, "gpt-3.5-turbo:svc2")

	// get next page
	nextPage, err := modelList.GetNextPage()
	assert.NoError(t, err)
	assert.Nil(t, nextPage)
}

func TestOpenAIHandler_GetModel(t *testing.T) {

	t.Run("model found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
		}
		c.Params = []gin.Param{{Key: "model", Value: "model1:svc1"}}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)

		tester.handler.GetModel(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.Model
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, model.ID, response.ID)
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Params = []gin.Param{{Key: "model", Value: "nonexistent:svc"}}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		tester.handler.GetModel(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestOpenAIHandler_Chat(t *testing.T) {

	t.Run("invalid request body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request.Method = http.MethodPost
		c.Request.Body = http.NoBody

		tester.handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "nonexistent:svc",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		tester.handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not running", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)

		tester.handler.Chat(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("llm prompt sensitive detected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
			Endpoint: "test-endpoint",
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID).
			Return(&rpc.CheckResult{IsSensitive: true}, nil)
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("llm prompt sensitive check failed", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Create a test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple handler that doesn't need to do anything for this test
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID).
			Return(nil, errors.New("some error"))
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("success", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Create a test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple handler that doesn't need to do anything for this test
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, llmTokenCounter).
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, counter token.Counter) error {
				wg.Done()
				return nil
			})
		tester.handler.Chat(c)
		wg.Wait()
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("record usage error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Create a test server
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simple handler that doesn't need to do anything for this test
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, llmTokenCounter).
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, counter token.Counter) error {
				wg.Done()
				return errors.New("record usage error")
			})
		tester.handler.Chat(c)
		wg.Wait()
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestOpenAIHandler_Embedding(t *testing.T) {
	t.Run("invalid request body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request.Method = http.MethodPost
		c.Request.Body = http.NoBody

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty input or model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		// Empty Input
		embeddingReq := EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "model1:svc1",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{},
				},
			},
		}
		body, _ := json.Marshal(embeddingReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Empty Model
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = &http.Request{
			Header: make(http.Header),
			Method: http.MethodPost,
		}
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentUserUUID(c, "testuuid")
		embeddingReq = EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{"test input"},
				},
			},
		}
		body, _ = json.Marshal(embeddingReq)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		embeddingReq := EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "nonexistent:svc",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{"test input"},
				},
			},
		}
		body, _ := json.Marshal(embeddingReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("get model error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		embeddingReq := EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "model1:svc1",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{"test input"},
				},
			},
		}
		body, _ := json.Marshal(embeddingReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(nil, errors.New("internal error"))

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("model not running", func(t *testing.T) {
		tester, c, w := setupTest(t)
		embeddingReq := EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "model1:svc1",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{"test input"},
				},
			},
		}
		body, _ := json.Marshal(embeddingReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID: "test-cls",
				SvcName:   "test-svc",
			},
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)

		tester.handler.Embedding(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("model without svc name", func(t *testing.T) {
		tester, c, _ := setupTest(t)
		embeddingReq := EmbeddingRequest{
			EmbeddingNewParams: openai.EmbeddingNewParams{
				Model: "model1",
				Input: openai.EmbeddingNewParamsInputUnion{
					OfArrayOfStrings: []string{"test input"},
				},
			},
		}
		body, _ := json.Marshal(embeddingReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "model1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "",
			},
			Endpoint: "https://api.example.com/embeddings",
		}
		var wg sync.WaitGroup
		wg.Add(1)
		tokenizer := token.NewTokenizerImpl(model.Endpoint, "", "model1", model.ImageID)
		tokenCounter := token.NewEmbeddingTokenCounter(tokenizer)
		tester.mocks.tokenCounterFactory.EXPECT().NewEmbedding(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    model.ID,
				ImageID:  model.ImageID,
			}).
			Return(tokenCounter).Once()
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").
			Return(model, nil)
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, mock.Anything).RunAndReturn(
			func(ctx context.Context, userID string, model *types.Model, counter token.Counter) error {
				wg.Done()
				return nil
			})

		tester.handler.Embedding(c)
		wg.Wait()
	})
}
