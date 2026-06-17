package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	comp "opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	commontypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/trace"
)

type testerOpenAIHandler struct {
	*testutil.GinTester
	mocks struct {
		openAIComp          *mockcomp.MockOpenAIComponent
		moderationComp      *mockcomp.MockModeration
		repoComp            *apicomp.MockRepoComponent
		mockClsComp         *apicomp.MockClusterComponent
		tokenCounterFactory *mocktoken.MockCounterFactory
		whitelistRule       *mockdatabase.MockRepositoryFileCheckRuleStore
		aiGenerationStore   *mockdatabase.MockAIGenerationStore
	}

	handler *OpenAIHandlerImpl
}

type testChatAttemptFailureReporterWithMutex struct {
	mu     sync.Mutex
	doneCh chan struct{}
	events []ChatAttemptFailureEvent
}

func (r *testChatAttemptFailureReporterWithMutex) ReportChatAttemptFailure(_ context.Context, event ChatAttemptFailureEvent) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	r.doneCh <- struct{}{}
	return nil
}

func (r *testChatAttemptFailureReporterWithMutex) Wait() {
	<-r.doneCh
}

func (r *testChatAttemptFailureReporterWithMutex) Events() []ChatAttemptFailureEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]ChatAttemptFailureEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func setupTest(t *testing.T) (*testerOpenAIHandler, *gin.Context, *httptest.ResponseRecorder) {
	mockOpenAI := mockcomp.NewMockOpenAIComponent(t)
	mockRepo := apicomp.NewMockRepoComponent(t)
	mockModeration := mockcomp.NewMockModeration(t)
	mockClsComp := apicomp.NewMockClusterComponent(t)
	mockTokenCounterFactory := mocktoken.NewMockCounterFactory(t)
	cfg := &config.Config{}
	mockWhitelistRule := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
	mockAIGenerationStore := mockdatabase.NewMockAIGenerationStore(t)
	handler := newOpenAIHandler(mockOpenAI, mockRepo, mockModeration, mockClsComp, mockTokenCounterFactory, text2image.NewRegistry(), text2video.NewRegistry(), cfg, nil, mockWhitelistRule, mockAIGenerationStore)

	// Set test user
	tester := &testerOpenAIHandler{
		GinTester: testutil.NewGinTester(),
		handler:   handler,
	}
	w := tester.GinTester.Response()
	c := tester.GinTester.Gctx()
	httpbase.SetCurrentUser(c, "testuser")
	httpbase.SetCurrentNamespaceUUID(c, "testuuid")
	tester.mocks.moderationComp = mockModeration
	tester.mocks.openAIComp = mockOpenAI
	tester.mocks.repoComp = mockRepo
	tester.mocks.mockClsComp = mockClsComp
	tester.mocks.tokenCounterFactory = mockTokenCounterFactory
	tester.mocks.whitelistRule = mockWhitelistRule
	tester.mocks.aiGenerationStore = mockAIGenerationStore

	tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(mock.Anything, mock.Anything, mock.Anything).Return([]database.RepositoryFileCheckRule{}, nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	return tester, c, w
}

func expectBuildVideoMeteringEvent(t *testing.T, tester *testerOpenAIHandler) *commontypes.MeteringEvent {
	event := &commontypes.MeteringEvent{
		Uuid:         uuid.New(),
		UserUUID:     "testuuid",
		Value:        1,
		ValueType:    commontypes.CountNumberType,
		Scene:        int(commontypes.SceneMultiModalServerless),
		OpUID:        string(commontypes.AccessTokenAppAIGateway),
		ResourceID:   "resource-id",
		ResourceName: "resource-id",
		CustomerID:   "customer-id",
		Extra:        `{"completion_data_type":"video"}`,
	}
	tester.mocks.openAIComp.EXPECT().BuildUsageMeteringEvent(mock.Anything, "testuuid", mock.Anything, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
		return usage != nil &&
			usage.DataType == string(commontypes.DataTypeVideo) &&
			usage.CompletionRC == 1
	}), "").Return(event, nil).Once()
	return event
}

