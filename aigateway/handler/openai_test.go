package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	rpcmock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	apicomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
)

type testerOpenAIHandler struct {
	mocks struct {
		openAIComp     *mockcomp.MockOpenAIComponent
		moderationComp *mockcomp.MockModeration
		repoComp       *apicomp.MockRepoComponent
		modSvcClient   *rpcmock.MockModerationSvcClient
		mockClsComp    *apicomp.MockClusterComponent
	}

	handler *OpenAIHandlerImpl
}

func setupTest(t *testing.T) (*testerOpenAIHandler, *gin.Context, *httptest.ResponseRecorder) {
	mockOpenAI := mockcomp.NewMockOpenAIComponent(t)
	mockRepo := apicomp.NewMockRepoComponent(t)
	modSvcClient := rpcmock.NewMockModerationSvcClient(t)
	mockModeration := mockcomp.NewMockModeration(t)
	mockClsComp := apicomp.NewMockClusterComponent(t)
	handler := newOpenAIHandler(mockOpenAI, mockRepo, modSvcClient, mockModeration, mockClsComp)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	// Set test user
	httpbase.SetCurrentUser(c, "testuser")
	httpbase.SetCurrentUserUUID(c, "testuuid")
	tester := &testerOpenAIHandler{
		handler: handler,
	}
	tester.mocks.modSvcClient = modSvcClient
	tester.mocks.moderationComp = mockModeration
	tester.mocks.openAIComp = mockOpenAI
	tester.mocks.repoComp = mockRepo
	tester.mocks.mockClsComp = mockClsComp
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
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: param.Opt[string]{
								Value: "Hello",
							},
						},
					},
				},
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
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: param.Opt[string]{
								Value: "Hello",
							},
						},
					},
				},
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
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: param.Opt[string]{
								Value: "Hello",
							},
						},
					},
				},
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
		tester.mocks.moderationComp.EXPECT().CheckLLMPrompt(mock.Anything, *chatReq.Messages[0].GetContent().AsAny().(*string), "testuuid"+model.ID).
			Return(&rpc.CheckResult{IsSensitive: true}, nil)
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("llm prompt sensitive check failed", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: param.Opt[string]{
								Value: "Hello",
							},
						},
					},
				},
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
		tester.mocks.moderationComp.EXPECT().CheckLLMPrompt(mock.Anything, *chatReq.Messages[0].GetContent().AsAny().(*string), "testuuid"+model.ID).
			Return(nil, errors.New("some error"))
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("llm prompt with unhandled struct", func(t *testing.T) {
		tester, c, w := setupTest(t)
		chatReq := ChatCompletionRequest{
			Model: "model1:svc1",
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
								{
									OfText: &openai.ChatCompletionContentPartTextParam{
										Text: "hello1",
									},
								},
								{
									OfText: &openai.ChatCompletionContentPartTextParam{
										Text: "hello2",
									},
								},
							},
						},
					},
				},
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

		tokenizer := token.NewTokenizerImpl(model.Endpoint, "", model.Object, "")
		llmTokenCounter := token.NewLLMTokenCounter(tokenizer)
		tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "test-cls").Return(&database.ClusterInfo{
			ClusterID: "test-cls",
		}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().RecordUsage(mock.Anything, "testuuid", model, llmTokenCounter).Return(nil).Run(func(c context.Context, userUUID string, model *types.Model, tokenCounter token.Counter) {
			wg.Done()
		})
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusOK, w.Code)
		wg.Wait()
	})
	t.Run("llm prompt with unhandled raw json", func(t *testing.T) {
		tester, c, w := setupTest(t)
		body := []byte("{\"model\":\"model1:svc1\",\"messages\":[{\"role\":\"system\",\"content\":\"You are a helpful assistant that can use tools to answer questions and perform tasks.\"},{\"role\":\"user\",\"content\":[\"a\",\"b\"]}],\"stream\":true,\"temperature\":0.1}")
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
		tester.mocks.moderationComp.EXPECT().CheckLLMPrompt(mock.Anything, "You are a helpful assistant that can use tools to answer questions and perform tasks.", "testuuid"+model.ID).
			Return(&rpc.CheckResult{IsSensitive: true}, nil)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1:svc1").Return(model, nil)
		tester.handler.Chat(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
