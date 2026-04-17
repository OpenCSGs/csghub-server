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
		modelIDFmt:     "%s(%s)",
	}

	t.Run("user not found", func(t *testing.T) {
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "nonexistent").
			Return(database.User{}, errors.New("user not exists")).Once()

		models, err := comp.GetAvailableModels(context.Background(), "nonexistent")
		assert.Error(t, err)
		assert.Nil(t, models)
	})

	t.Run("anonymous user can see public CSGHub models", func(t *testing.T) {
		now := time.Now()
		deploys := []database.Deploy{
			{
				ID:          1,
				SvcName:     "svc1",
				Type:        commontypes.InferenceType,
				UserID:      1,
				SecureLevel: commontypes.EndpointPublic,
				Repository: &database.Repository{
					Name: "model1",
					Path: "model1",
				},
				User: &database.User{
					Username: "publicuser",
					UUID:     "publicuser-uuid",
				},
				Endpoint: "endpoint1",
				Task:     "text-generation",
			},
			{
				ID:          2,
				SvcName:     "svc2",
				Type:        commontypes.ServerlessType,
				UserID:      2,
				SecureLevel: commontypes.EndpointPublic,
				Repository: &database.Repository{
					HFPath: "hf-model2",
				},
				User: &database.User{
					Username: "serverless-owner",
					UUID:     "serverless-owner-uuid",
				},
				Endpoint: "endpoint2",
				Task:     "text-to-image",
			},
		}
		deploys[0].CreatedAt = now
		deploys[1].CreatedAt = now

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(0)).
			Return(deploys, nil).Once()
		mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, mock.Anything).
			Return([]*database.LLMConfig{}, 0, nil)

		var wg sync.WaitGroup
		wg.Add(1)
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "model1:svc1(publicuser)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "hf-model2:svc2(OpenCSG)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()

		models, err := comp.GetAvailableModels(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, models, 2)
		assert.Equal(t, "model1:svc1", models[0].ID)
		assert.Equal(t, "publicuser", models[0].OwnedBy)
		assert.Equal(t, "hf-model2:svc2", models[1].ID)
		assert.Equal(t, "OpenCSG", models[1].OwnedBy)
		wg.Wait()
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
				ID:          1,
				SvcName:     "svc1",
				Type:        1,
				UserID:      1,
				SecureLevel: commontypes.EndpointPublic,
				Repository: &database.Repository{
					Name: "model1",
					Path: "model1",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint1",
				Task:     "text-generation",
			},
			{
				ID:          2,
				SvcName:     "svc2",
				Type:        3, // serverless
				UserID:      1,
				SecureLevel: commontypes.EndpointPublic,
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
		var wg sync.WaitGroup
		wg.Add(1)
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "model1:svc1(testuser)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "hf-model2:svc2(OpenCSG)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()

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

	t.Run("inference private should be marked private", func(t *testing.T) {
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
				ID:          3,
				SvcName:     "svc3",
				Type:        commontypes.InferenceType,
				UserID:      1,
				SecureLevel: commontypes.EndpointPrivate,
				Repository: &database.Repository{
					Name: "model3",
					Path: "model3",
				},
				User: &database.User{
					Username: "testuser",
				},
				Endpoint: "endpoint3",
				Task:     "text-generation",
			},
		}
		deploys[0].CreatedAt = now

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).
			Return(deploys, nil).Once()

		var wg sync.WaitGroup
		wg.Add(1)
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "model3:svc3(testuser)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()

		models, err := comp.GetAvailableModels(context.Background(), "testuser")
		assert.NoError(t, err)
		assert.Len(t, models, 1)
		assert.Equal(t, "model3:svc3", models[0].ID)
		assert.Equal(t, "testuser", models[0].OwnedBy)
		wg.Wait()
	})

}