func TestOpenAIHandler_ListModels(t *testing.T) {

	t.Run("successful passthrough", func(t *testing.T) {
		tester, c, w := setupTest(t)
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "model1:svc1", Object: "model", OwnedBy: "testuser"}},
		}
		expect := types.ModelList{
			Object:     "list",
			Data:       models,
			HasMore:    false,
			TotalCount: 1,
		}
		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{}).
			Return(expect, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, expect.Object, response.Object)
		assert.Equal(t, expect.Data, response.Data)
		assert.Equal(t, expect.TotalCount, response.TotalCount)
	})

	t.Run("passes query params to component", func(t *testing.T) {
		tester, c, w := setupTest(t)

		tester.WithQuery("model_id", "gpt").
			WithQuery("per", "2").
			WithQuery("page", "3")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{
				ModelID: "gpt",
				Per:     "2",
				Page:    "3",
			}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("passes task query param to component", func(t *testing.T) {
		tester, c, w := setupTest(t)

		tester.WithQuery("task", "text-generation").
			WithQuery("per", "10").
			WithQuery("page", "1")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{
				Task: "text-generation",
				Per:  "10",
				Page: "1",
			}).
			Return(types.ModelList{
				Object:     "list",
				Data:       []types.Model{{BaseModel: types.BaseModel{ID: "model1", Task: "text-generation"}}},
				HasMore:    false,
				TotalCount: 1,
			}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.ModelList
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, 1, response.TotalCount)
		assert.Equal(t, "text-generation", response.Data[0].Task)
	})

	t.Run("component error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{}).
			Return(types.ModelList{}, errors.New("boom")).Once()

		tester.handler.ListModels(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("invalid llm_types parameter", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", "invalid")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		errObj, ok := response["error"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "invalid_request_error", errObj["code"])
		assert.Contains(t, errObj["message"], "Invalid llm_types parameter")
		assert.Contains(t, errObj["message"], types.ProviderTypeExternalLLM)
		assert.Contains(t, errObj["message"], types.ProviderTypeServerless)
		assert.Contains(t, errObj["message"], types.ProviderTypeInference)
	})

	t.Run("valid llm_types parameter external_llm", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", types.ProviderTypeExternalLLM)

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{LLMTypes: []string{types.ProviderTypeExternalLLM}}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("valid llm_types parameter multiple values", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", types.ProviderTypeServerless)
		tester.WithQuery("llm_types", types.ProviderTypeInference)

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{LLMTypes: []string{types.ProviderTypeServerless, types.ProviderTypeInference}}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("llm_types parameter is case-insensitive", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", "SERVERLESS")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuser", types.ListModelsReq{LLMTypes: []string{"SERVERLESS"}}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
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
	tester.mocks.openAIComp.EXPECT().
		ListModels(mock.Anything, "testuser", types.ListModelsReq{}).
		Return(types.ModelList{
			Object:     "list",
			Data:       models,
			HasMore:    false,
			TotalCount: len(models),
		}, nil).
		Once()
	// Create gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add middleware to set current user (similar to how it's done in the actual router)
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")
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
				ID:      "model1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
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

	t.Run("model with slash in name - trims leading slash", func(t *testing.T) {
		tester, c, w := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "xzgan001/gguf_model:fepjlx3v39xc",
				Object:  "model",
				OwnedBy: "testuser",
			},
		}
		// Wildcard route adds leading slash
		c.Params = []gin.Param{{Key: "model", Value: "/xzgan001/gguf_model:fepjlx3v39xc"}}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "xzgan001/gguf_model:fepjlx3v39xc").Return(model, nil)

		tester.handler.GetModel(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.Model
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "xzgan001/gguf_model:fepjlx3v39xc", response.ID)
	})

	t.Run("model without leading slash - no trim needed", func(t *testing.T) {
		tester, c, w := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "simple-model:svc1",
				Object:  "model",
				OwnedBy: "testuser",
			},
		}
		c.Params = []gin.Param{{Key: "model", Value: "simple-model:svc1"}}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "simple-model:svc1").Return(model, nil)

		tester.handler.GetModel(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.Model
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "simple-model:svc1", response.ID)
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
				ID:      "model1",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
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
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: "test-endpoint",
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
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
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(nil, errors.New("some error"))
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
				Provider: model.Provider,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			})
		tester.handler.Chat(c)
		wg.Wait()

		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("usage limit exceeded", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
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
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
			},
			Endpoint: "http://example.com",
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, mock.Anything).
			Return(&comp.UsageLimitExceededError{Message: "usage quota exceeded"}).Once()
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
				Provider: model.Provider,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()

		tester.handler.Chat(c)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "rate_limit_exceeded")
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
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
				Provider: model.Provider,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
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
				ClusterID:     "test-cls",
				SvcName:       "test-svc",
				CSGHubModelID: "model1",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
		}
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    "model1",
				ImageID:  model.ImageID,
				Provider: model.Provider,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return errors.New("record usage error")
			})
		tester.handler.Chat(c)
		wg.Wait()
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("external model uses model id as request model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "external-model-id",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "external-model-id",
				Object:  "model",
				OwnedBy: "testuser",
			},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
			Upstreams: []commontypes.UpstreamConfig{
				{URL: testServer.URL, Enabled: true, ModelName: "external-model-id"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "external-model-id").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    model.ID,
				ImageID:  model.ImageID,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			})

		tester.handler.Chat(c)
		wg.Wait()
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("external formatted id request forwards base model id", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "test-model-1(OpenAI)",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		forwardedModel := ""
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			payload := map[string]any{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			if modelValue, ok := payload["model"].(string); ok {
				forwardedModel = modelValue
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "test-model-1",
				Object:  "model",
				OwnedBy: "OpenAI",
			},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
			Upstreams: []commontypes.UpstreamConfig{
				{URL: testServer.URL, Enabled: true, ModelName: "test-model-1"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model-1(OpenAI)").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    model.ID,
				ImageID:  model.ImageID,
			}).
			Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").
			RunAndReturn(func(ctx context.Context, uuid string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			})

		tester.handler.Chat(c)
		wg.Wait()
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test-model-1", forwardedModel)
	})
	t.Run("report primary upstream http status failure for downstream processing", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		reporter := &testChatAttemptFailureReporterWithMutex{doneCh: make(chan struct{}, 10)}
		tester.handler.SetChatAttemptFailureReporter(reporter)

		chatReq := ChatCompletionRequest{
			Model: "external-model-id",
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Hello"),
			},
		}
		body, _ := json.Marshal(chatReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(`{"error":"not found"}`))
			require.NoError(t, err)
		}))
		defer testServer.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "external-model-id",
				Object:  "model",
				OwnedBy: "testuser",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
			Endpoint: testServer.URL,
			Upstreams: []commontypes.UpstreamConfig{
				{URL: testServer.URL, Enabled: true, ModelName: "external-model-id"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "external-model-id").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckUsageLimit(mock.Anything, "testuuid", model, testServer.URL).Return(nil).Once()
		expectReq := ChatCompletionRequest{}
		_ = json.Unmarshal(body, &expectReq)
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, expectReq.Messages, "testuuid:"+model.ID, false).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		llmTokenCounter := mocktoken.NewMockChatTokenCounter(t)
		tester.mocks.tokenCounterFactory.EXPECT().NewChat(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    model.ID,
				ImageID:  model.ImageID,
				Provider: model.Provider,
			},
		).Return(llmTokenCounter)
		llmTokenCounter.EXPECT().AppendPrompts(expectReq.Messages).Return()
		llmTokenCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{}, nil).Maybe()
		llmTokenCounter.EXPECT().Completion(mock.Anything).Return().Maybe()
		var wg sync.WaitGroup
		wg.Add(2)
		tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, "testuuid", model, llmTokenCounter).
			RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
				wg.Done()
				return nil
			})
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			})

		tester.handler.Chat(c)
		wg.Wait()

		assert.Equal(t, http.StatusNotFound, w.Code)
		reporter.Wait()
		events := reporter.Events()
		require.Len(t, events, 1)
		assert.Equal(t, chatAttemptPhasePrimary, events[0].Phase)
		assert.Equal(t, "external-model-id", events[0].ModelID)
		assert.Equal(t, testServer.URL, events[0].Target)
		assert.Equal(t, http.StatusNotFound, events[0].StatusCode)
		assert.True(t, events[0].Retryable)
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
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
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
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/embeddings", Enabled: true, ModelName: "model1"},
			},
		}
		var wg sync.WaitGroup
		wg.Add(1)
		tokenizer := token.NewTokenizerImpl(model.Endpoint, "", "model1", model.ImageID, model.Provider)
		tokenCounter := token.NewEmbeddingTokenCounter(tokenizer)
		tester.mocks.tokenCounterFactory.EXPECT().NewEmbedding(
			token.CreateParam{
				Endpoint: model.Endpoint,
				Host:     "",
				Model:    model.ID,
				ImageID:  model.ImageID,
				Provider: model.Provider,
			}).
			Return(tokenCounter).Once()
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").
			Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, mock.Anything, mock.Anything, "").RunAndReturn(
			func(ctx context.Context, userID string, model *types.Model, targetModelName string, counter token.Counter, apikey string) error {
				wg.Done()
				return nil
			})

		tester.handler.Embedding(c)
		wg.Wait()
	})
}

