//go:build !ee && !saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestOpenAIComponent_GetAvailableModels_ExternalModel(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "gpt-4",
				Type:      database.LLMTypeAigatewayExternal,
				Enabled:   true,
				Provider:  "openai",
				Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				Upstreams: []database.Upstream{
					{
						ID:         1,
						Source:     commontypes.UpstreamSourceExternal,
						URL:        "http://openai-api/v1",
						Enabled:    true,
						Provider:   "openai",
						AuthHeader: "Bearer sk-xxx",
					},
				},
			},
		}, 1, nil).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4", models[0].ID)
	assert.Equal(t, "openai", models[0].OwnedBy)
	assert.Equal(t, "text-generation", models[0].Task)
	assert.Equal(t, commontypes.ProviderTypeExternalLLM, models[0].Metadata[types.MetaKeyLLMType])
	assert.Len(t, models[0].Upstreams, 1)
}

func TestOpenAIComponent_GetAvailableModels_ServerlessVisibleToAll(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:    "ns/serverless-model",
		HFPath:           "ns/serverless-model",
		LegacyModelID:    "ns/serverless-model",
		OwnerUUID:        "owner-uuid",
		OwnerUsername:    "owner",
		SvcType:          commontypes.ServerlessType,
		SvcName:          "svc1",
		SourceDeployID:   1,
		RuntimeFramework: "vllm",
	}
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "ns/serverless-model",
				Enabled:   true,
				Upstreams: []database.Upstream{
					{
						ID:        1,
						Source:    commontypes.UpstreamSourceCSGHubDeploy,
						URL:       "http://serverless-endpoint/v1",
						Enabled:   true,
						ModelName: "ns/serverless-model",
						Metadata: &commontypes.UpstreamMetadata{
							InternalModelInfo: internalInfo,
						},
					},
				},
			},
		}, 1, nil).Once()

	// Any user can see serverless models
	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "any-user-uuid", UserUUID: "any-user-uuid"}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "ns/serverless-model", models[0].ID)
	assert.Equal(t, "OpenCSG", models[0].OwnedBy)
	assert.Equal(t, commontypes.ProviderTypeServerless, models[0].Metadata[types.MetaKeyLLMType])
	assert.Equal(t, "ns/serverless-model", models[0].Metadata[types.MetaKeyRepoPath])
	assert.Equal(t, "owner-uuid", models[0].OwnerUUID)
}

func TestOpenAIComponent_GetAvailableModels_InferenceOnlyVisibleToOwner(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/inference-model",
		LegacyModelID:  "ns/inference-model:2",
		RepoName:       "ns/inference-model",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc2",
		SourceDeployID: 2,
	}
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "ns/inference-model:2",
				Enabled:   true,
				Upstreams: []database.Upstream{
					{
						ID:        1,
						Source:    commontypes.UpstreamSourceCSGHubDeploy,
						URL:       "http://inference-endpoint/v1",
						Enabled:   true,
						ModelName: "ns/inference-model",
						Metadata: &commontypes.UpstreamMetadata{
							InternalModelInfo: internalInfo,
						},
					},
				},
			},
		}, 1, nil).Once()

	// Non-owner: should not see inference model
	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "other-user-uuid", UserUUID: "other-user-uuid"}})
	require.NoError(t, err)
	assert.Empty(t, models)
}

func TestOpenAIComponent_GetAvailableModels_InferenceVisibleToOwner(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/inference-model",
		LegacyModelID:  "ns/inference-model:2",
		RepoName:       "ns/inference-model",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc2",
		SourceDeployID: 2,
	}
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "ns/inference-model:2",
				Enabled:   true,
				Upstreams: []database.Upstream{
					{
						ID:        1,
						Source:    commontypes.UpstreamSourceCSGHubDeploy,
						URL:       "http://inference-endpoint/v1",
						Enabled:   true,
						ModelName: "ns/inference-model",
						Metadata: &commontypes.UpstreamMetadata{
							InternalModelInfo: internalInfo,
						},
					},
				},
			},
		}, 1, nil).Once()

	// Owner: should see inference model
	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "ns/inference-model:2", models[0].ID)
	assert.Equal(t, "owner", models[0].OwnedBy)
	assert.Equal(t, commontypes.ProviderTypeInference, models[0].Metadata[types.MetaKeyLLMType])
}

