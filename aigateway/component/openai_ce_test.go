//go:build !ee && !saas

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
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
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
			UUID:     "testuser-uuid",
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
		expectModels := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "model1:svc1",
					OwnedBy: "testuser",
					Object:  "model",
					Created: deploys[0].CreatedAt.Unix(),
					Task:    "text-generation",
				},
				Endpoint: "endpoint1",
				InternalModelInfo: types.InternalModelInfo{
					ClusterID: deploys[0].ClusterID,
					SvcName:   deploys[0].SvcName,
					ImageID:   deploys[0].ImageID,
				},
				InternalUse: true,
			},
			{
				BaseModel: types.BaseModel{
					ID:      "hf-model2:svc2",
					OwnedBy: "OpenCSG",
					Object:  "model",
					Created: deploys[1].CreatedAt.Unix(),
					Task:    "text-to-image",
				},
				Endpoint: "endpoint2",
				InternalModelInfo: types.InternalModelInfo{
					ClusterID: deploys[1].ClusterID,
					SvcName:   deploys[1].SvcName,
					ImageID:   deploys[1].ImageID,
				},
				InternalUse: true,
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
			UUID:     "testuser-uuid",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil)
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
		expectModels := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:      "model1:svc1",
					OwnedBy: "testuser",
					Object:  "model",
					Created: deploys[0].CreatedAt.Unix(),
				},
				Endpoint: "endpoint1",
				InternalModelInfo: types.InternalModelInfo{
					ClusterID: deploys[0].ClusterID,
					SvcName:   deploys[0].SvcName,
					ImageID:   deploys[0].ImageID,
				},
				InternalUse: true,
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
			BaseModel: types.BaseModel{
				ID:      "model1:svc1",
				OwnedBy: "testuser",
				Object:  "model",
				Created: deploys[0].CreatedAt.Unix(),
				Task:    "text-generation",
			},
			Endpoint: "endpoint1",
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
			Provider:    "OpenAI",
			Type:        16,
			Enabled:     true,
		},
	}
	expectModels := []types.Model{
		{
			BaseModel: types.BaseModel{
				ID:      "test-model-1",
				OwnedBy: "OpenAI",
				Object:  "model",
			},
			Endpoint: "http://test-endpoint-1.com",
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "OpenAI",
				AuthHead: "Bearer test-token-1",
			},
			InternalUse: true,
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
