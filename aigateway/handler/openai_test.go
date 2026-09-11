package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	audioadapter "opencsg.com/csghub-server/aigateway/component/adapter/audio"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
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

func setupTest(t *testing.T) (*testerOpenAIHandler, *gin.Context, *httptest.ResponseRecorder) {
	mockOpenAI := mockcomp.NewMockOpenAIComponent(t)
	mockRepo := apicomp.NewMockRepoComponent(t)
	mockModeration := mockcomp.NewMockModeration(t)
	mockClsComp := apicomp.NewMockClusterComponent(t)
	mockTokenCounterFactory := mocktoken.NewMockCounterFactory(t)
	cfg := &config.Config{}
	mockWhitelistRule := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
	mockAIGenerationStore := mockdatabase.NewMockAIGenerationStore(t)
	handler := newOpenAIHandler(mockOpenAI, mockRepo, mockModeration, mockClsComp, mockTokenCounterFactory, text2image.NewRegistry(), text2video.NewRegistry(), audioadapter.NewRegistry(), cfg, nil, mockWhitelistRule, mockAIGenerationStore)

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

	return tester, c, w
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
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{}).
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
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{
				ModelID: "gpt",
				Per:     2,
				Page:    3,
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
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{
				Task: "text-generation",
				Per:  10,
				Page: 1,
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

	t.Run("passes has_associated_model true query param to component", func(t *testing.T) {
		tester, c, w := setupTest(t)
		hasAssociatedModel := true

		tester.WithQuery("has_associated_model", "true")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{
				HasAssociatedModel: &hasAssociatedModel,
			}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("passes has_associated_model false query param to component", func(t *testing.T) {
		tester, c, w := setupTest(t)
		hasAssociatedModel := false

		tester.WithQuery("has_associated_model", "false")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{
				HasAssociatedModel: &hasAssociatedModel,
			}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("accepts has_associated_model bool aliases", func(t *testing.T) {
		tester, c, w := setupTest(t)
		hasAssociatedModel := true

		tester.WithQuery("has_associated_model", "TRUE")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{
				HasAssociatedModel: &hasAssociatedModel,
			}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("component error", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{}).
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
		assert.Contains(t, errObj["message"], commontypes.ProviderTypeExternalLLM)
		assert.Contains(t, errObj["message"], commontypes.ProviderTypeServerless)
		assert.Contains(t, errObj["message"], commontypes.ProviderTypeInference)
	})

	for _, value := range []string{"yes", "not-a-bool"} {
		t.Run("invalid has_associated_model parameter "+value, func(t *testing.T) {
			tester, c, w := setupTest(t)
			tester.WithQuery("has_associated_model", value)

			tester.handler.ListModels(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			errObj, ok := response["error"].(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, "invalid_request_error", errObj["code"])
			assert.Contains(t, errObj["message"], "Invalid has_associated_model parameter")
		})
	}

	t.Run("only per returns bad request", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("per", "20")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		errObj, ok := response["error"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "invalid_request_error", errObj["code"])
		assert.Contains(t, errObj["message"], "per and page must be provided together")
	})

	t.Run("only page returns bad request", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("page", "1")

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		errObj, ok := response["error"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "invalid_request_error", errObj["code"])
		assert.Contains(t, errObj["message"], "per and page must be provided together")
	})

	for _, tc := range []struct {
		name string
		per  string
		page string
	}{
		{name: "non-integer per", per: "abc", page: "1"},
		{name: "non-integer page", per: "20", page: "abc"},
		{name: "zero per", per: "0", page: "1"},
		{name: "zero page", per: "20", page: "0"},
		{name: "negative per", per: "-1", page: "1"},
		{name: "negative page", per: "20", page: "-1"},
	} {
		t.Run("invalid pagination parameter "+tc.name, func(t *testing.T) {
			tester, c, w := setupTest(t)
			tester.WithQuery("per", tc.per)
			tester.WithQuery("page", tc.page)

			tester.handler.ListModels(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			errObj, ok := response["error"].(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, "invalid_request_error", errObj["code"])
			assert.Contains(t, errObj["message"], "Invalid pagination parameter")
		})
	}

	t.Run("valid llm_types parameter external_llm", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", commontypes.ProviderTypeExternalLLM)

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeExternalLLM}}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("valid llm_types parameter multiple values", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", commontypes.ProviderTypeServerless)
		tester.WithQuery("llm_types", commontypes.ProviderTypeInference)

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeServerless, commontypes.ProviderTypeInference}}).
			Return(types.ModelList{Object: "list", Data: []types.Model{}, HasMore: false, TotalCount: 0}, nil).Once()

		tester.handler.ListModels(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("llm_types parameter is case-insensitive", func(t *testing.T) {
		tester, c, w := setupTest(t)
		tester.WithQuery("llm_types", "SERVERLESS")

		tester.mocks.openAIComp.EXPECT().
			ListModels(mock.Anything, "testuuid", types.ListModelsReq{LLMTypes: []string{"SERVERLESS"}}).
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
		ListModels(mock.Anything, "testuuid", types.ListModelsReq{}).
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
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "model1:svc1").Return(model, nil)

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
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "nonexistent:svc").Return(nil, nil)

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
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "xzgan001/gguf_model:fepjlx3v39xc").Return(model, nil)

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
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "simple-model:svc1").Return(model, nil)

		tester.handler.GetModel(c)

		assert.Equal(t, http.StatusOK, w.Code)
		var response types.Model
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "simple-model:svc1", response.ID)
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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

func TestOpenAIHandler_GetVideo_PersistsLongCatDownloadURL(t *testing.T) {
	tester, c, w := setupTest(t)
	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "0123456789abcdef0123456789abcdef",
		OwnerUUID:          "testuuid",
		ModelID:            "longcat-model",
		Status:             "queued",
	}
	downloadURL := "https://video-bucket.oss.example.com/aigateway/generated/videos/task.mp4"
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/tasks/0123456789abcdef0123456789abcdef/status", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"task_id":"0123456789abcdef0123456789abcdef","status":"succeed","download_url":%q}`, downloadURL)
	}))
	defer downstream.Close()

	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "longcat-model", Task: "audio-text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
		InternalModelInfo: types.InternalModelInfo{
			RuntimeFramework: "longcat-video",
			CSGHubModelID:    "meituan-longcat/LongCat-Video-Avatar-1.5",
		},
		Endpoint: downstream.URL,
		Upstreams: []commontypes.UpstreamConfig{
			{URL: downstream.URL, Enabled: true, ModelName: "longcat-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "longcat-model").Return(model, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().FindByResourceID(mock.Anything, database.AIGenerationResourceTypeVideo, "video_gateway").Return(&generation, nil).Once()
	tester.mocks.aiGenerationStore.EXPECT().UpdateWithStatus(mock.Anything, mock.MatchedBy(func(input database.AIGeneration) bool {
		generation = input
		return input.Status == "completed" && input.ProviderMetadata["download_url"] == downloadURL
	}), "queued").Return(true, nil).Once()

	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video_gateway", nil)
	c.Params = gin.Params{{Key: "video_id", Value: "video_gateway"}}
	tester.handler.GetVideo(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, downloadURL, generation.ProviderMetadata["download_url"])
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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

func TestOpenAIHandler_GetVideoContent_UsesPersistedDownloadURL(t *testing.T) {
	tester, _, _ := setupTest(t)
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/aigateway/generated/videos/task_123.mp4", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		require.Equal(t, "bytes=0-1023", r.Header.Get("Range"))
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("oss-video-bytes"))
	}))
	defer download.Close()

	generation := database.AIGeneration{
		ID:                 1,
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway",
		ProviderResourceID: "task_123",
		ProviderMetadata:   map[string]any{"download_url": download.URL + "/aigateway/generated/videos/task_123.mp4"},
		OwnerUUID:          "testuuid",
		ModelID:            "video-model",
		Status:             "completed",
	}
	model := &types.Model{
		BaseModel:         types.BaseModel{ID: "video-model", Task: "text-to-video"},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "opencsg"},
		Endpoint:          "http://provider.invalid",
		Upstreams: []commontypes.UpstreamConfig{
			{URL: "http://provider.invalid", Enabled: true, ModelName: "video-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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
	req.Header.Set("Authorization", "Bearer user-api-key")
	req.Header.Set("Range", "bytes=0-1023")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "oss-video-bytes", w.Body.String())
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuuid", "video-model").Return(model, nil).Once()
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

func TestValidateVideoMultipartAudio(t *testing.T) {
	require.NoError(t, validateVideoMultipartAudio(buildVideoAudioForm(t, 2, "audio/wav", []byte("RIFF1234WAVE"))))

	err := validateVideoMultipartAudio(buildVideoAudioForm(t, 3, "audio/wav", []byte("RIFF1234WAVE")))
	require.ErrorContains(t, err, "at most two")

	err = validateVideoMultipartAudio(buildVideoAudioForm(t, 1, "text/plain", []byte("not audio")))
	require.ErrorContains(t, err, "unsupported audio content type")
}

func TestValidateVideoAdapterCompatibilityAudio(t *testing.T) {
	longCatCaps := text2video.Capabilities{
		SupportsCreate:         true,
		SupportsAudioReference: true,
		RequiresAudioReference: true,
		MaxAudioReferences:     2,
	}
	err := validateVideoAdapterCompatibility(types.VideoGenerationRequest{}, true, false, 0, longCatCaps)
	require.ErrorContains(t, err, "requires at least one audio")
	require.NoError(t, validateVideoAdapterCompatibility(types.VideoGenerationRequest{}, true, false, 2, longCatCaps))

	err = validateVideoAdapterCompatibility(types.VideoGenerationRequest{}, true, false, 1, text2video.Capabilities{SupportsCreate: true})
	require.ErrorContains(t, err, "does not support audio-guided")
}

func buildVideoAudioForm(t *testing.T, count int, contentType string, content []byte) *multipart.Form {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for index := 0; index < count; index++ {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="audio"; filename="speaker-%d.wav"`, index))
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		require.NoError(t, err)
		_, err = part.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len()+1024)))
	return req.MultipartForm
}