func TestOpenAIComponent_GetAvailableModels_MultiplePages(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	page1 := make([]*database.LLMConfig, 50)
	for i := range page1 {
		page1[i] = &database.LLMConfig{
			ID:        int64(i + 1),
			ModelName: "model-" + string(rune('a'+i)),
			Enabled:   true,
			Provider:  "openai",
			Upstreams: []database.Upstream{
				{
					ID:         int64(i + 1),
					Source:     commontypes.UpstreamSourceExternal,
					URL:        "http://endpoint/v1",
					Enabled:    true,
					Provider:   "openai",
					AuthHeader: "Bearer sk-xxx",
				},
			},
		}
	}
	page2 := []*database.LLMConfig{
		{
			ID:        51,
			ModelName: "model-z",
			Enabled:   true,
			Provider:  "anthropic",
			Upstreams: []database.Upstream{
				{
					ID:         51,
					Source:     commontypes.UpstreamSourceExternal,
					URL:        "http://endpoint/v1",
					Enabled:    true,
					Provider:   "anthropic",
					AuthHeader: "Bearer sk-yyy",
				},
			},
		},
	}

	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return(page1, 50, nil).Once()
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 2, mock.Anything, mock.Anything).
		Return(page2, 1, nil).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.NoError(t, err)
	assert.Len(t, models, 51)
}

func TestOpenAIComponent_GetAvailableModels_RepoPathFromRelation(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	originalMetadata := map[string]any{types.MetaKeyTasks: []any{"text-generation"}}
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "test-model-1",
				Enabled:   true,
				Provider:  "OpenAI",
				Metadata:  originalMetadata,
				RepoID:    100,
				Repo: &database.Repository{
					ID:      100,
					Path:    "test-ns/test-model-1",
					GitPath: "models/test-ns/test-model-1",
				},
				Upstreams: []database.Upstream{
					{
						ID:         1,
						Source:     commontypes.UpstreamSourceExternal,
						URL:        "http://openai-api/v1",
						Enabled:    true,
						Provider:   "OpenAI",
						AuthHeader: "Bearer sk-xxx",
					},
				},
			},
		}, 1, nil).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "test-model-1", models[0].ID)
	require.Equal(t, "test-ns/test-model-1", models[0].Metadata[types.MetaKeyRepoPath])
	require.Equal(t, commontypes.ProviderTypeExternalLLM, models[0].Metadata[types.MetaKeyLLMType])
	// Original metadata should not be mutated
	require.NotContains(t, originalMetadata, types.MetaKeyRepoPath)
	require.NotContains(t, originalMetadata, types.MetaKeyLLMType)
}

func TestOpenAIComponent_GetAvailableModels_DBError(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return(nil, 0, errors.New("db error")).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.Error(t, err)
	assert.Nil(t, models)
}

func TestOpenAIComponent_GetAvailableModels_EmptyResult(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{}, 0, nil).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.NoError(t, err)
	assert.Empty(t, models)
}

func TestOpenAIComponent_GetAvailableModels_SkipsCSGHubWithoutInternalInfo(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "broken-model",
				Enabled:   true,
				Upstreams: []database.Upstream{
					{
						ID:        1,
						Source:    commontypes.UpstreamSourceCSGHubDeploy,
						URL:       "http://broken/v1",
						Enabled:   true,
						ModelName: "broken-model",
						// Metadata is nil — should be skipped
					},
				},
			},
		}, 1, nil).Once()

	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "", UserUUID: ""}})
	require.NoError(t, err)
	assert.Empty(t, models)
}