func TestOpenAIComponent_GetAvailableModels_CacheUsesModelSnapshot(t *testing.T) {
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	comp := &openaiComponentImpl{
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
		modelIDFmt:     "%s(%s)",
	}

	now := time.Now()
	deploys := []database.Deploy{
		{
			ID:          1,
			SvcName:     "svc1",
			Type:        commontypes.InferenceType,
			UserID:      1,
			SecureLevel: commontypes.EndpointPublic,
			Repository: &database.Repository{
				Name: "model1",
				Path: "model1",
			},
			User: &database.User{
				Username: "publicuser",
				UUID:     "publicuser-uuid",
			},
			Endpoint: "endpoint1",
			Task:     "text-generation",
		},
		{
			ID:          2,
			SvcName:     "svc2",
			Type:        commontypes.ServerlessType,
			UserID:      2,
			SecureLevel: commontypes.EndpointPublic,
			Repository: &database.Repository{
				HFPath: "hf-model2",
			},
			User: &database.User{
				Username: "serverless-owner",
				UUID:     "serverless-owner-uuid",
			},
			Endpoint: "endpoint2",
			Task:     "text-to-image",
		},
	}
	deploys[0].CreatedAt = now
	deploys[1].CreatedAt = now

	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(0)).
		Return(deploys, nil).Once()
	mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, mock.Anything).
		Return([]*database.LLMConfig{}, 0, nil).Once()

	firstWriteStarted := make(chan struct{})
	continueFirstWrite := make(chan struct{})
	cacheCompleted := make(chan struct{})

	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "model1:svc1(publicuser)", mock.Anything).
		RunAndReturn(func(ctx context.Context, key string, field string, value any) error {
			close(firstWriteStarted)
			<-continueFirstWrite
			return nil
		}).Once()
	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "hf-model2:svc2(OpenCSG)", mock.Anything).
		RunAndReturn(func(ctx context.Context, key string, field string, value any) error {
			valueString, ok := value.(string)
			require.True(t, ok)

			var cachedModel types.Model
			require.NoError(t, json.Unmarshal([]byte(valueString), &cachedModel))
			assert.Equal(t, "hf-model2:svc2", cachedModel.ID)
			assert.Equal(t, types.ProviderTypeServerless, cachedModel.Metadata[types.MetaKeyLLMType])
			return nil
		}).Once()
	mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
		RunAndReturn(func(ctx context.Context, key string, ttl time.Duration) error {
			close(cacheCompleted)
			return nil
		}).Once()

	models, err := comp.GetAvailableModels(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, models, 2)

	select {
	case <-firstWriteStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async cache write to start")
	}

	models[1].ID = "mutated:svc2"
	models[1].Metadata[types.MetaKeyLLMType] = "mutated"

	close(continueFirstWrite)

	select {
	case <-cacheCompleted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async cache write to finish")
	}
}