func TestOpenAIHandler_EmbeddingTrace(t *testing.T) {
	t.Run("successful passthrough records trace", func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil

		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/api/v1/services/embeddings/text-embedding/text-embedding", r.URL.Path)
			require.Equal(t, "identity", r.Header.Get("Accept-Encoding"))
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`{"object":"list","data":[],"model":"resolved-embedding","usage":{"prompt_tokens":7,"total_tokens":7}}`))
			require.NoError(t, err)
		}))
		defer upstream.Close()

		recorder := &testEmbeddingRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{embeddingRecorder: recorder}
		tester.handler.llmTracer = tracer

		model := &types.Model{
			BaseModel: types.BaseModel{ID: "embedding-model", Object: "model", OwnedBy: "testuser"},
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "openai",
			},
			Upstreams: []commontypes.UpstreamConfig{
				{
					URL:       upstream.URL + "/api/v1/services/embeddings/text-embedding/text-embedding",
					Enabled:   true,
					ModelName: "resolved-embedding",
					Provider:  "openai",
				},
			},
		}
		counter := mocktoken.NewMockEmbeddingTokenCounter(t)
		counter.EXPECT().Embedding(mock.MatchedBy(func(usage openai.CreateEmbeddingResponseUsage) bool {
			return usage.PromptTokens == 7 && usage.TotalTokens == 7
		})).Return().Once()
		counter.EXPECT().Usage(mock.Anything).Return(&token.Usage{PromptTokens: 7, TotalTokens: 7}, nil).Once()
		tester.mocks.tokenCounterFactory.EXPECT().NewEmbedding(token.CreateParam{
			Endpoint: upstream.URL + "/api/v1/services/embeddings/text-embedding/text-embedding",
			Model:    "resolved-embedding",
			Provider: "openai",
		}).Return(counter).Once()
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "embedding-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, "resolved-embedding", counter, "").Return(nil).Once()

		req := EmbeddingRequest{EmbeddingNewParams: openai.EmbeddingNewParams{
			Model: "embedding-model",
			Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{"test input"}},
		}}
		body, err := json.Marshal(req)
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
		c.Set(trace.HeaderRequestID, "req-embedding-ok")
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")

		tester.handler.Embedding(c)

		require.Eventually(t, func() bool {
			_, _, ended, _ := recorder.snapshot()
			return ended
		}, time.Second, 10*time.Millisecond)
		starts := tracer.EmbeddingStarts()
		require.Len(t, starts, 1)
		require.Equal(t, "openai", starts[0].Provider)
		require.Equal(t, "embedding-model", starts[0].RequestModel)
		require.Equal(t, "resolved-embedding", starts[0].ResolvedModel)
		require.Equal(t, "/v1/embeddings", starts[0].Metadata[llmtrace.TraceMetadataKeyAIGatewayAPI])
		require.Equal(t, "req-embedding-ok", starts[0].Metadata["request_id"])
		require.Equal(t, "testuuid", starts[0].Metadata["user_id"])

		result, errorCode, ended, events := recorder.snapshot()
		require.True(t, ended)
		require.NotNil(t, result)
		require.Equal(t, int64(7), result.InputTokens)
		require.Equal(t, 1, result.InputCount)
		require.Equal(t, "resolved-embedding", result.ResponseModel)
		require.Empty(t, errorCode)
		require.Equal(t, []string{"result", "end"}, events)
	})

	t.Run("balance failure records trace error", func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil

		recorder := &testEmbeddingRecorderWithMutex{}
		tester.handler.llmTracer = &testLLMTracerWithMutex{embeddingRecorder: recorder}

		model := &types.Model{
			BaseModel: types.BaseModel{ID: "embedding-model", Object: "model", OwnedBy: "testuser"},
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "openai",
			},
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://upstream.example.test", Enabled: true, ModelName: "resolved-embedding", Provider: "openai"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "embedding-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(errorx.ErrInsufficientBalance).Once()

		req := EmbeddingRequest{EmbeddingNewParams: openai.EmbeddingNewParams{
			Model: "embedding-model",
			Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{"test input"}},
		}}
		body, err := json.Marshal(req)
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")

		tester.handler.Embedding(c)

		_, errorCode, ended, _ := recorder.snapshot()
		require.True(t, ended)
		require.Equal(t, types.TraceErrInsufficientBalance, errorCode)
	})

	t.Run("upstream error status records trace error", func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil

		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"object":"list","data":[],"model":"resolved-embedding","usage":{"prompt_tokens":3,"total_tokens":3}}`))
			require.NoError(t, err)
		}))
		defer upstream.Close()

		recorder := &testEmbeddingRecorderWithMutex{}
		tester.handler.llmTracer = &testLLMTracerWithMutex{embeddingRecorder: recorder}

		model := &types.Model{
			BaseModel: types.BaseModel{ID: "embedding-model", Object: "model", OwnedBy: "testuser"},
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "openai",
			},
			Upstreams: []commontypes.UpstreamConfig{
				{URL: upstream.URL, Enabled: true, ModelName: "resolved-embedding", Provider: "openai"},
			},
		}
		counter := mocktoken.NewMockEmbeddingTokenCounter(t)
		counter.EXPECT().Embedding(mock.MatchedBy(func(usage openai.CreateEmbeddingResponseUsage) bool {
			return usage.PromptTokens == 3 && usage.TotalTokens == 3
		})).Return().Once()
		counter.EXPECT().Usage(mock.Anything).Return(&token.Usage{PromptTokens: 3, TotalTokens: 3}, nil).Once()
		tester.mocks.tokenCounterFactory.EXPECT().NewEmbedding(token.CreateParam{
			Endpoint: upstream.URL,
			Model:    "resolved-embedding",
			Provider: "openai",
		}).Return(counter).Once()
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "embedding-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, "resolved-embedding", counter, "").Return(nil).Once()

		req := EmbeddingRequest{EmbeddingNewParams: openai.EmbeddingNewParams{
			Model: "embedding-model",
			Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{"test input"}},
		}}
		body, err := json.Marshal(req)
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")

		tester.handler.Embedding(c)

		require.Eventually(t, func() bool {
			_, _, ended, _ := recorder.snapshot()
			return ended
		}, time.Second, 10*time.Millisecond)
		result, errorCode, ended, events := recorder.snapshot()
		require.True(t, ended)
		require.NotNil(t, result)
		require.Equal(t, int64(3), result.InputTokens)
		require.Equal(t, types.TraceErrUpstreamError, errorCode)
		require.Equal(t, []string{"error", "result", "end"}, events)
	})
}

func TestOpenAIHandler_Transcription(t *testing.T) {
	t.Run("successful multipart passthrough with rewritten model", func(t *testing.T) {
		tester, c, w := setupTest(t)

		var downstreamModel string
		var downstreamPrompt string
		var downstreamFile string
		var downstreamAuth string
		var downstreamAcceptEncoding string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/audio/transcriptions", r.URL.Path)
			downstreamAuth = r.Header.Get("Authorization")
			downstreamAcceptEncoding = r.Header.Get("Accept-Encoding")
			require.NoError(t, r.ParseMultipartForm(32<<20))
			downstreamModel = r.FormValue("model")
			downstreamPrompt = r.FormValue("prompt")
			file, _, err := r.FormFile("file")
			require.NoError(t, err)
			defer file.Close()
			data, err := io.ReadAll(file)
			require.NoError(t, err)
			downstreamFile = string(data)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"hello world","usage":{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423,"seconds":9.2}}`))
		}))
		defer server.Close()

		c.Request = newMultipartTranscriptionRequest(t, "model1", "audio-bytes", map[string]string{
			"prompt": "meeting",
		})
		c.Set(trace.HeaderRequestID, "req-audio-ok")
		recorder := &testGenerationRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		tester.handler.llmTracer = tracer
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "audio-transcription",
			},
			Endpoint: server.URL + "/v1/audio/transcriptions",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/transcriptions", Enabled: true, ModelName: "backend-model"},
			},
			ExternalModelInfo: types.ExternalModelInfo{
				AuthHead: `{"Authorization":"Bearer provider-token"}`,
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.TotalTokens == 423 &&
				usage.PromptTokens == 371 &&
				usage.CompletionTokens == 52 &&
				usage.Duration == 9.2
		}), "").RunAndReturn(
			func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				require.Equal(t, int64(423), usage.TotalTokens)
				require.Equal(t, int64(371), usage.PromptTokens)
				require.Equal(t, int64(52), usage.CompletionTokens)
				require.Equal(t, 9.2, usage.Duration)
				wg.Done()
				return nil
			}).Once()

		tester.handler.Transcription(c)
		wg.Wait()

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.JSONEq(t, `{"text":"hello world","usage":{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423,"seconds":9.2}}`, w.Body.String())
		require.Equal(t, "backend-model", downstreamModel)
		require.Equal(t, "meeting", downstreamPrompt)
		require.Equal(t, "audio-bytes", downstreamFile)
		require.Equal(t, "Bearer provider-token", downstreamAuth)
		require.Equal(t, "identity", downstreamAcceptEncoding)
		starts := tracer.Starts()
		require.Len(t, starts, 1)
		require.Equal(t, "req-audio-ok", starts[0].RequestID)
		require.Equal(t, "generate_content", starts[0].OperationName)
		require.Equal(t, "text", starts[0].Metadata[llmtrace.TraceMetadataKeyGenAIOutputType])
		usage, response, errorCode, ended, events := generationTraceSnapshot(recorder)
		require.True(t, ended)
		require.NotNil(t, response)
		require.Equal(t, "backend-model", response.Model)
		require.Equal(t, 9.2, response.Metadata[llmtrace.TraceMetadataKeyAudioDurationSeconds])
		require.NotNil(t, usage)
		require.Equal(t, int64(423), usage.TotalTokens)
		require.Empty(t, errorCode)
		require.Equal(t, []string{"response", "usage", "end"}, events)
	})

	t.Run("missing model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartTranscriptionRequest(t, "", "audio-bytes", nil)

		tester.handler.Transcription(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model cannot be empty")
	})

	t.Run("missing file", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "model1"))
		require.NoError(t, writer.Close())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())

		tester.handler.Transcription(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "File cannot be empty")
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartTranscriptionRequest(t, "missing-model", "audio-bytes", nil)

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

		tester.handler.Transcription(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "model_not_found")
	})

	t.Run("upstream error status ends trace without billing", func(t *testing.T) {
		tester, c, w := setupTest(t)
		doneCh := make(chan struct{})
		recorder := &testGenerationRecorderWithMutex{doneCh: doneCh}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		tester.handler.llmTracer = tracer

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream unavailable"}}`))
		}))
		defer server.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "audio-transcription",
			},
			Endpoint: server.URL + "/v1/audio/transcriptions",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/transcriptions", Enabled: true, ModelName: "backend-model"},
			},
		}
		c.Request = newMultipartTranscriptionRequest(t, "model1", "audio-bytes", nil)
		c.Set(trace.HeaderRequestID, "req-audio-error")

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()

		tester.handler.Transcription(c)
		waitForGenerationTraceEnd(t, doneCh)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		usage, response, errorCode, ended, events := generationTraceSnapshot(recorder)
		require.True(t, ended)
		require.Nil(t, usage)
		require.NotNil(t, response)
		require.Equal(t, "backend-model", response.Model)
		require.Equal(t, types.TraceErrUpstreamError, errorCode)
		require.Equal(t, []string{"response", "error", "end"}, events)
	})
}

