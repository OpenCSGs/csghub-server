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
	mockbldmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"

	"github.com/openai/openai-go/v3"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"

	bldmq "opencsg.com/csghub-server/builder/mq"
	commontypes "opencsg.com/csghub-server/common/types"
)

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

func TestFilterAndPaginateModels(t *testing.T) {
	models := []types.Model{
		{BaseModel: types.BaseModel{ID: "gpt-4:svc1", Object: "model", OwnedBy: "u1"}},
		{BaseModel: types.BaseModel{ID: "gpt-3.5:svc2", Object: "model", OwnedBy: "u1"}},
		{BaseModel: types.BaseModel{ID: "claude:svc3", Object: "model", OwnedBy: "u2"}},
		{BaseModel: types.BaseModel{ID: "gpt-4o:svc4", Object: "model", OwnedBy: "u3"}},
	}

	t.Run("no filters default pagination", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{})
		assert.Equal(t, "list", resp.Object)
		assert.Equal(t, 4, resp.TotalCount)
		assert.Len(t, resp.Data, 4)
		assert.False(t, resp.HasMore)
		require.NotNil(t, resp.FirstID)
		require.NotNil(t, resp.LastID)
		assert.Equal(t, "gpt-4:svc1", *resp.FirstID)
		assert.Equal(t, "gpt-4o:svc4", *resp.LastID)
	})

	t.Run("fuzzy model_id filter is case-insensitive", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelID: "GPT"})
		assert.Equal(t, 3, resp.TotalCount)
		assert.Len(t, resp.Data, 3)
	})

	t.Run("pagination per/page applied after filters", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelID: "gpt", Per: "2", Page: "2"})
		// gpt matches 3 models; page=2 per=2 yields 1 item
		assert.Equal(t, 3, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "gpt-4o:svc4", resp.Data[0].ID)
		assert.False(t, resp.HasMore)
	})

	t.Run("source filter csghub", func(t *testing.T) {
		modelsWithSource := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-model:svc1", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "external-model", Object: "model", OwnedBy: "openai"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
			{BaseModel: types.BaseModel{ID: "csghub-model:svc2", Object: "model", OwnedBy: "u2"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "org/model2"}},
		}
		resp := filterAndPaginateModels(modelsWithSource, types.ListModelsReq{Source: string(types.ModelSourceCSGHub)})
		assert.Equal(t, 2, resp.TotalCount)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "csghub-model:svc1", resp.Data[0].ID)
		assert.Equal(t, "csghub-model:svc2", resp.Data[1].ID)
	})

	t.Run("source filter external", func(t *testing.T) {
		modelsWithSource := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-model:svc1", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "gpt-4", Object: "model", OwnedBy: "openai"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
			{BaseModel: types.BaseModel{ID: "claude", Object: "model", OwnedBy: "anthropic"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "anthropic"}},
		}
		resp := filterAndPaginateModels(modelsWithSource, types.ListModelsReq{Source: string(types.ModelSourceExternal)})
		assert.Equal(t, 2, resp.TotalCount)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "gpt-4", resp.Data[0].ID)
		assert.Equal(t, "claude", resp.Data[1].ID)
	})

	t.Run("source filter is case-insensitive", func(t *testing.T) {
		modelsWithSource := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-model:svc1", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "gpt-4", Object: "model", OwnedBy: "openai"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
		}
		resp := filterAndPaginateModels(modelsWithSource, types.ListModelsReq{Source: "CSGHub"})
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "csghub-model:svc1", resp.Data[0].ID)
	})

	t.Run("unknown source filter includes all", func(t *testing.T) {
		modelsWithSource := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-model:svc1", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "gpt-4", Object: "model", OwnedBy: "openai"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
		}
		resp := filterAndPaginateModels(modelsWithSource, types.ListModelsReq{Source: "unknown"})
		assert.Equal(t, 2, resp.TotalCount)
		assert.Len(t, resp.Data, 2)
	})

	t.Run("source filter csghub includes public and private deployments", func(t *testing.T) {
		modelsWithSource := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-public", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "csghub-private", Object: "model", OwnedBy: "u1"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model2"}},
			{BaseModel: types.BaseModel{ID: "external-public", Object: "model", OwnedBy: "openai"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
		}
		resp := filterAndPaginateModels(modelsWithSource, types.ListModelsReq{Source: string(types.ModelSourceCSGHub)})
		assert.Equal(t, 2, resp.TotalCount)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "csghub-public", resp.Data[0].ID)
		assert.Equal(t, "csghub-private", resp.Data[1].ID)
	})

	t.Run("task filter text-generation", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "model-1", Object: "model", OwnedBy: "u1", Task: "text-generation"}},
			{BaseModel: types.BaseModel{ID: "model-2", Object: "model", OwnedBy: "u1", Task: "text-to-image"}},
			{BaseModel: types.BaseModel{ID: "model-3", Object: "model", OwnedBy: "u2", Task: "text-generation"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-generation"})
		assert.Equal(t, 2, resp.TotalCount)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "model-1", resp.Data[0].ID)
		assert.Equal(t, "model-3", resp.Data[1].ID)
	})

	t.Run("task filter text-to-image", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "model-1", Object: "model", OwnedBy: "u1", Task: "text-generation"}},
			{BaseModel: types.BaseModel{ID: "model-2", Object: "model", OwnedBy: "u1", Task: "text-to-image"}},
			{BaseModel: types.BaseModel{ID: "model-3", Object: "model", OwnedBy: "u2", Task: "text-generation"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-to-image"})
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "model-2", resp.Data[0].ID)
	})

	t.Run("task filter is case-insensitive", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "model-1", Object: "model", OwnedBy: "u1", Task: "Text-Generation"}},
			{BaseModel: types.BaseModel{ID: "model-2", Object: "model", OwnedBy: "u1", Task: "TEXT-TO-IMAGE"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-generation"})
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "model-1", resp.Data[0].ID)
	})

	t.Run("task filter with no matches", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "model-1", Object: "model", OwnedBy: "u1", Task: "text-generation"}},
			{BaseModel: types.BaseModel{ID: "model-2", Object: "model", OwnedBy: "u1", Task: "text-to-image"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "non-existent-task"})
		assert.Equal(t, 0, resp.TotalCount)
		assert.Len(t, resp.Data, 0)
	})

	t.Run("task filter matches any comma-separated task on model", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "multi", Object: "model", OwnedBy: "u1", Task: "text-generation,text-to-image,summarization"}},
			{BaseModel: types.BaseModel{ID: "single", Object: "model", OwnedBy: "u1", Task: "text-to-image"}},
			{BaseModel: types.BaseModel{ID: "other", Object: "model", OwnedBy: "u1", Task: "embedding"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-generation"})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "multi", resp.Data[0].ID)

		resp = filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "summarization"})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "multi", resp.Data[0].ID)
	})

	t.Run("task filter collapses repeated commas in model Task", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "sparse", Object: "model", OwnedBy: "u1", Task: "text-generation,,text-to-image"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-generation"})
		assert.Equal(t, 1, resp.TotalCount)
		resp = filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "text-to-image"})
		assert.Equal(t, 1, resp.TotalCount)
	})

	t.Run("task filter combined with source filter", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "csghub-gen", Object: "model", OwnedBy: "u1", Task: "text-generation"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model1"}},
			{BaseModel: types.BaseModel{ID: "csghub-image", Object: "model", OwnedBy: "u1", Task: "text-to-image"}, InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "user/model2"}},
			{BaseModel: types.BaseModel{ID: "external-gen", Object: "model", OwnedBy: "openai", Task: "text-generation"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Source: string(types.ModelSourceCSGHub), Task: "text-generation"})
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "csghub-gen", resp.Data[0].ID)
	})
}