func TestOpenAIComponent_ListModels_CacheUsesOriginalIDWhenFormatModelIDApplied(t *testing.T) {
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	mockCache := mockcache.NewMockRedisClient(t)
	comp := &openaiComponentImpl{
		deployStore:    mockDeployStore,
		extllmStore:    mockLLMConfigStore,
		modelListCache: mockCache,
		modelIDFmt:     "%s(%s)",
	}

	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(0)).
		Return([]database.Deploy{}, nil).Once()

	searchType := 16
	enabled := true
	search := &commontypes.SearchLLMConfig{
		Type:    &searchType,
		Enabled: &enabled,
	}
	mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, search).
		Return([]*database.LLMConfig{
			{
				ID:                 1,
				ModelName:          "test-model-1",
				OfficialName:       "Test Model 1",
				ApiEndpoint:        "http://test-endpoint-1.com",
				AuthHeader:         "Bearer test-token-1",
				Provider:           "OpenAI",
				Type:               16,
				Enabled:            true,
				Metadata:           map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				NeedSensitiveCheck: true,
			},
			{
				ID:                 2,
				ModelName:          "test-model-2",
				OfficialName:       "Test Model 2",
				ApiEndpoint:        "http://test-endpoint-2.com",
				AuthHeader:         "Bearer test-token-2",
				Provider:           "Anthropic",
				Type:               16,
				Enabled:            true,
				Metadata:           map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				NeedSensitiveCheck: true,
			},
		}, 2, nil).Once()

	firstWriteStarted := make(chan struct{})
	continueFirstWrite := make(chan struct{})
	cacheCompleted := make(chan struct{})

	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "test-model-1(OpenAI)", mock.Anything).
		RunAndReturn(func(ctx context.Context, key string, field string, value any) error {
			close(firstWriteStarted)
			<-continueFirstWrite
			return nil
		}).Once()
	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "test-model-2(Anthropic)", mock.Anything).
		RunAndReturn(func(ctx context.Context, key string, field string, value any) error {
			valueString, ok := value.(string)
			require.True(t, ok)

			var cachedModel types.Model
			require.NoError(t, json.Unmarshal([]byte(valueString), &cachedModel))
			assert.Equal(t, "test-model-2", cachedModel.ID)
			assert.Equal(t, "Anthropic", cachedModel.Provider)
			assert.Equal(t, types.ProviderTypeExternalLLM, cachedModel.Metadata[types.MetaKeyLLMType])
			return nil
		}).Once()
	mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
		RunAndReturn(func(ctx context.Context, key string, ttl time.Duration) error {
			close(cacheCompleted)
			return nil
		}).Once()

	modelList, err := comp.ListModels(context.Background(), "", types.ListModelsReq{})
	require.NoError(t, err)
	require.Len(t, modelList.Data, 2)
	assert.Equal(t, "test-model-1(OpenAI)", modelList.Data[0].ID)
	assert.Equal(t, "test-model-2(Anthropic)", modelList.Data[1].ID)

	select {
	case <-firstWriteStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async cache write to start")
	}

	close(continueFirstWrite)

	select {
	case <-cacheCompleted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async cache write to finish")
	}
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
		modelIDFmt:     "%s(%s)",
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
					Name: "model1",
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
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "model1:svc1(testuser)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()
		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).Return(deploys, nil).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "model1:svc1(testuser)")
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
		// Cache miss: GetModelByID falls through to GetAvailableModels, which calls getCSGHubModels and getExternalModels
		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).Return([]database.Deploy{}, nil).Once()
		mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, mock.Anything).
			Return([]*database.LLMConfig{}, 0, nil).Once()
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
					Name: "model1",
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
				ID:           "model1:svc1",
				OwnedBy:      "testuser",
				Object:       "model",
				Created:      deploys[0].CreatedAt.Unix(),
				Task:         "text-generation",
				OfficialName: "model1",
				Metadata: map[string]any{
					types.MetaKeyLLMType: types.ProviderTypeInference,
				},
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

	t.Run("formatted external model id can match precomputed key", func(t *testing.T) {
		user := &database.User{
			ID:       1,
			Username: "testuser",
		}
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
			Return(*user, nil).Once()
		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).
			Return(1, nil).Once()
		mockCache.EXPECT().HGet(mock.Anything, modelCacheKey, "test-model-1(OpenAI)").
			Return("", redis.Nil).Once()

		mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).
			Return([]database.Deploy{}, nil).Once()
		searchType := 16
		enabled := true
		search := &commontypes.SearchLLMConfig{
			Type:    &searchType,
			Enabled: &enabled,
		}
		mockLLMConfigStore.EXPECT().Index(mock.Anything, 50, 1, search).
			Return([]*database.LLMConfig{
				{
					ID:          1,
					ModelName:   "test-model-1",
					ApiEndpoint: "http://test-endpoint-1.com",
					AuthHeader:  "Bearer test-token-1",
					Provider:    "OpenAI",
					Type:        16,
					Enabled:     true,
				},
			}, 1, nil).Once()

		var wg sync.WaitGroup
		wg.Add(1)
		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "test-model-1(OpenAI)", mock.Anything).
			Return(nil).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			RunAndReturn(func(ctx context.Context, s string, d time.Duration) error {
				wg.Done()
				return nil
			}).Once()

		model, err := comp.GetModelByID(context.Background(), "testuser", "test-model-1(OpenAI)")
		assert.NoError(t, err)
		assert.NotNil(t, model)
		assert.Equal(t, "test-model-1", model.ID)
		wg.Wait()
	})
}

