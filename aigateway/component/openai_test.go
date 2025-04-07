package component

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/aigateway/types"

	"github.com/openai/openai-go"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestOpenAIComponent_GetAvailableModels(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}

	comp := &openaiComponentImpl{
		userStore:   mockUserStore,
		deployStore: mockDeployStore,
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
	})
}

func TestOpenAIComponent_GetModelByID(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}

	comp := &openaiComponentImpl{
		userStore:   mockUserStore,
		deployStore: mockDeployStore,
	}

	t.Run("model found", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()

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

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).Return(deploys, nil).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "model1:svc1")
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "model1:svc1", model.ID)
	})

	t.Run("model not found", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).
			Return([]database.Deploy{}, nil).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "nonexistent:svc")
		assert.NoError(t, err)
		assert.Nil(t, model)
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

func TestOpenAIComponentImpl_RecordUsage(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}

	var mockCounter *mocktoken.MockLLMTokenCounter
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
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
				}
				mockCounter = mocktoken.NewMockLLMTokenCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&openai.CompletionUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)
				mockMQ.EXPECT().VerifyMeteringStream().Return(nil)
				mockMQ.EXPECT().PublishMeterDurationData(mock.Anything).RunAndReturn(func(data []byte) error {
					var evt commontypes.METERING_EVENT
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
						PromptTokenNum     int64 `json:"prompt_token_num"`
						CompletionTokenNum int64 `json:"completion_token_num"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, int64(100), tokenUsageExtra.PromptTokenNum)
					require.Equal(t, int64(50), tokenUsageExtra.CompletionTokenNum)
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
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
				}
				mockCounter = mocktoken.NewMockLLMTokenCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&openai.CompletionUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)
				mockMQ.EXPECT().VerifyMeteringStream().Return(nil)
				mockMQ.EXPECT().PublishMeterDurationData(mock.Anything).RunAndReturn(func(data []byte) error {
					var evt commontypes.METERING_EVENT
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
						PromptTokenNum     int64 `json:"prompt_token_num"`
						CompletionTokenNum int64 `json:"completion_token_num"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, int64(100), tokenUsageExtra.PromptTokenNum)
					require.Equal(t, int64(50), tokenUsageExtra.CompletionTokenNum)
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
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
				}
				mockCounter = mocktoken.NewMockLLMTokenCounter(t)

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
				eventPub := &event.EventPublisher{
					Connector:    mockMQ,
					SyncInterval: 1,
				}
				mockCounter = mocktoken.NewMockLLMTokenCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage().Return(&openai.CompletionUsage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)
				mockMQ.EXPECT().VerifyMeteringStream().Return(nil)
				mockMQ.EXPECT().PublishMeterDurationData(mock.Anything).Return(errors.New("publish error")).Times(3)
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