func TestOpenAIComponent_GetAvailableModels_MixedInternalAndExternal(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/internal-model",
		RepoName:       "ns/internal-model",
		LegacyModelID:  "ns/internal-model:1",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc1",
		SourceDeployID: 1,
	}
	mockLLMConfigStore.EXPECT().IndexWithRepoForCaller(mock.Anything, 50, 1, mock.Anything, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "ns/internal-model:1",
				Enabled:   true,
				Upstreams: []database.Upstream{
					{
						ID:        1,
						Source:    commontypes.UpstreamSourceCSGHubDeploy,
						URL:       "http://internal/v1",
						Enabled:   true,
						ModelName: "ns/internal-model",
						Metadata: &commontypes.UpstreamMetadata{
							InternalModelInfo: internalInfo,
						},
					},
				},
			},
			{
				ID:        2,
				ModelName: "gpt-4",
				Enabled:   true,
				Provider:  "openai",
				Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				Upstreams: []database.Upstream{
					{
						ID:         2,
						Source:     commontypes.UpstreamSourceExternal,
						URL:        "http://openai/v1",
						Enabled:    true,
						Provider:   "openai",
						AuthHeader: "Bearer sk-xxx",
					},
				},
			},
		}, 2, nil).Once()

	// Owner calling: should see both
	models, err := comp.GetAvailableModels(context.Background(), &types.GetAvailableModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}})
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "ns/internal-model:1", models[0].ID)
	assert.Equal(t, "gpt-4", models[1].ID)
}

func TestOpenAIComponent_GetModelByID_ExternalModel(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "gpt-4", mock.Anything).
		Return(&database.LLMConfig{
			ID:         1,
			ModelName:  "gpt-4",
			Enabled:    true,
			Provider:   "openai",
			AuthHeader: "Bearer sk-xxx",
			Metadata:   map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
			Upstreams: []database.Upstream{
				{
					ID:         1,
					Source:     commontypes.UpstreamSourceExternal,
					URL:        "http://openai-api/v1",
					Enabled:    true,
					Provider:   "openai",
					AuthHeader: "Bearer sk-xxx",
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "user-uuid", UserUUID: "user-uuid"}, ModelID: "gpt-4"})
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "gpt-4", model.ID)
	assert.Equal(t, "openai", model.OwnedBy)
	assert.Equal(t, commontypes.ProviderTypeExternalLLM, model.Metadata[types.MetaKeyLLMType])
	assert.Equal(t, "openai", model.Provider)
	assert.Equal(t, "Bearer sk-xxx", model.AuthHead)
	assert.Len(t, model.Upstreams, 1)
}

func TestOpenAIComponent_GetModelByID_InternalModel(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/internal-model",
		RepoName:       "ns/internal-model",
		LegacyModelID:  "ns/internal-model:a",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc1",
		SourceDeployID: 10,
	}
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/internal-model:a", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/internal-model:a",
			Enabled:   true,
			Upstreams: []database.Upstream{
				{
					ID:        1,
					Source:    commontypes.UpstreamSourceCSGHubDeploy,
					URL:       "http://internal-endpoint/v1",
					Enabled:   true,
					ModelName: "ns/internal-model",
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: internalInfo,
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}, ModelID: "ns/internal-model:a"})
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "ns/internal-model:a", model.ID) // base36(10) = "a"
	assert.Equal(t, "owner", model.OwnedBy)
	assert.Equal(t, commontypes.ProviderTypeInference, model.Metadata[types.MetaKeyLLMType])
	assert.Equal(t, "ns/internal-model", model.Metadata[types.MetaKeyRepoPath])
	assert.Equal(t, "owner-uuid", model.OwnerUUID)
	assert.Equal(t, "svc1", model.SvcName)
}

func TestOpenAIComponent_GetModelByID_PrivateModel_OwnerCanAccess(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/private-model",
		RepoName:       "ns/private-model",
		LegacyModelID:  "ns/private-model:a",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc1",
		SourceDeployID: 10,
	}
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/private-model:a", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/private-model:a",
			Enabled:   true,
			Upstreams: []database.Upstream{
				{
					ID:        1,
					Source:    commontypes.UpstreamSourceCSGHubDeploy,
					URL:       "http://internal-endpoint/v1",
					Enabled:   true,
					ModelName: "ns/private-model",
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: internalInfo,
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}, ModelID: "ns/private-model:a"})
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "owner-uuid", model.OwnerUUID)
}