func newMultipartTranscriptionRequest(t *testing.T, model, fileContent string, fields map[string]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if model != "" {
		require.NoError(t, writer.WriteField("model", model))
	}
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if fileContent != "" {
		part, err := writer.CreateFormFile("file", "sample.wav")
		require.NoError(t, err)
		_, err = part.Write([]byte(fileContent))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func waitForGenerationTraceEnd(t *testing.T, doneCh <-chan struct{}) {
	t.Helper()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for generation trace to end")
	}
}

func TestOpenAIHandler_GenerateImage(t *testing.T) {
	t.Run("invalid request body", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request.Method = http.MethodPost
		c.Request.Body = http.NoBody

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		tester, c, w := setupTest(t)
		// Test missing prompt
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model: "test-model:svc",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test missing model
		tester2, c2, w2 := setupTest(t)
		imageReq2 := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Prompt: "test prompt",
			},
		}
		body2, _ := json.Marshal(imageReq2)
		c2.Request.Method = http.MethodPost
		c2.Request.Body = io.NopCloser(bytes.NewReader(body2))

		tester2.handler.GenerateImage(c2)

		assert.Equal(t, http.StatusBadRequest, w2.Code)
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "nonexistent:svc",
				Prompt: "test prompt",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "nonexistent:svc").Return(nil, nil)

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("get model error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "test-model:svc",
				Prompt: "test prompt",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model:svc").Return(nil, errors.New("internal error"))

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("model not running", func(t *testing.T) {
		tester, c, w := setupTest(t)
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "test-model:svc",
				Prompt: "test prompt",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model:svc").Return(nil, nil)

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("sensitive content detected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "test-model",
				Prompt: "sensitive prompt",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "test-model",
				Object:  "model",
				OwnedBy: "testuser",
				Task:    "text-to-image",
			},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "",
			},
			Endpoint: "https://api.example.com/images/generations",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/images/generations", Enabled: true, ModelName: "test-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "sensitive prompt", "testuuid").Return(&rpc.CheckResult{IsSensitive: true}, nil)

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("sensitive content check failed", func(t *testing.T) {
		tester, c, w := setupTest(t)
		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "test-model",
				Prompt: "test prompt",
			},
		}
		body, _ := json.Marshal(imageReq)
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "test-model",
				Object:  "model",
				OwnedBy: "testuser",
				Task:    "text-to-image",
			},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "",
			},
			Endpoint: "https://api.example.com/images/generations",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/images/generations", Enabled: true, ModelName: "test-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model").Return(model, nil)
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil)
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "test prompt", "testuuid").Return(nil, errors.New("moderation service error"))

		tester.handler.GenerateImage(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("upstream error status ends trace without billing", func(t *testing.T) {
		tester, c, w := setupTest(t)
		doneCh := make(chan struct{})
		recorder := &testGenerationRecorderWithMutex{doneCh: doneCh}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		tester.handler.llmTracer = tracer

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"created":0,"data":[]}`))
		}))
		defer server.Close()

		imageReq := ImageGenerationRequest{
			ImageGenerateParams: openai.ImageGenerateParams{
				Model:  "test-model",
				Prompt: "test prompt",
			},
		}
		body, err := json.Marshal(imageReq)
		require.NoError(t, err)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(trace.HeaderRequestID, "req-image-error")

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:      "test-model",
				Object:  "model",
				OwnedBy: "testuser",
				Task:    "text-to-image",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "openai",
			},
			Endpoint: server.URL + "/v1/images/generations",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/images/generations", Enabled: true, ModelName: "test-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "test-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "test prompt", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		tester.handler.GenerateImage(c)
		waitForGenerationTraceEnd(t, doneCh)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		usage, response, errorCode, ended, events := generationTraceSnapshot(recorder)
		require.True(t, ended)
		require.Nil(t, usage)
		require.NotNil(t, response)
		require.Equal(t, "test-model", response.Model)
		require.Equal(t, types.TraceErrUpstreamError, errorCode)
		require.Equal(t, []string{"response", "error", "end"}, events)
	})

	t.Run("proxyToApi is / when endpoint has no path (space deployment)", func(t *testing.T) {
		// Spaces (HF Inference Toolkit) serve at root. When model.Endpoint is
		// "http://svc.spaces.a800.external" (no path), we must use proxyToApi="/"
		// so the proxy rewrites the path to / instead of keeping /v1/images/generations.
		tests := []struct {
			endpoint string
			wantPath string
			desc     string
		}{
			{"http://svc.spaces.a800.external", "/", "no path -> root"},
			{"http://svc.spaces.a800.external/", "/", "trailing slash -> root"},
			{"https://api.example.com/v1/images", "/v1/images", "explicit path preserved"},
			{"", "", "empty endpoint -> no rewrite"},
		}
		for _, tt := range tests {
			t.Run(tt.desc, func(t *testing.T) {
				proxyToApi := ""
				if tt.endpoint != "" {
					uri, err := url.ParseRequestURI(tt.endpoint)
					require.NoError(t, err)
					proxyToApi = uri.Path
					if proxyToApi == "" {
						proxyToApi = "/"
					}
				}
				assert.Equal(t, tt.wantPath, proxyToApi)
			})
		}
	})
}

func TestOpenAIHandler_CreateVideo(t *testing.T) {
	t.Run("create video with json request stores provider video id", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var downstreamModel string
		var createdGeneration database.AIGeneration
		recorder := &testGenerationRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		tester.handler.llmTracer = tracer

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/videos", r.URL.Path)
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			downstreamModel = req["model"].(string)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"vid_123","object":"video","status":"queued"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:   "video-model",
				Task: "text-to-video",
				Metadata: map[string]any{
					types.MetaKeyLLMType:           types.ProviderTypeExternalLLM,
					types.MetaKeyPricingConfigured: true,
				},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
			Endpoint:          downstream.URL + "/v1/videos",
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 123, URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model", Provider: "openai"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a flying car", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		meteringEvent := &commontypes.MeteringEvent{
			Uuid:         uuid.New(),
			UserUUID:     "testuuid",
			Value:        1,
			ValueType:    commontypes.CountNumberType,
			Scene:        int(commontypes.SceneMultiModalServerless),
			OpUID:        string(commontypes.AccessTokenAppAIGateway),
			ResourceID:   "resource-id",
			ResourceName: "resource-id",
			CustomerID:   "customer-id",
			Extra:        `{"completion_data_type":"video"}`,
		}
		tester.mocks.openAIComp.EXPECT().BuildUsageMeteringEvent(mock.Anything, "testuuid", model, "video-model", mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.DataType == string(commontypes.DataTypeVideo) &&
				usage.Duration == 5 &&
				usage.CompletionRC == 1 &&
				usage.CompletionDesc == "make a flying car"
		}), "api-key").Return(meteringEvent, nil).Once()
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.MatchedBy(func(generation database.AIGeneration) bool {
			createdGeneration = generation
			return generation.ResourceType == database.AIGenerationResourceTypeVideo &&
				strings.HasPrefix(generation.ResourceID, "video_") &&
				generation.ProviderResourceID == "vid_123" &&
				generation.OwnerUUID == "testuuid" &&
				generation.ModelID == "video-model" &&
				generation.Status == "queued"
		})).Return(&database.AIGeneration{}, nil).Once()

		body := `{"model":"video-model","prompt":"make a flying car","seconds":5}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		httpbase.SetAccessToken(c, "api-key")
		c.Set(trace.HeaderRequestID, "req-video-ok")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		var resp types.VideoObject
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotEqual(t, "vid_123", resp.ID)
		require.True(t, strings.HasPrefix(resp.ID, "video_"))
		require.Equal(t, "queued", resp.Status)
		generation := createdGeneration
		require.Equal(t, resp.ID, generation.ResourceID)
		require.Equal(t, "video-model", generation.ModelID)
		require.Equal(t, "testuuid", generation.OwnerUUID)
		require.Equal(t, "vid_123", generation.ProviderResourceID)
		require.NotEqual(t, uuid.Nil, generation.EventUUID)
		require.NotContains(t, generation.ProviderMetadata, "prompt")
		require.NotContains(t, generation.ProviderMetadata, "seconds")
		require.NotContains(t, generation.ProviderMetadata, "target")
		require.Equal(t, int64(123), generation.UpstreamID)
		require.Equal(t, meteringEvent.Uuid, generation.EventUUID)
		require.Equal(t, meteringEvent, generation.MeteringMetadata)
		require.Equal(t, model.ID, downstreamModel)
		starts := tracer.Starts()
		require.Len(t, starts, 1)
		require.Equal(t, "req-video-ok", starts[0].RequestID)
		require.Equal(t, "generate_content", starts[0].OperationName)
		require.Equal(t, "video", starts[0].Metadata[llmtrace.TraceMetadataKeyGenAIOutputType])
		require.Equal(t, int64(5), starts[0].Metadata[llmtrace.TraceMetadataKeyVideoSeconds])
		usage, response, errorCode, ended, events := generationTraceSnapshot(recorder)
		require.True(t, ended)
		require.Nil(t, usage)
		require.NotNil(t, response)
		require.Equal(t, "openai", response.Provider)
		require.Equal(t, "video-model", response.Model)
		require.Equal(t, resp.ID, response.Metadata[llmtrace.TraceMetadataKeyVideoID])
		require.Equal(t, "queued", response.Metadata[llmtrace.TraceMetadataKeyVideoStatus])
		require.Equal(t, int64(5), response.Metadata[llmtrace.TraceMetadataKeyVideoSeconds])
		require.Empty(t, errorCode)
		require.Equal(t, []string{"response", "end"}, events)
	})

	t.Run("create video with json image_url input_reference for image-to-video task", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotInputReference map[string]any

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/videos", r.URL.Path)
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			gotInputReference = req["input_reference"].(map[string]any)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"vid_img_url","object":"video","status":"queued"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "image-video-model", Task: "image-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
			Endpoint:          downstream.URL + "/v1/videos",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "image-video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "image-video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "animate this image", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.Anything).
			Return(&database.AIGeneration{}, nil).Once()

		body := `{"model":"image-video-model","prompt":"animate this image","input_reference":{"image_url":"https://example.com/frame.png"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "https://example.com/frame.png", gotInputReference["image_url"])
	})

	t.Run("create video with json file_id input_reference", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotInputReference map[string]any

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			gotInputReference = req["input_reference"].(map[string]any)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"vid_file_id","object":"video","status":"queued"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
			Endpoint:          downstream.URL + "/v1/videos",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "animate this asset", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.Anything).
			Return(&database.AIGeneration{}, nil).Once()

		body := `{"model":"video-model","prompt":"animate this asset","input_reference":{"file_id":"file_123"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "file_123", gotInputReference["file_id"])
	})

	t.Run("reject invalid json input_reference", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body := `{"model":"video-model","prompt":"animate this asset","input_reference":{}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "input_reference must include file_id or image_url")
	})

	t.Run("create video with multipart request", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotModel string
		var gotPrompt string
		var createdGeneration database.AIGeneration

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, r.ParseMultipartForm(1024*1024))
			gotModel = r.FormValue("model")
			gotPrompt = r.FormValue("prompt")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"vid_456","object":"video","status":"queued"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
			Endpoint:          downstream.URL + "/v1/videos",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.MatchedBy(func(generation database.AIGeneration) bool {
			createdGeneration = generation
			return strings.HasPrefix(generation.ResourceID, "video_") &&
				generation.ModelID == "video-model" &&
				generation.ProviderResourceID == "vid_456"
		})).Return(&database.AIGeneration{}, nil).Once()

		c.Request = newMultipartVideoRequest(t, "video-model", "make a boat", "image/png")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		var resp types.VideoObject
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, resp.ID, createdGeneration.ResourceID)
		require.Equal(t, "video-model", createdGeneration.ModelID)
		require.Equal(t, "vid_456", createdGeneration.ProviderResourceID)
		require.Equal(t, "video-model", gotModel)
		require.Equal(t, "make a boat", gotPrompt)
	})

	t.Run("create video with minimax adapter normalizes request and response", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotReq map[string]any
		var createdGeneration database.AIGeneration

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/video_generation", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReq))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_123"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "video-model",
				Task:     "image-to-video",
				Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
			Endpoint:          downstream.URL + "/v1/video_generation",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/video_generation", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.MatchedBy(func(generation database.AIGeneration) bool {
			createdGeneration = generation
			return strings.HasPrefix(generation.ResourceID, "video_") &&
				generation.ProviderResourceID == "task_123"
		})).Return(&database.AIGeneration{}, nil).Once()

		body := `{"model":"video-model","prompt":"make a boat","size":"1280x720","input_reference":{"image_url":"https://example.com/frame.png"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		var resp types.VideoObject
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.True(t, strings.HasPrefix(resp.ID, "video_"))
		require.Equal(t, resp.ID, createdGeneration.ResourceID)
		require.Equal(t, "task_123", createdGeneration.ProviderResourceID)
		require.Equal(t, "video-model", gotReq["model"])
		require.Equal(t, "768P", gotReq["resolution"])
		require.Equal(t, "https://example.com/frame.png", gotReq["first_frame_image"])
	})

	t.Run("create video with minimax adapter rejects unsupported openai size", func(t *testing.T) {
		tester, c, w := setupTest(t)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "video-model",
				Task:     "text-to-video",
				Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
			Endpoint:          "https://api.minimax.example.com/v1/video_generation",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.minimax.example.com/v1/video_generation", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		body := `{"model":"video-model","prompt":"make a boat","size":"1024x1792"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "MiniMax video backend does not support size")
	})

	t.Run("create video with minimax adapter surfaces downstream provider error message", func(t *testing.T) {
		tester, c, w := setupTest(t)

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/video_generation", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"base_resp": {
					"status_code": 1004,
					"status_msg": "unsupported resolution for model"
				}
			}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "video-model",
				Task:     "text-to-video",
				Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
			Endpoint:          downstream.URL + "/v1/video_generation",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/video_generation", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		body := `{"model":"video-model","prompt":"make a boat","size":"1280x720"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "unsupported resolution for model")
		require.NotContains(t, w.Body.String(), "missing task_id")
	})

	t.Run("create video includes downstream top-level error message on parse failure", func(t *testing.T) {
		tester, c, w := setupTest(t)

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/video_generation", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":"Failed to get product"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "video-model",
				Task:     "text-to-video",
				Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
			Endpoint:          downstream.URL + "/v1/video_generation",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL + "/v1/video_generation", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		body := `{"model":"video-model","prompt":"make a boat"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "Failed to get product")
		require.Contains(t, w.Body.String(), "missing task_id")
	})

	t.Run("create video with seedance adapter rejects unsupported openai size", func(t *testing.T) {
		tester, c, w := setupTest(t)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "video-model",
				Task:     "text-to-video",
				Metadata: map[string]any{"video_api": map[string]any{"type": "seedance"}},
			},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "seedance"},
			Endpoint:          "https://ark.ap-southeast.bytepluses.com/api/v3",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://ark.ap-southeast.bytepluses.com/api/v3", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		body := `{"model":"video-model","prompt":"make a boat","size":"1024x1792"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Seedance video backend does not support size")
	})

	t.Run("create video with lightx2v multipart request", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotPrompt string
		var gotWidth string
		var gotHeight string
		var gotDuration string
		var gotImageFile bool
		var createdGeneration database.AIGeneration

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/tasks/video/form", r.URL.Path)
			require.NoError(t, r.ParseMultipartForm(1024*1024))
			gotPrompt = r.FormValue("prompt")
			gotWidth = r.FormValue("width")
			gotHeight = r.FormValue("height")
			gotDuration = r.FormValue("video_duration")
			files := r.MultipartForm.File["image_file"]
			gotImageFile = len(files) == 1
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_789","status":"submitted"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "video-model", Task: "image-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
			InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"},
			Endpoint:          downstream.URL,
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL, Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.MatchedBy(func(generation database.AIGeneration) bool {
			createdGeneration = generation
			return strings.HasPrefix(generation.ResourceID, "video_") &&
				generation.ModelID == "video-model" &&
				generation.ProviderResourceID == "task_789"
		})).Return(&database.AIGeneration{}, nil).Once()

		c.Request = newMultipartVideoRequestWithSize(t, "video-model", "make a boat", "1280x720", "image/png")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		var resp types.VideoObject
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, resp.ID, createdGeneration.ResourceID)
		require.Equal(t, "task_789", createdGeneration.ProviderResourceID)
		require.Equal(t, "make a boat", gotPrompt)
		require.Equal(t, "1280", gotWidth)
		require.Equal(t, "720", gotHeight)
		require.Equal(t, "", gotDuration)
		require.True(t, gotImageFile)
	})

	t.Run("create video with lightx2v rejects json file_id input_reference", func(t *testing.T) {
		tester, c, w := setupTest(t)

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "video-model", Task: "image-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
			InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"},
			Endpoint:          "https://lightx2v.internal",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://lightx2v.internal", Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()

		body := `{"model":"video-model","prompt":"animate this asset","input_reference":{"file_id":"file_123"}}`
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "selected model does not support input_reference.file_id")
	})

	t.Run("create video with lightx2v multipart text-only request falls back to json create", func(t *testing.T) {
		tester, c, w := setupTest(t)
		var gotBody map[string]any
		var createdGeneration database.AIGeneration

		downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/tasks/video", r.URL.Path)
			require.Equal(t, "application/json", strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_t2v","status":"submitted"}`))
		}))
		defer downstream.Close()

		model := &types.Model{
			BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
			ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
			InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"},
			Endpoint:          downstream.URL,
			Upstreams: []commontypes.UpstreamConfig{
				{URL: downstream.URL, Enabled: true, ModelName: "video-model"},
			},
		}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "make a boat", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		expectBuildVideoMeteringEvent(t, tester)
		tester.mocks.aiGenerationStore.EXPECT().Create(mock.Anything, mock.MatchedBy(func(generation database.AIGeneration) bool {
			createdGeneration = generation
			return strings.HasPrefix(generation.ResourceID, "video_") &&
				generation.ModelID == "video-model" &&
				generation.ProviderResourceID == "task_t2v"
		})).Return(&database.AIGeneration{}, nil).Once()

		c.Request = newMultipartTextOnlyVideoRequestWithSizeAndSeconds(t, "video-model", "make a boat", "1280x720", 5)

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusOK, w.Code)
		var resp types.VideoObject
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, resp.ID, createdGeneration.ResourceID)
		require.Equal(t, "task_t2v", createdGeneration.ProviderResourceID)
		require.Equal(t, "make a boat", gotBody["prompt"])
		require.Equal(t, float64(1280), gotBody["width"])
		require.Equal(t, float64(720), gotBody["height"])
		require.Equal(t, float64(5), gotBody["video_duration"])
	})

	t.Run("reject multipart input_reference with unsupported content type", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newMultipartVideoRequest(t, "video-model", "make a boat", "text/plain")

		tester.handler.CreateVideo(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "unsupported input_reference content type")
	})
}