func TestOpenAIComponentImpl_getCSGHubModels_SkipsDeploysWithMissingRelations(t *testing.T) {
	mockDeployStore := mockdb.NewMockDeployTaskStore(t)
	comp := &openaiComponentImpl{
		deployStore: mockDeployStore,
		modelIDFmt:  "%s(%s)",
	}

	now := time.Now()
	deploys := []database.Deploy{
		{
			ID:      1,
			SvcName: "missing-repo",
			Type:    commontypes.InferenceType,
			User: &database.User{
				Username: "owner",
				UUID:     "owner-uuid",
			},
		},
		{
			ID:      2,
			SvcName: "missing-user",
			Type:    commontypes.InferenceType,
			UserID:  2,
			Repository: &database.Repository{
				Name: "model-without-user",
				Path: "namespace/model-without-user",
			},
		},
		{
			ID:      3,
			SvcName: "valid-svc",
			Type:    commontypes.InferenceType,
			Repository: &database.Repository{
				Name: "valid-model",
				Path: "namespace/valid-model",
			},
			User: &database.User{
				Username: "valid-owner",
				UUID:     "valid-owner-uuid",
			},
			Endpoint: "valid-endpoint",
		},
	}
	for i := range deploys {
		deploys[i].CreatedAt = now
	}

	mockDeployStore.EXPECT().RunningVisibleToUser(mock.Anything, int64(1)).
		Return(deploys, nil).Once()

	models, err := comp.getCSGHubModels(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, models, 1)
	assert.Equal(t, "namespace/valid-model:valid-svc", models[0].ID)
	assert.Equal(t, "valid-owner", models[0].OwnedBy)
	assert.Equal(t, "valid-owner-uuid", models[0].OwnerUUID)
	assert.Equal(t, "valid-endpoint", models[0].Endpoint)
}