func TestOpenAIComponent_GetModelByID_PrivateModel_NonOwnerDenied(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/private-model",
		RepoName:       "ns/private-model",
		LegacyModelID:  "ns/private-model:a",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.InferenceType,
		SvcName:        "svc1",
		SourceDeployID: 10,
	}
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/private-model:a", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/private-model:a",
			Enabled:   true,
			Upstreams: []database.Upstream{
				{
					ID:        1,
					Source:    commontypes.UpstreamSourceCSGHubDeploy,
					URL:       "http://internal-endpoint/v1",
					Enabled:   true,
					ModelName: "ns/private-model",
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: internalInfo,
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "other-user-uuid", UserUUID: "other-user-uuid"}, ModelID: "ns/private-model:a"})
	require.NoError(t, err)
	assert.Nil(t, model)
}

func TestOpenAIComponent_GetModelByID_ServerlessModel_AnyUserCanAccess(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfo := &commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/serverless-model",
		HFPath:         "ns/serverless-model",
		LegacyModelID:  "ns/serverless-model",
		OwnerUUID:      "owner-uuid",
		OwnerUsername:  "owner",
		SvcType:        commontypes.ServerlessType,
		SvcName:        "svc1",
		SourceDeployID: 10,
	}
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/serverless-model", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/serverless-model",
			Enabled:   true,
			Upstreams: []database.Upstream{
				{
					ID:        1,
					Source:    commontypes.UpstreamSourceCSGHubDeploy,
					URL:       "http://serverless-endpoint/v1",
					Enabled:   true,
					ModelName: "ns/serverless-model",
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: internalInfo,
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "any-user-uuid", UserUUID: "any-user-uuid"}, ModelID: "ns/serverless-model"})
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "owner-uuid", model.OwnerUUID)
}

func TestOpenAIComponent_GetModelByID_NotFound(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "nonexistent", mock.Anything).
		Return(nil, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "user-uuid", UserUUID: "user-uuid"}, ModelID: "nonexistent"})
	require.NoError(t, err)
	assert.Nil(t, model)
}

func TestOpenAIComponent_GetModelByID_DBError(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "error-model", mock.Anything).
		Return(nil, errors.New("db error")).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "user-uuid", UserUUID: "user-uuid"}, ModelID: "error-model"})
	require.Error(t, err)
	assert.Nil(t, model)
}

func TestOpenAIComponent_GetModelByID_DisabledConfigReturnsNil(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	// llm_config exists but is disabled (admin hasn't enabled it yet).
	// GetModelByID should return nil — the model must not be accessible
	// until the llm_config is explicitly enabled, even though the upstream
	// itself is enabled.
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/disabled-model:a", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/disabled-model:a",
			Enabled:   false,
			Upstreams: []database.Upstream{
				{
					ID:      1,
					Source:  commontypes.UpstreamSourceCSGHubDeploy,
					URL:     "http://internal-endpoint/v1",
					Enabled: true,
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: &commontypes.InternalModelInfo{
							CSGHubModelID:  "ns/disabled-model",
							RepoName:       "ns/disabled-model",
							LegacyModelID:  "ns/disabled-model:a",
							OwnerUUID:      "owner-uuid",
							SvcType:        commontypes.InferenceType,
							SourceDeployID: 10,
						},
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}, ModelID: "ns/disabled-model:a"})
	require.NoError(t, err)
	assert.Nil(t, model, "disabled llm_config should not be accessible via GetModelByID")
}