func TestOpenAIHandler_GetVideo(t *testing.T) {
	tester, c, w := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "queued",
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/videos/vid_123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"vid_123","object":"video","status":"completed","created_at":123}`))
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
		Endpoint:          downstream.URL + "/v1/videos",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().UpdateWithStatus(mock.Anything, mock.MatchedBy(func(input database.AIGeneration) bool {
		generation = input
		return input.ResourceID == "video_gateway" && input.Status == "completed"
	}), "queued").Return(true, nil).Once()

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway", nil)
	c.Params = gin.Params{{Key: "video_id", Value: "video_gateway"}}

	tester.handler.GetVideo(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"id":"video_gateway","object":"video","status":"completed","created_at":123}`, w.Body.String())
	require.Equal(t, "completed", generation.Status)
}

func TestOpenAIHandler_GetVideo_ReturnsTerminalRowWithoutUpstreamFetch(t *testing.T) {
	tester, c, w := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "completed",
	}

	var upstreamCalls int
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
		Endpoint:          downstream.URL + "/v1/videos",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway", nil)
	c.Params = gin.Params{{Key: "video_id", Value: "video_gateway"}}

	tester.handler.GetVideo(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"id":"video_gateway","object":"video","status":"completed","model":"video-model"}`, w.Body.String())
	require.Zero(t, upstreamCalls)
}

func TestOpenAIHandler_GetVideoContent(t *testing.T) {
	tester, _, _ := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "completed",
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/videos/vid_123/content", r.URL.Path)
		require.Equal(t, "thumbnail", r.URL.Query().Get("variant"))
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", strconv.Itoa(len("video-bytes")))
		_, _ = w.Write([]byte("video-bytes"))
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
		Endpoint:          downstream.URL + "/v1/videos",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")
		c.Next()
	})
	router.GET("/v1/videos/:video_id/content", tester.handler.GetVideoContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway/content?variant=thumbnail", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "video-bytes", w.Body.String())
}

func TestOpenAIHandler_GetVideoContent_NotReadyDoesNotCallUpstream(t *testing.T) {
	tester, _, _ := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "queued",
	}

	upstreamCalls := 0
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		t.Fatalf("upstream should not be called for non-completed video")
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
		Endpoint:          downstream.URL + "/v1/videos",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL + "/v1/videos", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")
		c.Next()
	})
	router.GET("/v1/videos/:video_id/content", tester.handler.GetVideoContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway/content", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "video_not_ready")
	require.Zero(t, upstreamCalls)
}

func TestOpenAIHandler_GetVideoContent_MiniMaxResolvesDownloadURL(t *testing.T) {
	tester, _, _ := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "task_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "completed",
	}

	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("minimax-video-bytes"))
	}))
	defer download.Close()

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/query/video_generation":
			require.Equal(t, "task_123", r.URL.Query().Get("task_id"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"task_123","status":"Success","file_id":"file_123"}`))
		case "/v1/files/retrieve":
			require.Equal(t, "file_123", r.URL.Query().Get("file_id"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"file":{"download_url":%q}}`, download.URL+"/video.mp4")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{
			ID:       "video-model",
			Task:     "text-to-video",
			Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
		},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
		Endpoint:          downstream.URL + "/v1/video_generation",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL + "/v1/video_generation", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().UpdateProviderMetadata(mock.Anything, int64(1), mock.MatchedBy(func(providerMetadata map[string]any) bool {
		generation.ProviderMetadata = providerMetadata
		return providerMetadata["file_id"] == "file_123"
	})).Return(nil).Once()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")
		c.Next()
	})
	router.GET("/v1/videos/:video_id/content", tester.handler.GetVideoContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway/content", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "minimax-video-bytes", w.Body.String())
	require.Equal(t, "file_123", generation.ProviderMetadata["file_id"])
}

func TestOpenAIHandler_GetVideoContent_LightX2VStreamsDirectly(t *testing.T) {
	tester, _, _ := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "task_123",
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "completed",
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/files/download/outputs/videos/task_123.mp4", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("lightx2v-video-bytes"))
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"},
		Endpoint:          downstream.URL,
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL, Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "video-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		httpbase.SetCurrentUser(c, "testuser")
		httpbase.SetCurrentNamespaceUUID(c, "testuuid")
		c.Next()
	})
	router.GET("/v1/videos/:video_id/content", tester.handler.GetVideoContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway/content", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "lightx2v-video-bytes", w.Body.String())
}

func newMultipartVideoRequest(t *testing.T, model, prompt, fileContentType string) *http.Request {
	return newMultipartVideoRequestWithSize(t, model, prompt, "", fileContentType)
}

func newMultipartTextOnlyVideoRequestWithSizeAndSeconds(t *testing.T, model, prompt, size string, seconds int) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", model))
	require.NoError(t, writer.WriteField("prompt", prompt))
	if size != "" {
		require.NoError(t, writer.WriteField("size", size))
	}
	if seconds > 0 {
		require.NoError(t, writer.WriteField("seconds", strconv.Itoa(seconds)))
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func newMultipartVideoRequestWithSize(t *testing.T, model, prompt, size, fileContentType string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", model))
	require.NoError(t, writer.WriteField("prompt", prompt))
	if size != "" {
		require.NoError(t, writer.WriteField("size", size))
	}
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="input_reference"; filename="frame.png"`},
		"Content-Type":        []string{fileContentType},
	})
	require.NoError(t, err)
	_, err = part.Write(testVideoInputReferenceBytes(fileContentType))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func testVideoInputReferenceBytes(contentType string) []byte {
	switch contentType {
	case "image/png":
		return []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	case "image/jpeg":
		return []byte{0xff, 0xd8, 0xff, 0xdb}
	case "image/webp":
		return []byte{'R', 'I', 'F', 'F', 0x24, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'}
	default:
		return []byte("image-bytes")
	}
}