func TestOpenAIComponentImpl_applyFormatModelIDToModelList(t *testing.T) {
	comp := &openaiComponentImpl{}

	t.Run("nil model list", func(t *testing.T) {
		comp.applyFormatModelIDToModelList(nil)
	})

	t.Run("empty model list should keep first and last nil", func(t *testing.T) {
		modelList := types.ModelList{
			Object:     "list",
			Data:       []types.Model{},
			FirstID:    nil,
			LastID:     nil,
			HasMore:    false,
			TotalCount: 0,
		}

		comp.applyFormatModelIDToModelList(&modelList)

		assert.Nil(t, modelList.FirstID)
		assert.Nil(t, modelList.LastID)
		assert.Len(t, modelList.Data, 0)
	})

	t.Run("should override id with format model id and recalculate first and last", func(t *testing.T) {
		modelList := types.ModelList{
			Object: "list",
			Data: []types.Model{
				{
					BaseModel: types.BaseModel{
						ID: "gpt-4o",
					},
					ExternalModelInfo: types.ExternalModelInfo{
						FormatModelID: "gpt-4o(openai)",
					},
				},
				{
					BaseModel: types.BaseModel{
						ID: "llama3",
					},
				},
				{
					BaseModel: types.BaseModel{
						ID: "claude-3",
					},
					ExternalModelInfo: types.ExternalModelInfo{
						FormatModelID: "claude-3(anthropic)",
					},
				},
			},
			HasMore:    false,
			TotalCount: 3,
		}

		comp.applyFormatModelIDToModelList(&modelList)

		require.Len(t, modelList.Data, 3)
		assert.Equal(t, "gpt-4o(openai)", modelList.Data[0].ID)
		assert.Equal(t, "llama3", modelList.Data[1].ID)
		assert.Equal(t, "claude-3(anthropic)", modelList.Data[2].ID)

		require.NotNil(t, modelList.FirstID)
		require.NotNil(t, modelList.LastID)
		assert.Equal(t, "gpt-4o(openai)", *modelList.FirstID)
		assert.Equal(t, "claude-3(anthropic)", *modelList.LastID)
	})
}