func TestOpenAIComponent_GetModelByID_SetsSupportFunctionCallFromEngineArgs(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	internalInfoWithToolCall := &commontypes.InternalModelInfo{
		CSGHubModelID:    "ns/tool-model",
		RepoName:         "ns/tool-model",
		LegacyModelID:    "ns/tool-model:1",
		OwnerUUID:        "owner-uuid",
		OwnerUsername:    "owner",
		SvcType:          commontypes.InferenceType,
		SvcName:          "svc1",
		SourceDeployID:   1,
		RuntimeFramework: "vllm",
		EngineArgs:       `{"enable-tool-calling":"enable"}`,
	}
	mockLLMConfigStore.EXPECT().GetByModelNameForCaller(mock.Anything, "ns/tool-model:1", mock.Anything).
		Return(&database.LLMConfig{
			ID:        1,
			ModelName: "ns/tool-model:1",
			Enabled:   true,
			Upstreams: []database.Upstream{
				{
					ID:      1,
					Source:  commontypes.UpstreamSourceCSGHubDeploy,
					URL:     "http://tool-endpoint/v1",
					Enabled: true,
					Metadata: &commontypes.UpstreamMetadata{
						InternalModelInfo: internalInfoWithToolCall,
					},
				},
			},
		}, nil).Once()

	model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "owner-uuid", UserUUID: "owner-uuid"}, ModelID: "ns/tool-model:1"})
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.True(t, model.SupportFunctionCall)
	assert.Equal(t, "ns/tool-model:1", model.ID) // base36(1) = "1"
}

func TestOpenAIComponent_ListModels(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	// The unscoped listing is served by one pushdown query; an anonymous
	// caller carries no pinned user and (on CE) no price filter.
	mockLLMConfigStore.EXPECT().ListGatewayModels(mock.Anything, -1, 0, mock.MatchedBy(func(q *commontypes.GatewayModelListQuery) bool {
		return q.CallerUUID == "" && q.PinnedUserUUID == "" && !q.RequireConfiguredPrice
	})).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "gpt-4",
				Enabled:   true,
				Provider:  "openai",
				Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				Upstreams: []database.Upstream{
					{
						ID:         1,
						Source:     commontypes.UpstreamSourceExternal,
						URL:        "http://openai/v1",
						Enabled:    true,
						Provider:   "openai",
						AuthHeader: "Bearer sk-xxx",
					},
				},
			},
			{
				ID:        2,
				ModelName: "claude-3",
				Enabled:   true,
				Provider:  "anthropic",
				Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				Upstreams: []database.Upstream{
					{
						ID:         2,
						Source:     commontypes.UpstreamSourceExternal,
						URL:        "http://anthropic/v1",
						Enabled:    true,
						Provider:   "anthropic",
						AuthHeader: "Bearer sk-yyy",
					},
				},
			},
		}, 2, nil).Once()

	modelList, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}})
	require.NoError(t, err)
	require.Len(t, modelList.Data, 2)
	assert.Equal(t, "gpt-4", modelList.Data[0].ID)
	assert.Equal(t, "claude-3", modelList.Data[1].ID)
	assert.Equal(t, 2, modelList.TotalCount)
}

// The unscoped listing translates the request parameters into the pushdown
// query verbatim and forwards the pagination window.
func TestOpenAIComponent_ListModels_PushesRequestFiltersDown(t *testing.T) {
	mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
	comp := &openaiComponentImpl{
		extllmStore: mockLLMConfigStore,
	}

	hasAssociated := true
	// per=150 is clamped to 100; page=3 gives offset (3-1)*100. CE has no
	// pinned-model support: the listing carries no pinned user.
	mockLLMConfigStore.EXPECT().ListGatewayModels(mock.Anything, 100, 200, mock.MatchedBy(func(q *commontypes.GatewayModelListQuery) bool {
		return q.Keyword == "glm" &&
			len(q.LLMTypes) == 1 && q.LLMTypes[0] == "external_llm" &&
			q.Task == "text-generation" &&
			q.HasAssociatedModel != nil && *q.HasAssociatedModel == hasAssociated &&
			q.CallerUUID == "caller-uuid" && q.PinnedUserUUID == ""
	})).
		Return([]*database.LLMConfig{}, 0, nil).Once()

	_, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{NsUUID: "caller-uuid"},
		ModelID:            "glm",
		LLMTypes:           []string{"external_llm"},
		Task:               "text-generation",
		HasAssociatedModel: &hasAssociated,
		Per:                150,
		Page:               3,
	})
	require.NoError(t, err)
}