func TestOpenAIComponent_saveModelsToCache(t *testing.T) {
	t.Run("uses format model id as hash field and sets ttl", func(t *testing.T) {
		mockCache := mockcache.NewMockRedisClient(t)
		comp := &openaiComponentImpl{modelListCache: mockCache}

		models := []types.Model{
			{
				BaseModel: types.BaseModel{
					ID:           "base-model-id",
					Object:       "model",
					OwnedBy:      "openai",
					OfficialName: "gpt-4o",
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeExternalLLM,
					},
				},
				Endpoint: "http://test-endpoint",
				ExternalModelInfo: types.ExternalModelInfo{
					Provider:           "openai",
					AuthHead:           "Bearer test-token",
					FormatModelID:      "base-model-id(openai)",
					NeedSensitiveCheck: true,
				},
			},
		}

		mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "base-model-id(openai)", mock.Anything).
			RunAndReturn(func(ctx context.Context, key string, field string, value interface{}) error {
				valueString, ok := value.(string)
				require.True(t, ok)

				var cachedModel types.Model
				require.NoError(t, json.Unmarshal([]byte(valueString), &cachedModel))
				assert.Equal(t, "base-model-id", cachedModel.ID)
				assert.Equal(t, "openai", cachedModel.Provider)
				assert.Equal(t, "Bearer test-token", cachedModel.AuthHead)
				return nil
			}).Once()
		mockCache.EXPECT().Expire(mock.Anything, modelCacheKey, modelCacheTTL).
			Return(nil).Once()

		err := comp.saveModelsToCache(models)
		require.NoError(t, err)
	})
}

func TestOpenAIComponent_loadModelFromCache(t *testing.T) {
	t.Run("cache key not exists returns nil model", func(t *testing.T) {
		mockCache := mockcache.NewMockRedisClient(t)
		comp := &openaiComponentImpl{modelListCache: mockCache}

		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).Return(0, nil).Once()

		model, err := comp.loadModelFromCache(context.Background(), "test-model(OpenAI)")
		require.NoError(t, err)
		assert.Nil(t, model)
	})

	t.Run("load cached model by format model id", func(t *testing.T) {
		mockCache := mockcache.NewMockRedisClient(t)
		comp := &openaiComponentImpl{modelListCache: mockCache}

		cachedModel := types.Model{
			BaseModel: types.BaseModel{
				ID:           "test-model",
				Object:       "model",
				OwnedBy:      "OpenAI",
				OfficialName: "test-model",
				Metadata: map[string]any{
					types.MetaKeyLLMType: types.ProviderTypeExternalLLM,
				},
			},
			Endpoint: "http://test-endpoint",
			ExternalModelInfo: types.ExternalModelInfo{
				Provider:           "OpenAI",
				AuthHead:           "Bearer test-token",
				NeedSensitiveCheck: true,
			},
		}
		cachedModel.ForInternalUse()
		cachedJSON, err := json.Marshal(cachedModel)
		require.NoError(t, err)

		mockCache.EXPECT().Exists(mock.Anything, modelCacheKey).Return(1, nil).Once()
		mockCache.EXPECT().HGet(mock.Anything, modelCacheKey, "test-model(OpenAI)").
			Return(string(cachedJSON), nil).Once()

		model, err := comp.loadModelFromCache(context.Background(), "test-model(OpenAI)")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, "test-model", model.ID)
		assert.Equal(t, "OpenAI", model.Provider)
		assert.Equal(t, "Bearer test-token", model.AuthHead)
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
		modelIDFmt:     "%s(%s)",
	}
	searchType := 16
	enabled := true
	search := &commontypes.SearchLLMConfig{
		Type:    &searchType,
		Enabled: &enabled,
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
		modelIDFmt:     "%s(%s)",
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
	user := &database.User{
		ID:       1,
		Username: "testuser",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").
		Return(*user, nil).Once()
	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, user.ID).
		Return([]database.Deploy{}, nil)
	searchType := 16
	enabled := true
	search := &commontypes.SearchLLMConfig{
		Type:    &searchType,
		Enabled: &enabled,
	}
	mockLLMConfigStore.EXPECT().Index(ctx, 50, 1, search).Return(mockModels, 1, nil)
	mockCache.EXPECT().HSet(mock.Anything, modelCacheKey, "test-model-1(OpenAI)", mock.Anything).
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
