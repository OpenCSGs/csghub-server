package component

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockbldmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"

	"github.com/openai/openai-go/v3"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestOpenAIComponent_GetAvailableModels(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	comp := &openaiComponentImpl{
		userStore:      mockUserStore,
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
	}

	t.Run("user not found", func(t *testing.T) {
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "nonexistent").
			Return(database.User{}, errors.New("user not exists")).Once()

		models, err := comp.GetAvailableModels(context.Background(), "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, models)
	})

	t.Run("successful case", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()
		mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, mock.Anything).
			Return([]*database.LLMConfig{}, 0, nil)
		now := time.Now()
		deploys := []database.Deploy{
			{
				ID:      1,
				SvcName: "svc1",
				Type:    1,
				Repository: &database.Repository{
					Path: "model1",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint1",
				Task:     "text-generation",
			},
			{
				ID:      2,
				SvcName: "svc2",
				Type:    3, // serverless
				Repository: &database.Repository{
					HFPath: "hf-model2",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint2",
				Task:     "text-to-image",
			},
		}
		deploys[0].CreatedAt = now
		deploys[1].CreatedAt = now

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).
			Return(deploys, nil).Once()
		expectModels := []types.ModelWithEndpoint{
			{
				ID:       "model1:svc1",
				OwnedBy:  "testuser",
				Object:   "model",
				Endpoint: "endpoint1",
				Created:  deploys[0].CreatedAt.Unix(),
				Task:     "text-generation",
			},
			{
				ID:       "hf-model2:svc2",
				OwnedBy:  "OpenCSG",
				Object:   "model",
				Endpoint: "endpoint2",
				Created:  deploys[1].CreatedAt.Unix(),
				Task:     "text-to-image",
			},
		}
		var wg sync.WaitGroup
		wg.Add(1)
		for _, model := range expectModels {
			expectJson, _ := json.Marshal(model)
			mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, model.ID, string(expectJson)).
				Return(nil)
		}
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			})

		models, err := comp.GetAvailableModels(context.Background(), "testuser")
		assert.NoError(t, err)
		assert.Len(t, models, 2)

		// Verify first model
		assert.Equal(t, "model1:svc1", models[0].ID)
		assert.Equal(t, "testuser", models[0].OwnedBy)
		assert.Equal(t, "endpoint1", models[0].Endpoint)
		assert.Equal(t, "text-generation", models[0].Task)

		// Verify second model (serverless)
		assert.Equal(t, "hf-model2:svc2", models[1].ID)
		assert.Equal(t, "OpenCSG", models[1].OwnedBy)
		assert.Equal(t, "endpoint2", models[1].Endpoint)
		assert.Equal(t, "text-to-image", models[1].Task)
		wg.Wait()
	})
}

func TestOpenAIComponent_GetModelByID(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	comp := &openaiComponentImpl{
		userStore:      mockUserStore,
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
	}

	t.Run("model cache expire", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()
		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).
			Return(0, nil).Once()
		mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, mock.Anything).
			Return([]*database.LLMConfig{}, 0, nil).Once()
		now := time.Now()
		deploys := []database.Deploy{
			{
				ID:      1,
				SvcName: "svc1",
				Type:    1,
				Repository: &database.Repository{
					Path: "model1",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint1",
			},
		}
		deploys[0].CreatedAt = now
		var wg sync.WaitGroup
		wg.Add(1)
		expectModels := []types.ModelWithEndpoint{
			{
				ID:       "model1:svc1",
				OwnedBy:  "testuser",
				Object:   "model",
				Endpoint: "endpoint1",
				Created:  deploys[0].CreatedAt.Unix(),
			},
		}
		for _, model := range expectModels {
			expectJson, _ := json.Marshal(model)
			mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, model.ID, string(expectJson)).
				Return(nil).Once()
		}
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()
		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).Return(deploys, nil).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "model1:svc1")
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "model1:svc1", model.ID)
		wg.Wait()
	})

	t.Run("model not found", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()
		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).
			Return(1, nil).Once()
		mockCache.EXPECT().HGet(mock.Anything, modelCacheKey, "nonexistent:svc").
			Return("", redis.Nil).Once()
		model, err := comp.GetModelByID(context.Background(), "testuser", "nonexistent:svc")
		assert.NoError(t, err)
		assert.Nil(t, model)
	})

	t.Run("model found", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()
		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).
			Return(1, nil).Once()

		now := time.Now()
		deploys := []database.Deploy{
			{
				ID:      1,
				SvcName: "svc1",
				Type:    1,
				Repository: &database.Repository{
					Path: "model1",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint1",
			},
		}
		deploys[0].CreatedAt = now
		expectModel := types.Model{
			ID:       "model1:svc1",
			OwnedBy:  "testuser",
			Object:   "model",
			Endpoint: "endpoint1",
			Created:  deploys[0].CreatedAt.Unix(),
		}
		expectJson, _ := json.Marshal(expectModel)
		mockCache.EXPECT().HGet(mock.Anything, modelCacheKey, expectModel.ID).
			Return(string(expectJson), nil).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "model1:svc1")
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "model1:svc1", model.ID)
	})
}