func TestOpenAIComponent_checkOrganization(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)

	comp := &openaiComponentImpl{
		userStore:  mockUserStore,
		organStore: mockOrgStore,
	}

	t.Run("users belong to same organization - should return true", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "owner-uuid-456"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   2,
			UUID: ownerUUID,
		}

		org1 := database.Organization{
			ID:   100,
			Name: "org1",
		}
		org2 := database.Organization{
			ID:   200,
			Name: "org2",
		}

		userOrgs := []database.Organization{org1, org2}
		ownerOrgs := []database.Organization{org2, {ID: 300, Name: "org3"}}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(userOrgs, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, owner.ID).Return(ownerOrgs, nil).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.NoError(t, err)
		assert.True(t, result, "Users should belong to same organization")
	})

	t.Run("users do not belong to same organization - should return false", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "owner-uuid-456"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   2,
			UUID: ownerUUID,
		}

		userOrgs := []database.Organization{
			{ID: 100, Name: "org1"},
			{ID: 200, Name: "org2"},
		}
		ownerOrgs := []database.Organization{
			{ID: 300, Name: "org3"},
			{ID: 400, Name: "org4"},
		}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(userOrgs, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, owner.ID).Return(ownerOrgs, nil).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.NoError(t, err)
		assert.False(t, result, "Users should not belong to same organization")
	})

	t.Run("user has no organizations - should return false", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "owner-uuid-456"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   2,
			UUID: ownerUUID,
		}

		userOrgs := []database.Organization{}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(userOrgs, nil).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.NoError(t, err)
		assert.False(t, result, "User with no organizations should not have access")
	})

	t.Run("owner has no organizations - should return false", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "owner-uuid-456"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   2,
			UUID: ownerUUID,
		}

		userOrgs := []database.Organization{
			{ID: 100, Name: "org1"},
		}
		ownerOrgs := []database.Organization{}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(userOrgs, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, owner.ID).Return(ownerOrgs, nil).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.NoError(t, err)
		assert.False(t, result, "Owner with no organizations should not grant access")
	})

	t.Run("user not found - should return false without error", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "nonexistent-user"
		ownerUUID := "owner-uuid-456"

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(&database.User{}, errors.New("user not found")).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.Error(t, err)
		assert.False(t, result, "Should return false when user is not found")
	})

	t.Run("owner not found - should return false without error", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "nonexistent-owner"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(&database.User{}, errors.New("owner not found")).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.Error(t, err)
		assert.False(t, result, "Should return false when owner is not found")
	})

	t.Run("error getting user organizations - should return false without error", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-123"
		ownerUUID := "owner-uuid-456"

		user := &database.User{
			ID:   1,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   2,
			UUID: ownerUUID,
		}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(nil, errors.New("database error")).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.Error(t, err)
		assert.False(t, result, "Should return false when there's an error getting user organizations")
	})

	t.Run("error getting owner organizations - should return false without error", func(t *testing.T) {
		ctx := context.Background()
		userUUID := "user-uuid-666"
		ownerUUID := "owner-uuid-777"

		user := &database.User{
			ID:   66,
			UUID: userUUID,
		}
		owner := &database.User{
			ID:   77,
			UUID: ownerUUID,
		}

		userOrgs := []database.Organization{
			{ID: 100, Name: "org1"},
		}

		mockUserStore.EXPECT().FindByUUID(ctx, userUUID).Return(user, nil).Once()
		mockUserStore.EXPECT().FindByUUID(ctx, ownerUUID).Return(owner, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, user.ID).Return(userOrgs, nil).Once()
		mockOrgStore.EXPECT().GetUserBelongOrgs(ctx, owner.ID).Return(nil, errors.New("database error")).Once()

		result, err := comp.checkOrganization(ctx, userUUID, ownerUUID)

		assert.Error(t, err)
		assert.False(t, result, "Should return false when there's an error getting owner organizations")
	})
}

