package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// Integration test: replicates the admin frontend PUT /api/v1/config/upstreams/:id
// flow (JSON binding -> component -> DB) and the client read-back paths, to verify
// capacity_policy (including queue_wait_seconds) is persisted and returned.
func TestLLMServiceComponent_UpdateUpstream_CapacityPolicy_DBPersistence(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	llmConfigStore := database.NewLLMConfigStoreWithDB(db, &config.Config{})
	upstreamStore := database.NewUpstreamStoreWithDB(db, nil)
	healthStateStore := database.NewAIGatewayUpstreamHealthStateStoreWithDB(db)
	circuitStateStore := database.NewAIGatewayUpstreamCircuitStateStoreWithDB(db)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    llmConfigStore,
		upstreamStore:     upstreamStore,
		healthStateStore:  healthStateStore,
		circuitStateStore: circuitStateStore,
	}

	cfg := &database.LLMConfig{
		ModelName:   "capacity-e2e-model",
		ApiEndpoint: "http://example.com/v1",
		AuthHeader:  "none",
		Type:        database.LLMTypeAigatewayExternal,
		Enabled:     true,
	}
	createdCfg, err := llmConfigStore.Create(ctx, *cfg)
	require.NoError(t, err)
	cfg.ID = createdCfg.ID

	dbUp := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://upstream.example.com/v1",
		Weight:      1,
		Enabled:     true,
		Source:      types.UpstreamSourceExternal,
	}
	require.NoError(t, upstreamStore.Create(ctx, dbUp))

	t.Run("frontend payload with queue_wait_seconds persists to DB", func(t *testing.T) {
		payload := `{"capacity_policy":{"enabled":true,"max_concurrency":5,"max_queue_depth":10,"max_tpm":1000,"max_rpm":20,"queue_wait_seconds":30}}`
		var req *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(payload), &req))
		req.ID = dbUp.ID

		res, err := mc.UpdateUpstream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, 30, res.CapacityPolicy.QueueWaitSeconds)

		persisted, err := upstreamStore.GetByID(ctx, dbUp.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted.CapacityPolicy)
		require.Equal(t, 5, persisted.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 30, persisted.CapacityPolicy.QueueWaitSeconds)

		// Client read path (ShowLLMConfig) returns the persisted policy.
		shown, err := mc.ShowLLMConfig(ctx, cfg.ID)
		require.NoError(t, err)
		require.NotNil(t, shown)
		require.Len(t, shown.Upstreams, 1)
		require.NotNil(t, shown.Upstreams[0].CapacityPolicy)
		require.Equal(t, 30, shown.Upstreams[0].CapacityPolicy.QueueWaitSeconds)
	})

	t.Run("enabled policy with queue_wait_seconds omitted (0) is accepted as unset", func(t *testing.T) {
		payload := `{"capacity_policy":{"enabled":true,"max_concurrency":8,"max_rpm":10}}`
		var req *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(payload), &req))
		req.ID = dbUp.ID

		res, err := mc.UpdateUpstream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, 8, res.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 0, res.CapacityPolicy.QueueWaitSeconds)

		persisted, err := upstreamStore.GetByID(ctx, dbUp.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted.CapacityPolicy)
		require.Equal(t, 8, persisted.CapacityPolicy.MaxConcurrency)
	})

	t.Run("enabled policy with all limits unset (all zeros) is accepted for default-fill", func(t *testing.T) {
		payload := `{"capacity_policy":{"enabled":true}}`
		var req *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(payload), &req))
		req.ID = dbUp.ID

		res, err := mc.UpdateUpstream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, true, res.CapacityPolicy.Enabled)

		persisted, err := upstreamStore.GetByID(ctx, dbUp.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted.CapacityPolicy)
		require.Equal(t, true, persisted.CapacityPolicy.Enabled)
	})

	t.Run("negative limits are rejected", func(t *testing.T) {
		payload := `{"capacity_policy":{"enabled":true,"max_concurrency":-1}}`
		var req *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(payload), &req))
		req.ID = dbUp.ID

		_, err := mc.UpdateUpstream(ctx, req)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidLLMConfig)
	})

	t.Run("capacity_policy omitted keeps stored policy", func(t *testing.T) {
		payload := `{"weight":2}`
		var req *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(payload), &req))
		req.ID = dbUp.ID

		res, err := mc.UpdateUpstream(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, true, res.CapacityPolicy.Enabled)
	})

	// Locks in the HTTP-level semantics of capacity_policy updates on a
	// dedicated upstream row: explicit JSON null binds as omitted (keep),
	// a limit-free update only toggles Enabled and preserves the stored
	// limits (temporary disable + re-enable), and an explicit limit
	// replaces the stored policy wholesale.
	t.Run("HTTP update semantics: null keeps, toggle preserves limits", func(t *testing.T) {
		up := &database.Upstream{
			LLMConfigID: cfg.ID,
			URL:         "http://toggle.example.com/v1",
			Weight:      1,
			Enabled:     true,
			Source:      types.UpstreamSourceExternal,
		}
		require.NoError(t, upstreamStore.Create(ctx, up))

		seed := `{"capacity_policy":{"enabled":true,"max_concurrency":5,"max_rpm":20,"queue_wait_seconds":30}}`
		var seedReq *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(seed), &seedReq))
		seedReq.ID = up.ID
		_, err := mc.UpdateUpstream(ctx, seedReq)
		require.NoError(t, err)

		var nullReq *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(`{"capacity_policy":null}`), &nullReq))
		nullReq.ID = up.ID
		res, err := mc.UpdateUpstream(ctx, nullReq)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, true, res.CapacityPolicy.Enabled)
		require.Equal(t, 5, res.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 30, res.CapacityPolicy.QueueWaitSeconds)

		var offReq *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(`{"capacity_policy":{"enabled":false}}`), &offReq))
		offReq.ID = up.ID
		res, err = mc.UpdateUpstream(ctx, offReq)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, false, res.CapacityPolicy.Enabled)
		require.Equal(t, 5, res.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 20, res.CapacityPolicy.MaxRPM)
		require.Equal(t, 30, res.CapacityPolicy.QueueWaitSeconds)

		persisted, err := upstreamStore.GetByID(ctx, up.ID)
		require.NoError(t, err)
		require.NotNil(t, persisted.CapacityPolicy)
		require.Equal(t, false, persisted.CapacityPolicy.Enabled)
		require.Equal(t, 5, persisted.CapacityPolicy.MaxConcurrency)

		var onReq *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(`{"capacity_policy":{"enabled":true}}`), &onReq))
		onReq.ID = up.ID
		res, err = mc.UpdateUpstream(ctx, onReq)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, true, res.CapacityPolicy.Enabled)
		require.Equal(t, 5, res.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 20, res.CapacityPolicy.MaxRPM)
		require.Equal(t, 30, res.CapacityPolicy.QueueWaitSeconds)

		var replaceReq *types.UpdateUpstreamReq
		require.NoError(t, json.Unmarshal([]byte(`{"capacity_policy":{"enabled":true,"max_concurrency":9}}`), &replaceReq))
		replaceReq.ID = up.ID
		res, err = mc.UpdateUpstream(ctx, replaceReq)
		require.NoError(t, err)
		require.NotNil(t, res.CapacityPolicy)
		require.Equal(t, true, res.CapacityPolicy.Enabled)
		require.Equal(t, 9, res.CapacityPolicy.MaxConcurrency)
		require.Equal(t, 0, res.CapacityPolicy.MaxRPM)
		require.Equal(t, 0, res.CapacityPolicy.QueueWaitSeconds)
	})
}
