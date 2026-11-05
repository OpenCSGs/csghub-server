package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// accessPolicyRow builds an external llm config row with the given access
// policy. Self-contained on purpose: the CE-tagged catalogue test helpers
// are not visible under the ee/saas tags.
func accessPolicyRow(name, policy string) *database.LLMConfig {
	row := &database.LLMConfig{
		ModelName: name,
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Provider:  "zhipu",
		Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
		Upstreams: []database.Upstream{{
			ID: 7, Source: commontypes.UpstreamSourceExternal,
			URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
		}},
	}
	row.AccessPolicy = policy
	return row
}

// whitelistOrPublicRows returns one whitelist-mode and one public llm config
// row, both with the production column values ("public" is the NOT NULL
// default, not an empty string).
func whitelistOrPublicRows() []*database.LLMConfig {
	return []*database.LLMConfig{
		accessPolicyRow("wl-model", commontypes.LLMAccessPolicyWhitelist),
		accessPolicyRow("public-model", commontypes.LLMAccessPolicyPublic),
	}
}

func newAccessPolicyStore(t *testing.T) *mockdb.MockLLMConfigStore {
	t.Helper()
	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().ListGatewayModels(mock.Anything, -1, 0, mock.Anything).
		Return(whitelistOrPublicRows(), 2, nil).Once()
	store.EXPECT().GetByModelNameForCaller(mock.Anything, "auto", mock.Anything).Return(nil, nil).Maybe()
	return store
}

// The whitelist badge rides only on the model-services query
// (has_associated_model=true), and only for whitelist-mode models: the
// column's "public" default stays off the wire, and every other listing and
// the single-model detail carry no access_policy at all.
func TestListModels_AccessPolicyOnlyOnModelServicesQuery(t *testing.T) {
	findRow := func(list types.ModelList, id string) types.Model {
		for _, m := range list.Data {
			if m.ID == id {
				return m
			}
		}
		return types.Model{}
	}

	t.Run("carried for whitelist models on the model-services query", func(t *testing.T) {
		comp := &openaiComponentImpl{extllmStore: newAccessPolicyStore(t)}
		flag := true
		list, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}, HasAssociatedModel: &flag})
		require.NoError(t, err)
		require.Len(t, list.Data, 2)
		require.Equal(t, commontypes.LLMAccessPolicyWhitelist, findRow(list, "wl-model").AccessPolicy)
		require.Empty(t, findRow(list, "public-model").AccessPolicy, "the production public value stays off the wire")
	})

	t.Run("stripped without the flag", func(t *testing.T) {
		comp := &openaiComponentImpl{extllmStore: newAccessPolicyStore(t)}
		list, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}})
		require.NoError(t, err)
		require.Len(t, list.Data, 2)
		require.Empty(t, findRow(list, "wl-model").AccessPolicy)
		require.Empty(t, findRow(list, "public-model").AccessPolicy)
	})

	t.Run("stripped when the flag is false", func(t *testing.T) {
		comp := &openaiComponentImpl{extllmStore: newAccessPolicyStore(t)}
		flag := false
		list, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}, HasAssociatedModel: &flag})
		require.NoError(t, err)
		require.Len(t, list.Data, 2)
		require.Empty(t, findRow(list, "wl-model").AccessPolicy)
		require.Empty(t, findRow(list, "public-model").AccessPolicy)
	})

	t.Run("single-model detail carries no access policy", func(t *testing.T) {
		store := mockdb.NewMockLLMConfigStore(t)
		store.EXPECT().GetByModelNameForCaller(mock.Anything, "wl-model", mock.Anything).
			Return(whitelistOrPublicRows()[0], nil).Once()
		comp := &openaiComponentImpl{extllmStore: store}
		model, err := comp.GetModelByID(context.Background(), &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "wl-model"})
		require.NoError(t, err)
		require.NotNil(t, model)
		require.Empty(t, model.AccessPolicy)
	})

	t.Run("the shared conversion never fills the policy", func(t *testing.T) {
		// A future interface converting through llmConfigToModel must not leak
		// the whitelist marker: the conversion itself never sets the field.
		comp := &openaiComponentImpl{}
		model, ok := comp.llmConfigToModel(context.Background(), whitelistOrPublicRows()[0], "")
		require.True(t, ok)
		require.Empty(t, model.AccessPolicy)
	})
}
