package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"

	"opencsg.com/csghub-server/builder/store/database"

	commontypes "opencsg.com/csghub-server/common/types"
)

func TestFilterAndPaginateModels(t *testing.T) {
	models := []types.Model{
		{BaseModel: types.BaseModel{ID: "gpt-4:svc1", Object: "model", OwnedBy: "u1"}},
		{BaseModel: types.BaseModel{ID: "gpt-3.5:svc2", Object: "model", OwnedBy: "u1"}},
		{BaseModel: types.BaseModel{ID: "claude:svc3", Object: "model", OwnedBy: "u2"}},
		{BaseModel: types.BaseModel{ID: "gpt-4o:svc4", Object: "model", OwnedBy: "u3"}},
	}

	t.Run("no filters returns all models without pagination params", func(t *testing.T) {
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
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelID: "gpt", Per: 2, Page: 2})
		// gpt matches 3 models; page=2 per=2 yields 1 item
		assert.Equal(t, 3, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "gpt-4o:svc4", resp.Data[0].ID)
		assert.False(t, resp.HasMore)
	})

	t.Run("pagination has more when more filtered models remain", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelID: "gpt", Per: 2, Page: 1})
		assert.Equal(t, 3, resp.TotalCount)
		require.Len(t, resp.Data, 2)
		assert.Equal(t, "gpt-4:svc1", resp.Data[0].ID)
		assert.Equal(t, "gpt-3.5:svc2", resp.Data[1].ID)
		assert.True(t, resp.HasMore)
	})

	t.Run("per is capped at 100 when pagination is requested", func(t *testing.T) {
		manyModels := make([]types.Model, 105)
		for i := range manyModels {
			manyModels[i] = types.Model{BaseModel: types.BaseModel{ID: fmt.Sprintf("model-%03d", i), Object: "model", OwnedBy: "u1"}}
		}

		resp := filterAndPaginateModels(manyModels, types.ListModelsReq{Per: 101, Page: 1})
		assert.Equal(t, 105, resp.TotalCount)
		assert.Len(t, resp.Data, 100)
		assert.True(t, resp.HasMore)
	})

	t.Run("llm_types filter external_llm", func(t *testing.T) {
		modelsWithLLMTypes := []types.Model{
			{BaseModel: types.BaseModel{ID: "inference-model", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference}}},
			{BaseModel: types.BaseModel{ID: "external-model", Object: "model", OwnedBy: "openai", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyPricingConfigured: true}}},
			{BaseModel: types.BaseModel{ID: "serverless-model", Object: "model", OwnedBy: "u2", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeServerless, types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithLLMTypes, types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeExternalLLM}})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "external-model", resp.Data[0].ID)
	})

	t.Run("llm_types filter supports multiple values", func(t *testing.T) {
		modelsWithLLMTypes := []types.Model{
			{BaseModel: types.BaseModel{ID: "inference-model", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference}}},
			{BaseModel: types.BaseModel{ID: "external-model", Object: "model", OwnedBy: "openai", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyPricingConfigured: true}}},
			{BaseModel: types.BaseModel{ID: "serverless-model", Object: "model", OwnedBy: "u2", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeServerless, types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithLLMTypes, types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeServerless, commontypes.ProviderTypeInference}})
		assert.Equal(t, 2, resp.TotalCount)
		require.Len(t, resp.Data, 2)
		assert.Equal(t, "inference-model", resp.Data[0].ID)
		assert.Equal(t, "serverless-model", resp.Data[1].ID)
	})

	t.Run("llm_types filter is case-insensitive and trims spaces", func(t *testing.T) {
		modelsWithLLMTypes := []types.Model{
			{BaseModel: types.BaseModel{ID: "serverless-model", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeServerless, types.MetaKeyPricingConfigured: true}}},
			{BaseModel: types.BaseModel{ID: "external-model", Object: "model", OwnedBy: "openai", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithLLMTypes, types.ListModelsReq{LLMTypes: []string{" SERVERLESS "}})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "serverless-model", resp.Data[0].ID)
	})

	t.Run("llm_types filter excludes models without llm_type", func(t *testing.T) {
		modelsWithLLMTypes := []types.Model{
			{BaseModel: types.BaseModel{ID: "missing-llm-type", Object: "model", OwnedBy: "u1"}},
			{BaseModel: types.BaseModel{ID: "external-model", Object: "model", OwnedBy: "openai", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithLLMTypes, types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeExternalLLM}})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "external-model", resp.Data[0].ID)
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

	t.Run("task filter image-to-image", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "model-1", Object: "model", OwnedBy: "u1", Task: "text-to-image"}},
			{BaseModel: types.BaseModel{ID: "model-2", Object: "model", OwnedBy: "u1", Task: "image-to-image"}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{Task: "image-to-image"})
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

	t.Run("task filter combined with llm_types filter", func(t *testing.T) {
		modelsWithTask := []types.Model{
			{BaseModel: types.BaseModel{ID: "inference-gen", Object: "model", OwnedBy: "u1", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference}}},
			{BaseModel: types.BaseModel{ID: "inference-image", Object: "model", OwnedBy: "u1", Task: "text-to-image", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference}}},
			{BaseModel: types.BaseModel{ID: "external-gen", Object: "model", OwnedBy: "openai", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithTask, types.ListModelsReq{LLMTypes: []string{commontypes.ProviderTypeInference}, Task: "text-generation"})
		assert.Equal(t, 1, resp.TotalCount)
		assert.Len(t, resp.Data, 1)
		assert.Equal(t, "inference-gen", resp.Data[0].ID)
	})

	t.Run("has_associated_model true keeps models with non-empty repo_path", func(t *testing.T) {
		hasAssociatedModel := true
		modelsWithRepoPath := []types.Model{
			{BaseModel: types.BaseModel{ID: "associated", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: "ns/model"}}},
			{BaseModel: types.BaseModel{ID: "blank", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: " "}}},
			{BaseModel: types.BaseModel{ID: "missing", Object: "model", OwnedBy: "u1", Metadata: map[string]any{}}},
			{BaseModel: types.BaseModel{ID: "nil-metadata", Object: "model", OwnedBy: "u1"}},
			{BaseModel: types.BaseModel{ID: "non-string", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: 123}}},
		}
		resp := filterAndPaginateModels(modelsWithRepoPath, types.ListModelsReq{HasAssociatedModel: &hasAssociatedModel})
		assert.Equal(t, 1, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "associated", resp.Data[0].ID)
	})

	t.Run("has_associated_model false keeps models without non-empty repo_path", func(t *testing.T) {
		hasAssociatedModel := false
		modelsWithRepoPath := []types.Model{
			{BaseModel: types.BaseModel{ID: "associated", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: "ns/model"}}},
			{BaseModel: types.BaseModel{ID: "empty", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: ""}}},
			{BaseModel: types.BaseModel{ID: "missing", Object: "model", OwnedBy: "u1", Metadata: map[string]any{}}},
			{BaseModel: types.BaseModel{ID: "nil-metadata", Object: "model", OwnedBy: "u1"}},
			{BaseModel: types.BaseModel{ID: "non-string", Object: "model", OwnedBy: "u1", Metadata: map[string]any{types.MetaKeyRepoPath: 123}}},
		}
		resp := filterAndPaginateModels(modelsWithRepoPath, types.ListModelsReq{HasAssociatedModel: &hasAssociatedModel})
		assert.Equal(t, 4, resp.TotalCount)
		require.Len(t, resp.Data, 4)
		assert.Equal(t, "empty", resp.Data[0].ID)
		assert.Equal(t, "missing", resp.Data[1].ID)
		assert.Equal(t, "nil-metadata", resp.Data[2].ID)
		assert.Equal(t, "non-string", resp.Data[3].ID)
	})

	t.Run("has_associated_model combines with existing filters and pagination", func(t *testing.T) {
		hasAssociatedModel := true
		modelsWithRepoPath := []types.Model{
			{BaseModel: types.BaseModel{ID: "alpha-gen", Object: "model", OwnedBy: "u1", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference, types.MetaKeyRepoPath: "ns/alpha"}}},
			{BaseModel: types.BaseModel{ID: "beta-gen", Object: "model", OwnedBy: "u1", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference, types.MetaKeyRepoPath: "ns/beta"}}},
			{BaseModel: types.BaseModel{ID: "gamma-gen", Object: "model", OwnedBy: "u1", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference}}},
			{BaseModel: types.BaseModel{ID: "delta-image", Object: "model", OwnedBy: "u1", Task: "text-to-image", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeInference, types.MetaKeyRepoPath: "ns/delta"}}},
			{BaseModel: types.BaseModel{ID: "external-gen", Object: "model", OwnedBy: "openai", Task: "text-generation", Metadata: map[string]any{types.MetaKeyLLMType: commontypes.ProviderTypeExternalLLM, types.MetaKeyRepoPath: "ns/external", types.MetaKeyPricingConfigured: true}}},
		}
		resp := filterAndPaginateModels(modelsWithRepoPath, types.ListModelsReq{
			ModelID:            "gen",
			LLMTypes:           []string{commontypes.ProviderTypeInference},
			Task:               "text-generation",
			HasAssociatedModel: &hasAssociatedModel,
			Per:                1,
			Page:               2,
		})
		assert.Equal(t, 2, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "beta-gen", resp.Data[0].ID)
		assert.False(t, resp.HasMore)
	})

	t.Run("model scope keeps only allowed ids and feeds pagination totals", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelScope: []string{"gpt-3.5:svc2", "gpt-4o:svc4"}})
		assert.Equal(t, 2, resp.TotalCount)
		require.Len(t, resp.Data, 2)
		assert.Equal(t, "gpt-3.5:svc2", resp.Data[0].ID)
		assert.Equal(t, "gpt-4o:svc4", resp.Data[1].ID)
	})

	t.Run("model scope combines with pagination", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelScope: []string{"gpt-4:svc1", "gpt-3.5:svc2"}, Per: 1, Page: 2})
		assert.Equal(t, 2, resp.TotalCount)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, "gpt-3.5:svc2", resp.Data[0].ID)
		assert.False(t, resp.HasMore)
	})

	t.Run("empty model scope is unrestricted", func(t *testing.T) {
		resp := filterAndPaginateModels(models, types.ListModelsReq{ModelScope: nil})
		assert.Equal(t, 4, resp.TotalCount)
	})
}