func TestGetSceneFromSvcType(t *testing.T) {
	tests := []struct {
		name     string
		svcType  int
		expected int
	}{
		{
			name:     "inference type",
			svcType:  commontypes.InferenceType,
			expected: int(commontypes.SceneModelInference),
		},
		{
			name:     "serverless type",
			svcType:  commontypes.ServerlessType,
			expected: int(commontypes.SceneModelServerless),
		},
		{
			name:     "unknown type",
			svcType:  999, // Some arbitrary value not defined in commontypes
			expected: int(commontypes.SceneUnknow),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSceneFromSvcType(tt.svcType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOpenAIComponent_ExtGetAvailableModels_Error(t *testing.T) {
	ctx := context.Background()
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockDeployStore := mockdb.NewMockDeployTaskStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	component := &openaiComponentImpl{
		userStore:      mockUserStore,
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
	}
	searchType := 16
	search := &commontypes.SearchLLMConfig{
		Type: &searchType,
	}
	mockLLMConfigStore.EXPECT().Index(ctx, 50, 1, search).
		Return(nil, 0, errors.New("test error")).Once()
	user := &database.User{
		ID:       1,
		Username: "testuser",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
		Return(*user, nil).Once()
	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, user.ID).
		Return([]database.Deploy{}, nil)

	models, err := component.GetAvailableModels(ctx, "testuser")

	require.Nil(t, err)
	require.Nil(t, models)
}

func TestOpenAIComponent_ExtGetAvailableModels_SinglePage(t *testing.T) {
	ctx := context.Background()
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockDeployStore := mockdb.NewMockDeployTaskStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	component := &openaiComponentImpl{
		userStore:      mockUserStore,
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
	}
	mockModels := []*database.LLMConfig{
		{
			ID:          1,
			ModelName:   "test-model-1",
			ApiEndpoint: "http://test-endpoint-1.com",
			AuthHeader:  "Bearer test-token-1",
			Type:        16,
			Enabled:     true,
		},
	}
	expectModels := []types.ModelWithEndpoint{
		{
			ID:       "test-model-1",
			Endpoint: "http://test-endpoint-1.com",
			AuthHead: "Bearer test-token-1",
			Object:   "model",
			OwnedBy:  "OpenCSG",
		},
	}
	expectJson, _ := json.Marshal(expectModels[0])

	user := &database.User{
		ID:       1,
		Username: "testuser",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
		Return(*user, nil).Once()
	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, user.ID).
		Return([]database.Deploy{}, nil)
	searchType := 16
	search := &commontypes.SearchLLMConfig{
		Type: &searchType,
	}
	mockLLMConfigStore.EXPECT().Index(ctx, 50, 1, search).Return(mockModels, 1, nil)
	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "test-model-1", string(expectJson)).
		Return(nil).Once()
	var wg sync.WaitGroup
	wg.Add(1)
	mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
		RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
			wg.Done()
			return nil
		})
	models, err := component.GetAvailableModels(ctx, "testuser")

	require.Nil(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "test-model-1", models[0].ID)
	wg.Wait()
}

func TestOpenAIComponentImpl_RecordUsage(t *testing.T) {
	cfg, err := config.LoadConfig()
	require.Nil(t, err)

	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}

	var mockCounter *mocktoken.MockCounter
	var comp *openaiComponentImpl

	tests := []struct {
		name      string
		userUUID  string
		model     *types.Model
		usage     *openai.CompletionUsage
		wantError bool
		setupMock func()
	}{
		{
			name:     "successful record - dedicated inference",
			userUUID: "test-user-uuid",
			model: &types.Model{
				CSGHubModelID: "test-model",
				SvcName:       "test-service",
				SvcType:       commontypes.InferenceType,
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: false,
			setupMock: func() {

				mockMQ := mockmq.NewMockMessageQueue(t)
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
					MQ:           mockBLDMQ,
					Cfg:          cfg,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "test-model", evt.ResourceID)
					require.Equal(t, "test-model", evt.ResourceName)
					require.Equal(t, "test-service", evt.CustomerID)
					require.Equal(t, int(commontypes.SceneModelInference), evt.Scene)
					require.Equal(t, "test-user-uuid", evt.UserUUID)
					require.Equal(t, commontypes.TokenNumberType, evt.ValueType)
					require.Equal(t, int64(150), evt.Value)
					var tokenUsageExtra struct {
						PromptTokenNum     string `json:"prompt_token_num"`
						CompletionTokenNum string `json:"completion_token_num"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, "100", tokenUsageExtra.PromptTokenNum)
					require.Equal(t, "50", tokenUsageExtra.CompletionTokenNum)
					return nil
				})
			},
		},
		{
			name:     "successful record - serverless inference",
			userUUID: "test-user-uuid",
			model: &types.Model{
				CSGHubModelID: "test-model",
				SvcName:       "test-service",
				SvcType:       commontypes.ServerlessType,
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: false,
			setupMock: func() {
				mockMQ := mockmq.NewMockMessageQueue(t)
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
					MQ:           mockBLDMQ,
					Cfg:          cfg,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "test-model", evt.ResourceID)
					require.Equal(t, "test-model", evt.ResourceName)
					require.Equal(t, "test-service", evt.CustomerID)
					require.Equal(t, int(commontypes.SceneModelServerless), evt.Scene)
					require.Equal(t, "test-user-uuid", evt.UserUUID)
					require.Equal(t, commontypes.TokenNumberType, evt.ValueType)
					require.Equal(t, int64(150), evt.Value)
					var tokenUsageExtra struct {
						PromptTokenNum     string `json:"prompt_token_num"`
						CompletionTokenNum string `json:"completion_token_num"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, "100", tokenUsageExtra.PromptTokenNum)
					require.Equal(t, "50", tokenUsageExtra.CompletionTokenNum)
					return nil

				})
			},
		},
		{
			name:     "counter error",
			userUUID: "test-user-uuid",
			model: &types.Model{
				CSGHubModelID: "test-model",
				SvcName:       "test-service",
				SvcType:       commontypes.InferenceType,
			},
			wantError: true,
			setupMock: func() {
				mockMQ := mockmq.NewMockMessageQueue(t)
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(nil, errors.New("counter error"))
			},
		},
		{
			name:     "publish error",
			userUUID: "test-user-uuid",
			model: &types.Model{
				CSGHubModelID: "test-model",
				SvcName:       "test-service",
				SvcType:       commontypes.InferenceType,
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: true,
			setupMock: func() {
				mockMQ := mockmq.NewMockMessageQueue(t)
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
					MQ:           mockBLDMQ,
					Cfg:          cfg,
				}
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)
				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).Return(errors.New("publish error")).Times(3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			err := comp.RecordUsage(context.Background(), tt.userUUID, tt.model, mockCounter)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