func TestOpenAIComponentImpl_RecordUsage(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}
	mockOrgStore := &mockdb.MockOrgStore{}

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
			name:     "successful record - dedicated inference by other user but not same organ",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeInference,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       commontypes.InferenceType,
					OwnerUUID:     "another-user-uuid",
				},
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: false,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
					organStore:  mockOrgStore,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)
				user := &database.User{
					ID:       1,
					Username: "testuser",
				}
				owner := &database.User{
					ID:       2,
					Username: "owneruser",
				}
				mockUserStore.EXPECT().FindByUUID(mock.Anything, "test-user-uuid").Return(user, nil).Once()
				mockUserStore.EXPECT().FindByUUID(mock.Anything, "another-user-uuid").Return(owner, nil).Once()
				mockOrgStore.EXPECT().GetUserBelongOrgs(mock.Anything, user.ID).Return([]database.Organization{}, nil).Once()
				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "csghub://inference/test-model", evt.ResourceID)
					require.Equal(t, "csghub://inference/test-model", evt.ResourceName)
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
			name:     "successful record - dedicated inference deployed by same user",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeInference,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       commontypes.InferenceType,
					OwnerUUID:     "test-user-uuid",
				},
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: false,
			setupMock: func() {

				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "csghub://inference/test-model", evt.ResourceID)
					require.Equal(t, "csghub://inference/test-model", evt.ResourceName)
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
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeServerless,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       commontypes.ServerlessType,
				},
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: false,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "csghub://serverless/test-model", evt.ResourceID)
					require.Equal(t, "csghub://serverless/test-model", evt.ResourceName)
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
			name:      "nil model",
			userUUID:  "test-user-uuid",
			model:     nil,
			wantError: true,
			setupMock: func() {
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub: &event.EventPublisher{
						SyncInterval: 1,
						MQ:           mockbldmq.NewMockMessageQueue(t),
					},
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
				}, nil)
			},
		},
		{
			name:     "conflicting csghub id and external provider",
			userUUID: "test-user-uuid",
			model: &types.Model{
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "m",
				},
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: "openai",
				},
			},
			wantError: true,
			setupMock: func() {
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub: &event.EventPublisher{
						SyncInterval: 1,
						MQ:           mockbldmq.NewMockMessageQueue(t),
					},
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
				}, nil)
			},
		},
		{
			name:     "missing csghub id and provider",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{ID: "orphan"},
			},
			wantError: true,
			setupMock: func() {
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub: &event.EventPublisher{
						SyncInterval: 1,
						MQ:           mockbldmq.NewMockMessageQueue(t),
					},
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
				}, nil)
			},
		},
		{
			name:     "csghub model with invalid service type",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeInference,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       2,
				},
			},
			wantError: true,
			setupMock: func() {
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub: &event.EventPublisher{
						SyncInterval: 1,
						MQ:           mockbldmq.NewMockMessageQueue(t),
					},
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2,
				}, nil)
			},
		},
		{
			name:     "counter error",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeInference,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       commontypes.InferenceType,
				},
			},
			wantError: true,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(nil, errors.New("counter error"))
			},
		},
		{
			name:     "publish error",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					Metadata: map[string]any{
						types.MetaKeyLLMType: types.ProviderTypeInference,
					},
				},
				InternalModelInfo: types.InternalModelInfo{
					CSGHubModelID: "test-model",
					SvcName:       "test-service",
					SvcType:       commontypes.InferenceType,
					OwnerUUID:     "test-user-uuid",
				},
			},
			usage: &openai.CompletionUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			wantError: true,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
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