func TestSanitizeMeteringEventForLog(t *testing.T) {
	event := commontypes.MeteringEvent{
		Extra: `{"prompt_token_num":"12","completion_token_num":"8","owner_type":"x","api_key":"sk-test-secret"}`,
	}

	sanitized := sanitizeMeteringEventForLog(event)
	require.NotEqual(t, event.Extra, sanitized.Extra)
	require.Contains(t, sanitized.Extra, `"api_key":""`)
	require.NotContains(t, sanitized.Extra, "sk-test-secret")
	require.True(t, strings.Contains(sanitized.Extra, `"prompt_token_num":"12"`))
	require.True(t, strings.Contains(sanitized.Extra, `"completion_token_num":"8"`))
}

// TestOpenAIComponent_CanManageModel checks the expected user query options.
func TestOpenAIComponent_CanManageModel(t *testing.T) {
	ctx := context.Background()
	model := &types.Model{
		InternalModelInfo: types.InternalModelInfo{
			OwnerUUID: "owner-uuid",
		},
	}

	t.Run("admin can manage any model", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		comp := &openaiComponentImpl{userStore: mockUserStore}
		mockUserStore.EXPECT().FindByUsername(ctx, "adminuser", commontypes.UserWithAdminPermissions).Return(database.User{UUID: "admin-uuid", RoleAssignments: []database.UserRole{{RoleCode: "owner", Role: &database.Role{IsAdmin: true,
			RoleCode: "owner", Permissions: []database.AdminRolePermission{{PermissionCode: commontypes.AdminPermissionAll}},
		}}}}, nil).Once()

		allowed, err := comp.CanManageModel(ctx, "adminuser", "admin-uuid", model)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("deploy owner can manage", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		comp := &openaiComponentImpl{userStore: mockUserStore}
		mockUserStore.EXPECT().FindByUsername(ctx, "owner", commontypes.UserWithAdminPermissions).Return(database.User{UUID: "owner-uuid"}, nil).Once()

		allowed, err := comp.CanManageModel(ctx, "owner", "owner-uuid", model)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("other user is denied", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		comp := &openaiComponentImpl{userStore: mockUserStore}
		mockUserStore.EXPECT().FindByUsername(ctx, "other", commontypes.UserWithAdminPermissions).Return(database.User{UUID: "other-uuid"}, nil).Once()

		allowed, err := comp.CanManageModel(ctx, "other", "other-uuid", model)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("external model without owner denies non-admin", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		comp := &openaiComponentImpl{userStore: mockUserStore}
		mockUserStore.EXPECT().FindByUsername(ctx, "someone", commontypes.UserWithAdminPermissions).Return(database.User{UUID: "someone-uuid"}, nil).Once()

		allowed, err := comp.CanManageModel(ctx, "someone", "someone-uuid", &types.Model{})
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("nil model returns error instead of permission decision", func(t *testing.T) {
		comp := &openaiComponentImpl{}

		allowed, err := comp.CanManageModel(ctx, "someone", "someone-uuid", nil)
		require.Error(t, err)
		assert.False(t, allowed)
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

func TestBuildUsageExtraDataIncludesCachedPromptTokens(t *testing.T) {
	meteringInfo := usageMeteringInfo{OwnerType: commontypes.ExternalInference}

	for _, tt := range []struct {
		name               string
		cachedPromptTokens int64
		wantCachedTokens   string
	}{
		{
			name:               "cached prompt tokens reported",
			cachedPromptTokens: 40,
			wantCachedTokens:   "40",
		},
		{
			name:             "cached prompt tokens absent",
			wantCachedTokens: "0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			extraData, err := buildUsageExtraData(&types.Model{}, "target-model", &token.Usage{
				PromptTokens:       100,
				CachedPromptTokens: tt.cachedPromptTokens,
				CompletionTokens:   50,
			}, 0, meteringInfo)
			require.NoError(t, err)

			var extra usageMeteringExtra
			require.NoError(t, json.Unmarshal([]byte(extraData), &extra))
			require.Equal(t, "100", extra.PromptTokenNum)
			require.Equal(t, tt.wantCachedTokens, extra.PromptTokenCacheNum)
			require.Equal(t, "50", extra.CompletionTokenNum)
		})
	}
}

func TestBuildUsageExtraDataUpstreamID(t *testing.T) {
	meteringInfo := usageMeteringInfo{OwnerType: commontypes.ExternalInference}
	usage := &token.Usage{PromptTokens: 100, CompletionTokens: 50}

	t.Run("upstream id reported", func(t *testing.T) {
		extraData, err := buildUsageExtraData(&types.Model{UpstreamID: 42}, "target-model", usage, 0, meteringInfo)
		require.NoError(t, err)
		var extra usageMeteringExtra
		require.NoError(t, json.Unmarshal([]byte(extraData), &extra))
		require.Equal(t, "42", extra.UpstreamID)
	})

	t.Run("zero upstream id omitted", func(t *testing.T) {
		extraData, err := buildUsageExtraData(&types.Model{}, "target-model", usage, 0, meteringInfo)
		require.NoError(t, err)
		require.NotContains(t, extraData, "upstream_id")
	})

}

func TestBuildUsageExtraDataIncludesReasoningTokens(t *testing.T) {
	meteringInfo := usageMeteringInfo{OwnerType: commontypes.ExternalInference}

	for _, tt := range []struct {
		name             string
		reasoningTokens  int64
		wantReasoningNum string
	}{
		{
			name:             "reasoning tokens reported",
			reasoningTokens:  86,
			wantReasoningNum: "86",
		},
		{
			name:             "reasoning tokens absent",
			wantReasoningNum: "0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			extraData, err := buildUsageExtraData(&types.Model{}, "target-model", &token.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				ReasoningTokens:  tt.reasoningTokens,
			}, 0, meteringInfo)
			require.NoError(t, err)

			var extra usageMeteringExtra
			require.NoError(t, json.Unmarshal([]byte(extraData), &extra))
			require.Equal(t, "100", extra.PromptTokenNum)
			require.Equal(t, "50", extra.CompletionTokenNum)
			require.Equal(t, tt.wantReasoningNum, extra.ReasoningTokenNum)
		})
	}
}

func TestDbUpstreamsToConfigsMetadataPassthrough(t *testing.T) {
	metadata := &commontypes.UpstreamMetadata{
		ResponsesChatAdapter: &commontypes.ResponsesChatAdapter{
			ReasoningRequest: &commontypes.ReasoningRequestConfig{
				Enabled: true,
			},
		},
	}
	result := dbUpstreamsToConfigs([]database.Upstream{{
		ID:       1,
		URL:      "http://upstream.example.com/v1/chat/completions",
		Enabled:  true,
		Metadata: metadata,
	}}, commontypes.CapacityPolicy{})
	require.Len(t, result, 1)
	require.Equal(t, metadata, result[0].Metadata)
}

func TestDbUpstreamsToConfigsCapacityPolicyDefaults(t *testing.T) {
	defaults := commontypes.CapacityPolicy{
		MaxConcurrency:   16,
		MaxQueueDepth:    32,
		MaxTPM:           200000,
		MaxRPM:           120,
		QueueWaitSeconds: 60,
	}

	t.Run("enabled policy with all limits unset is default-filled as queue mode", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
			CapacityPolicy: &commontypes.CapacityPolicy{
				Enabled: true,
			},
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Len(t, result, 1)
		// Queue-mode fill: TPM/RPM stay unset — not enforceable in queue mode.
		require.Equal(t, &commontypes.CapacityPolicy{
			Enabled:          true,
			MaxConcurrency:   16,
			MaxQueueDepth:    32,
			QueueWaitSeconds: 60,
		}, result[0].CapacityPolicy)
		require.True(t, result[0].CapacityPolicy.QueueEnabled())
	})

	t.Run("enabled policy with positive limits keeps its values", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
			CapacityPolicy: &commontypes.CapacityPolicy{
				Enabled:        true,
				MaxConcurrency: 4,
				MaxQueueDepth:  8,
				MaxTPM:         1000,
				MaxRPM:         10,
			},
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Len(t, result, 1)
		require.Equal(t, &commontypes.CapacityPolicy{
			Enabled:        true,
			MaxConcurrency: 4,
			MaxQueueDepth:  8,
			MaxTPM:         1000,
			MaxRPM:         10,
		}, result[0].CapacityPolicy)
	})

	t.Run("any limit set keeps the whole policy untouched", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
			CapacityPolicy: &commontypes.CapacityPolicy{
				Enabled: true,
				MaxTPM:  5000,
			},
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Len(t, result, 1)
		// MaxTPM=0 is explicit unlimited; defaults must not leak in.
		require.Equal(t, &commontypes.CapacityPolicy{
			Enabled: true,
			MaxTPM:  5000,
		}, result[0].CapacityPolicy)
	})

	t.Run("disabled policy is left untouched", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
			CapacityPolicy: &commontypes.CapacityPolicy{
				Enabled:        false,
				MaxConcurrency: 0,
				MaxRPM:         -1,
			},
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Len(t, result, 1)
		require.Equal(t, &commontypes.CapacityPolicy{
			Enabled:        false,
			MaxConcurrency: 0,
			MaxRPM:         -1,
		}, result[0].CapacityPolicy)
	})

	t.Run("nil policy stays nil", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Len(t, result, 1)
		require.Nil(t, result[0].CapacityPolicy)
	})

	t.Run("zero defaults keep no-limit semantics for enabled policy", func(t *testing.T) {
		dbUpstreams := []database.Upstream{{
			ID:  1,
			URL: "http://upstream.example.com",
			CapacityPolicy: &commontypes.CapacityPolicy{
				Enabled: true,
			},
		}}
		result := dbUpstreamsToConfigs(dbUpstreams, commontypes.CapacityPolicy{})
		require.Len(t, result, 1)
		require.Equal(t, &commontypes.CapacityPolicy{Enabled: true}, result[0].CapacityPolicy)
	})

	t.Run("database upstream policy is not mutated", func(t *testing.T) {
		dbPolicy := &commontypes.CapacityPolicy{Enabled: true, MaxTPM: 0}
		dbUpstreams := []database.Upstream{{
			ID:             1,
			URL:            "http://upstream.example.com",
			CapacityPolicy: dbPolicy,
		}}
		_ = dbUpstreamsToConfigs(dbUpstreams, defaults)
		require.Equal(t, &commontypes.CapacityPolicy{Enabled: true, MaxTPM: 0}, dbPolicy)
	})
}

