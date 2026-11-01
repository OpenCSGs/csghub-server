//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// The catalogue listing trusts the is_available filter inside the listing
// statement: a row returned by the store is annotated with the availability
// derived from its (marginally newer) relation states but is never dropped —
// dropping it would shrink the page below total_count and skip it on the
// next offset.
func TestListModels_CatalogueAnnotatesWithoutDroppingUnavailableRows(t *testing.T) {
	// The mock returns a row whose loaded relation state is unhealthy — the
	// "relation snapshot is newer than the listing statement" divergence.
	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().ListGatewayModels(mock.Anything, -1, 0, mock.Anything).
		Return([]*database.LLMConfig{modelRowWithUpstream("glm-5.1", database.Upstream{
			ID: 7, Source: commontypes.UpstreamSourceExternal,
			URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
			HealthCheckEnabled: true,
			HealthState:        &database.AIGatewayUpstreamHealthState{HealthState: "unhealthy"},
		})}, 1, nil).Once()
	store.EXPECT().GetByModelNameForCaller(mock.Anything, "auto", mock.Anything).Return(nil, nil).Maybe()

	comp := &openaiComponentImpl{extllmStore: store}

	list, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}})
	require.NoError(t, err)
	require.Len(t, list.Data, 1, "the catalogue path must not drop rows after the statement")
	assert.Equal(t, "glm-5.1", list.Data[0].ID)
	assert.NotNil(t, list.Data[0].Availability)
	assert.False(t, list.Data[0].Availability.IsAvailable, "the row is annotated with the reloaded state")
}

// The availability annotator never drops rows, whatever the loaded states say.
func TestAnnotateModelListAvailability_KeepsAllRows(t *testing.T) {
	models := []types.Model{
		{BaseModel: types.BaseModel{ID: "healthy"}, Upstreams: []commontypes.UpstreamConfig{
			{ID: 1, Enabled: true, URL: "http://ok/v1"},
		}},
		{BaseModel: types.BaseModel{ID: "upstreamless"}},
	}
	annotateModelListAvailability(models)
	require.Len(t, models, 2, "annotation must not drop rows")
	assert.True(t, models[0].Availability.IsAvailable)
	assert.False(t, models[1].Availability.IsAvailable)
}

// CE never publishes the virtual model, even if a router is injected by a
// caller using the shared component type.
func TestListModels_CEDoesNotPublishAutoModel(t *testing.T) {
	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().ListGatewayModels(mock.Anything, -1, 0, mock.Anything).
		Return([]*database.LLMConfig{modelRowWithUpstream("glm-5.1", database.Upstream{
			ID: 7, Source: commontypes.UpstreamSourceExternal,
			URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
		})}, 1, nil).Once()
	comp := &openaiComponentImpl{extllmStore: store, autoRouter: mockcomponent.NewMockAutoModelRouter(t)}
	list, err := comp.ListModels(context.Background(), types.ListModelsReq{Caller: &commontypes.GatewayWhitelistCaller{}})
	require.NoError(t, err)
	require.Len(t, list.Data, 1)
	assert.Equal(t, "glm-5.1", list.Data[0].ID)
	assert.Empty(t, comp.AutoModelID())
	_, err = comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{})
	require.ErrorContains(t, err, "not available in CE")
}

func modelRowWithUpstream(modelName string, upstream database.Upstream) *database.LLMConfig {
	return &database.LLMConfig{
		ID: 1, ModelName: modelName, Type: database.LLMTypeAigatewayExternal,
		Enabled: true, Provider: "zhipu",
		Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
		Upstreams: []database.Upstream{upstream},
	}
}