func TestOpenAIComponentImpl_RecordUsage_ExternalModel(t *testing.T) {
	mockUserStore := &mockdb.MockUserStore{}
	mockDeployStore := &mockdb.MockDeployTaskStore{}

	var mockCounter *mocktoken.MockCounter
	var comp *openaiComponentImpl

	tests := []struct {
		name      string
		userUUID  string
		model     *types.Model
		wantError bool
		setupMock func()
	}{
		{
			name:     "successful record - external model with OpenAI provider",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					ID:      "gpt-4(openai)",
					OwnedBy: "openai",
				},
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: "openai",
				},
			},
			wantError: false,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     200,
					CompletionTokens: 100,
					TotalTokens:      300,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "openai://gpt-4(openai)", evt.ResourceID)
					require.Equal(t, "openai://gpt-4(openai)", evt.ResourceName)
					require.Equal(t, "gpt-4(openai)", evt.CustomerID)
					require.Equal(t, "test-user-uuid", evt.UserUUID)
					require.Equal(t, commontypes.TokenNumberType, evt.ValueType)
					require.Equal(t, int64(300), evt.Value)
					require.Equal(t, int(commontypes.SceneModelServerless), evt.Scene)

					var tokenUsageExtra struct {
						PromptTokenNum     string                     `json:"prompt_token_num"`
						CompletionTokenNum string                     `json:"completion_token_num"`
						OwnerType          commontypes.TokenUsageType `json:"owner_type"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, "200", tokenUsageExtra.PromptTokenNum)
					require.Equal(t, "100", tokenUsageExtra.CompletionTokenNum)
					require.Equal(t, commontypes.ExternalInference, tokenUsageExtra.OwnerType)
					return nil
				})
			},
		},
		{
			name:     "counter error for external model",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					ID:      "gpt-3.5-turbo",
					OwnedBy: "openai",
				},
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: "openai",
				},
			},
			wantError: true,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(nil, errors.New("counter error"))
			},
		},
		{
			name:     "publish error for external model",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					ID:      "gemini-pro",
					OwnedBy: "google",
				},
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: "google",
				},
			},
			wantError: true,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)
				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)
				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     50,
					CompletionTokens: 25,
					TotalTokens:      75,
				}, nil)
				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).Return(errors.New("publish error")).Times(3)
			},
		},
		{
			name:     "external model with zero tokens",
			userUUID: "test-user-uuid",
			model: &types.Model{
				BaseModel: types.BaseModel{
					ID:      "test-model(test-provider)",
					OwnedBy: "test-provider",
				},
				ExternalModelInfo: types.ExternalModelInfo{
					Provider: "test-provider",
				},
			},
			wantError: false,
			setupMock: func() {
				mockBLDMQ := mockbldmq.NewMockMessageQueue(t)

				eventPub := &event.EventPublisher{
					SyncInterval: 1,
					MQ:           mockBLDMQ,
				}
				mockCounter = mocktoken.NewMockCounter(t)

				comp = &openaiComponentImpl{
					userStore:   mockUserStore,
					deployStore: mockDeployStore,
					eventPub:    eventPub,
				}
				mockCounter.EXPECT().Usage(mock.Anything).Return(&token.Usage{
					PromptTokens:     0,
					CompletionTokens: 0,
					TotalTokens:      0,
				}, nil)

				mockBLDMQ.EXPECT().Publish(bldmq.MeterDurationSendSubject, mock.Anything).RunAndReturn(func(topic string, data []byte) error {
					var evt commontypes.MeteringEvent
					err := json.Unmarshal(data, &evt)
					require.NoError(t, err)
					require.Equal(t, "test-provider://test-model(test-provider)", evt.ResourceID)
					require.Equal(t, "test-provider://test-model(test-provider)", evt.ResourceName)
					require.Equal(t, "test-model(test-provider)", evt.CustomerID)
					require.Equal(t, int64(0), evt.Value)

					var tokenUsageExtra struct {
						PromptTokenNum     string                     `json:"prompt_token_num"`
						CompletionTokenNum string                     `json:"completion_token_num"`
						OwnerType          commontypes.TokenUsageType `json:"owner_type"`
					}
					err = json.Unmarshal([]byte(evt.Extra), &tokenUsageExtra)
					require.NoError(t, err)
					require.Equal(t, "0", tokenUsageExtra.PromptTokenNum)
					require.Equal(t, "0", tokenUsageExtra.CompletionTokenNum)
					require.Equal(t, commontypes.ExternalInference, tokenUsageExtra.OwnerType)
					return nil
				})
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