func TestModelUpstreamsAvailable(t *testing.T) {
	healthyUpstream := func() commontypes.UpstreamConfig {
		return commontypes.UpstreamConfig{
			Enabled:            true,
			HealthCheckEnabled: true,
			HealthState:        string(types.HealthStateHealthy),
		}
	}
	disabledUpstream := func() commontypes.UpstreamConfig {
		return commontypes.UpstreamConfig{Enabled: false}
	}
	circuitOpenUpstream := func() commontypes.UpstreamConfig {
		return commontypes.UpstreamConfig{
			Enabled:               true,
			CircuitBreakerEnabled: true,
			CircuitState:          string(types.CircuitStateOpen),
		}
	}
	unhealthyUpstream := func() commontypes.UpstreamConfig {
		return commontypes.UpstreamConfig{
			Enabled:            true,
			HealthCheckEnabled: true,
			HealthState:        string(types.HealthStateUnhealthy),
		}
	}
	unknownUpstream := func() commontypes.UpstreamConfig {
		return commontypes.UpstreamConfig{
			Enabled:     true,
			HealthState: string(types.HealthStateUnknown),
		}
	}

	tests := []struct {
		name      string
		upstreams []commontypes.UpstreamConfig
		want      bool
	}{
		{
			name:      "empty upstreams is unavailable",
			upstreams: nil,
			want:      false,
		},
		{
			name:      "single healthy upstream",
			upstreams: []commontypes.UpstreamConfig{healthyUpstream()},
			want:      true,
		},
		{
			name:      "single disabled upstream",
			upstreams: []commontypes.UpstreamConfig{disabledUpstream()},
			want:      false,
		},
		{
			name:      "single circuit-open upstream",
			upstreams: []commontypes.UpstreamConfig{circuitOpenUpstream()},
			want:      false,
		},
		{
			name:      "single unhealthy upstream",
			upstreams: []commontypes.UpstreamConfig{unhealthyUpstream()},
			want:      false,
		},
		{
			name:      "all upstreams unavailable",
			upstreams: []commontypes.UpstreamConfig{disabledUpstream(), circuitOpenUpstream(), unhealthyUpstream()},
			want:      false,
		},
		{
			name:      "at least one healthy among unavailable",
			upstreams: []commontypes.UpstreamConfig{disabledUpstream(), healthyUpstream(), circuitOpenUpstream()},
			want:      true,
		},
		{
			name:      "unknown health state is available",
			upstreams: []commontypes.UpstreamConfig{unknownUpstream()},
			want:      true,
		},
		{
			name: "health check disabled with healthy state",
			upstreams: []commontypes.UpstreamConfig{
				{Enabled: true, HealthState: string(types.HealthStateUnhealthy)},
			},
			want: true, // health check not enabled, so unhealthy state is ignored
		},
		{
			name: "circuit breaker disabled with open state",
			upstreams: []commontypes.UpstreamConfig{
				{Enabled: true, CircuitState: string(types.CircuitStateOpen)},
			},
			want: true, // circuit breaker not enabled, so open state is ignored
		},
		{
			name: "degraded health state",
			upstreams: []commontypes.UpstreamConfig{
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(types.HealthStateDegraded)},
			},
			want: true, // degraded is not unhealthy
		},
		{
			name: "circuit half-open",
			upstreams: []commontypes.UpstreamConfig{
				{Enabled: true, CircuitBreakerEnabled: true, CircuitState: string(types.CircuitStateHalfOpen)},
			},
			want: true, // half-open is not open
		},
		{
			name: "circuit closed",
			upstreams: []commontypes.UpstreamConfig{
				{Enabled: true, CircuitBreakerEnabled: true, CircuitState: string(types.CircuitStateClosed)},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelUpstreamsAvailable(tt.upstreams)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestComputeModelListAvailability(t *testing.T) {
	makeModel := func(id string, upstreams []commontypes.UpstreamConfig) types.Model {
		return types.Model{BaseModel: types.BaseModel{ID: id}, Upstreams: upstreams}
	}
	availableUpstream := commontypes.UpstreamConfig{
		Enabled:            true,
		HealthCheckEnabled: true,
		HealthState:        string(types.HealthStateHealthy),
	}
	disabledUpstream := commontypes.UpstreamConfig{Enabled: false}

	t.Run("nil input", func(t *testing.T) {
		result := computeModelListAvailability(nil)
		assert.Nil(t, result)
	})

	t.Run("empty input", func(t *testing.T) {
		result := computeModelListAvailability([]types.Model{})
		assert.Len(t, result, 0)
	})

	t.Run("all models available", func(t *testing.T) {
		models := []types.Model{
			makeModel("m1", []commontypes.UpstreamConfig{availableUpstream}),
			makeModel("m2", []commontypes.UpstreamConfig{availableUpstream}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 2)
		assert.Equal(t, "m1", result[0].ID)
		assert.Equal(t, "m2", result[1].ID)
		assert.True(t, result[0].Availability.IsAvailable)
		assert.True(t, result[1].Availability.IsAvailable)
	})

	t.Run("all models unavailable", func(t *testing.T) {
		models := []types.Model{
			makeModel("m1", []commontypes.UpstreamConfig{disabledUpstream}),
			makeModel("m2", []commontypes.UpstreamConfig{disabledUpstream}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 0)
	})

	t.Run("mixed available and unavailable", func(t *testing.T) {
		models := []types.Model{
			makeModel("m-available", []commontypes.UpstreamConfig{availableUpstream}),
			makeModel("m-unavailable", []commontypes.UpstreamConfig{disabledUpstream}),
			makeModel("m-available2", []commontypes.UpstreamConfig{availableUpstream}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 2)
		assert.Equal(t, "m-available", result[0].ID)
		assert.True(t, result[0].Availability.IsAvailable)
		assert.Equal(t, "m-available2", result[1].ID)
		assert.True(t, result[1].Availability.IsAvailable)
	})

	t.Run("model with no upstreams is unavailable", func(t *testing.T) {
		models := []types.Model{
			makeModel("no-upstreams", nil),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 0)
	})

	t.Run("model with mixed upstreams one healthy", func(t *testing.T) {
		models := []types.Model{
			makeModel("mixed", []commontypes.UpstreamConfig{disabledUpstream, availableUpstream}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 1)
		assert.True(t, result[0].Availability.IsAvailable, "model with at least one healthy upstream should be available")
	})

	t.Run("circuit-open upstream makes model unavailable", func(t *testing.T) {
		models := []types.Model{
			makeModel("circuit-open", []commontypes.UpstreamConfig{{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(types.CircuitStateOpen),
			}}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 0)
	})

	t.Run("unhealthy upstream makes model unavailable", func(t *testing.T) {
		models := []types.Model{
			makeModel("unhealthy", []commontypes.UpstreamConfig{{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(types.HealthStateUnhealthy),
			}}),
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 0)
	})

	t.Run("preserves model fields on kept models", func(t *testing.T) {
		models := []types.Model{
			{BaseModel: types.BaseModel{ID: "keep", OwnedBy: "owner1"}, Upstreams: []commontypes.UpstreamConfig{availableUpstream}},
		}
		result := computeModelListAvailability(models)
		assert.Len(t, result, 1)
		assert.Equal(t, "keep", result[0].ID)
		assert.Equal(t, "owner1", result[0].OwnedBy)
		assert.True(t, result[0].Availability.IsAvailable)
	})

	t.Run("does not mutate input slice", func(t *testing.T) {
		models := []types.Model{
			makeModel("a", []commontypes.UpstreamConfig{availableUpstream}),
			makeModel("b", []commontypes.UpstreamConfig{disabledUpstream}),
		}
		originalLen := len(models)
		_ = computeModelListAvailability(models)
		assert.Len(t, models, originalLen, "input slice length must not change")
	})
}

// buildInternalLLMConfigForVisibility builds an llm_config with a single
// csghub-sourced upstream carrying the given internal model info, for
// llmConfigToModel visibility tests.
func buildInternalLLMConfigForVisibility(info commontypes.InternalModelInfo) *database.LLMConfig {
	return &database.LLMConfig{
		ID:        1,
		ModelName: info.LegacyModelID,
		Enabled:   true,
		Upstreams: []database.Upstream{
			{
				ID:      11,
				URL:     "http://svc:8080/v1",
				Enabled: true,
				Source:  commontypes.UpstreamSourceCSGHubDeploy,
				Metadata: &commontypes.UpstreamMetadata{
					InternalModelInfo: &info,
				},
			},
		},
	}
}

// llmConfigToModel visibility: public (SecureLevel=EndpointPublic) internal
// models skip the owner check and are visible to any caller; private and
// unset (legacy zero-value) secure levels remain owner-only; serverless is
// always visible; the owner always sees their own model.
func TestLLMConfigToModel_VisibilityBySecureLevel(t *testing.T) {
	ctx := context.Background()
	callerUUID := "caller-uuid"
	ownerUUID := "owner-uuid"

	baseInfo := commontypes.InternalModelInfo{
		CSGHubModelID:  "ns/model",
		LegacyModelID:  "model:abc",
		OwnerUUID:      ownerUUID,
		OwnerUsername:  "owner",
		OwnerNamespace: "owner",
		SvcType:        commontypes.InferenceType,
		SourceDeployID: 100,
	}

	t.Run("public inference model visible to non-owner", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = commontypes.EndpointPublic

		model, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), callerUUID)
		require.True(t, ok)
		assert.Equal(t, info.LegacyModelID, model.ID)
	})

	t.Run("private inference model hidden from non-owner", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = commontypes.EndpointPrivate

		_, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), callerUUID)
		assert.False(t, ok, "private model must not be visible to a non-owner")
	})

	t.Run("unset secure level keeps owner-only visibility for backward compatibility", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = 0

		_, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), callerUUID)
		assert.False(t, ok, "legacy zero-value secure level must stay owner-only")
	})

	t.Run("owner sees their own private model", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = commontypes.EndpointPrivate

		model, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), ownerUUID)
		require.True(t, ok)
		assert.Equal(t, info.LegacyModelID, model.ID)
	})

	t.Run("public serverless model visible to non-owner", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = commontypes.EndpointPublic
		info.SvcType = commontypes.ServerlessType

		model, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), callerUUID)
		require.True(t, ok)
		assert.Equal(t, info.LegacyModelID, model.ID)
	})

	t.Run("visibility check skipped when applyVisibility is false", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		info := baseInfo
		info.SecureLevel = commontypes.EndpointPrivate

		model, ok := comp.llmConfigToModel(ctx, buildInternalLLMConfigForVisibility(info), callerUUID)
		require.False(t, ok)
		assert.Empty(t, model.ID)
	})
}
