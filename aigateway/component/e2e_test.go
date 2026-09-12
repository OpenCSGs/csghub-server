package component

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/component/upstream"
	"opencsg.com/csghub-server/aigateway/types"
	deploybuilder "opencsg.com/csghub-server/builder/deploy"
	deploycommon "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	commontypes "opencsg.com/csghub-server/common/types"
)

// e2eTestEnv holds all the real DB-backed stores and components needed for
// end-to-end integration tests of the unified upstream read/write path.
type e2eTestEnv struct {
	db             *database.DB
	syncComp       upstream.AIGatewayUpstreamSyncComponent
	openaiComp     *openaiComponentImpl
	deployStore    database.DeployTaskStore
	upstreamStore  database.UpstreamStore
	llmConfigStore database.LLMConfigStore
	repoStore      database.RepoStore
	userStore      database.UserStore
	modelIDBuilder upstream.ModelIDBuilder
}

func newE2ETestEnv(t *testing.T) *e2eTestEnv {
	t.Helper()
	db := tests.InitTestDB()

	cfg := &config.Config{}
	cfg.Database.Driver = "pg"
	cfg.Database.SearchConfiguration = "opencsgchinese"

	deployStore := database.NewDeployTaskStoreWithDB(db)
	upstreamStore := database.NewUpstreamStoreWithDB(db, nil)
	llmConfigStore := database.NewLLMConfigStoreWithDB(db, cfg)
	repoStore := database.NewRepoStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	clusterStore := database.NewClusterInfoStoreWithDB(db)
	modelIDBuilder := upstream.NewModelIDBuilder()

	syncComp := upstream.NewAIGatewayUpstreamSyncComponent(upstream.AIGatewayUpstreamSyncComponentConfig{
		DeployStore:    deployStore,
		UpstreamStore:  upstreamStore,
		LLMConfigStore: llmConfigStore,
		ClusterStore:   clusterStore,
		ModelIDBuilder: modelIDBuilder,
	})

	openaiComp := &openaiComponentImpl{
		extllmStore:    llmConfigStore,
		modelIDBuilder: modelIDBuilder,
		extendOpenai:   newExtendOpenaiForTest(db),
	}

	return &e2eTestEnv{
		db:             db,
		syncComp:       syncComp,
		openaiComp:     openaiComp,
		deployStore:    deployStore,
		upstreamStore:  upstreamStore,
		llmConfigStore: llmConfigStore,
		repoStore:      repoStore,
		userStore:      userStore,
		modelIDBuilder: modelIDBuilder,
	}
}

// createTestUserAndRepo inserts a user and a model repository into the database
// and returns them. These are prerequisites for creating a deploy record.
func createTestUserAndRepo(ctx context.Context, t *testing.T, env *e2eTestEnv, username, repoPath string) (*database.User, *database.Repository) {
	t.Helper()
	user := &database.User{
		UUID:     "e2e-" + username + "-uuid",
		Username: username,
		NickName: username,
		Email:    username + "@e2e-test.com",
		Password: "dummy",
		GitID:    int64(len(username) + 900000),
	}
	_, err := env.db.Core.NewInsert().Model(user).Exec(ctx, user)
	require.NoError(t, err)

	repo, err := env.repoStore.CreateRepo(ctx, database.Repository{
		Path:           repoPath,
		GitPath:        "models/" + repoPath,
		Name:           repoPath,
		Nickname:       repoPath,
		RepositoryType: commontypes.ModelRepo,
		UserID:         user.ID,
		DefaultBranch:  "main",
	})
	require.NoError(t, err)
	return user, repo
}

// createTestDeploy inserts a deploy record with the given type and status.
func createTestDeploy(ctx context.Context, t *testing.T, env *e2eTestEnv, user *database.User, repo *database.Repository, deployType int, svcName string) *database.Deploy {
	t.Helper()
	deploy := &database.Deploy{
		SpaceID:   0,
		Status:    deploycommon.Running, // 23 — matches ListAllRunningDeploys filter
		GitPath:   repo.Path,
		GitBranch: "main",
		Template:  "default",
		Hardware:  "GPU",
		UserID:    user.ID,
		RepoID:    repo.ID,
		Type:      deployType,
		SvcName:   svcName,
		Endpoint:  "http://" + svcName + ".svc.cluster.local:8080/v1",
		ClusterID: "",
		RuntimeFramework: "vllm",
	}
	err := env.deployStore.CreateDeploy(ctx, deploy)
	require.NoError(t, err)
	require.NotZero(t, deploy.ID)
	return deploy
}

// syncDeployByID loads a deploy with relations from the DB, builds a
// DeployUpstreamInfo, and calls SyncRunningDeploy. This simulates the
// trigger-side flow: the trigger loads the deploy, builds the info, and
// the consumer processes it. Returns the built info so callers can access
// the pre-computed LegacyModelID.
func syncDeployByID(ctx context.Context, t *testing.T, env *e2eTestEnv, deployID int64) (*commontypes.DeployUpstreamInfo, error) {
	t.Helper()
	deploy, err := env.deployStore.GetDeployByIDWithRelations(ctx, deployID)
	require.NoError(t, err)
	require.NotNil(t, deploy)
	info := deploybuilder.BuildDeployUpstreamInfoWithDeploy(ctx, deploy, nil)
	require.NotNil(t, info)
	return info, env.syncComp.SyncRunningDeploy(ctx, info)
}

// createExternalLLMConfig inserts an external llm_config with an external upstream directly
// into the database (bypassing the sync component, since external models are not managed by deploy sync).
func createExternalLLMConfig(ctx context.Context, t *testing.T, env *e2eTestEnv, modelName, provider, url string) *database.LLMConfig {
	t.Helper()
	cfg, err := env.llmConfigStore.Create(ctx, database.LLMConfig{
		ModelName: modelName,
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Provider:  provider,
		AuthHeader: "Bearer ext-key",
		Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
	})
	require.NoError(t, err)

	up := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         url,
		Weight:      1,
		Enabled:     true,
		ModelName:   modelName,
		Source:      commontypes.UpstreamSourceExternal,
		Provider:    provider,
		AuthHeader:  "Bearer ext-key",
	}
	err = env.upstreamStore.Create(ctx, up)
	require.NoError(t, err)
	return cfg
}

// =================================================================
// E2E Test: Deploy Sync Flow (running → stop → delete)
// =================================================================

func TestE2E_DeploySync_RunningCreatesUpstream(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sync-running-user", "e2e/sync-running-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-running")

	// Before sync: no upstream should exist for this deploy.
	existing, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.Nil(t, existing)

	// Run sync — simulates a "deploy is running" event.
	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	// Verify the upstream was created in the DB.
	upstream, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream)
	assert.True(t, upstream.Enabled, "upstream should be enabled for a running deploy")
	assert.Equal(t, commontypes.UpstreamSourceCSGHubDeploy, upstream.Source)
	assert.Equal(t, deploy.ID, upstream.SourceID)
	assert.Equal(t, repo.Path, upstream.ModelName)
	assert.Equal(t, "http://svc-e2e-running.svc.cluster.local:8080/v1", upstream.URL)
	assert.NotNil(t, upstream.Metadata)
	require.NotNil(t, upstream.Metadata.InternalModelInfo)
	assert.Equal(t, repo.Path, upstream.Metadata.InternalModelInfo.CSGHubModelID)
	assert.Equal(t, user.UUID, upstream.Metadata.InternalModelInfo.OwnerUUID)
	assert.Equal(t, user.Username, upstream.Metadata.InternalModelInfo.OwnerUsername)
	assert.Equal(t, deploy.SvcName, upstream.Metadata.InternalModelInfo.SvcName)
	assert.Equal(t, commontypes.InferenceType, upstream.Metadata.InternalModelInfo.SvcType)
	assert.Equal(t, deploy.ID, upstream.Metadata.InternalModelInfo.SourceDeployID)

	// Verify an llm_config was also created with the legacy model ID.
	// The llm_config is created disabled — admin must enable it via the API.
	legacyModelID := info.LegacyModelID
	llmCfg, err := env.llmConfigStore.GetByModelName(ctx, legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, llmCfg)
	assert.False(t, llmCfg.Enabled, "llm_config should be disabled until admin enables it")
}

func TestE2E_DeploySync_StopDisablesUpstream(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sync-stop-user", "e2e/sync-stop-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-stop")

	// Sync running first to create the upstream.
	_, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	upstream, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream)
	require.True(t, upstream.Enabled)

	// Now simulate a "deploy stopped" event.
	err = env.syncComp.DisableDeployTarget(ctx, deploy.ID)
	require.NoError(t, err)

	// Verify the upstream is now disabled.
	disabled, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, disabled)
	assert.False(t, disabled.Enabled, "upstream should be disabled after stop event")
	assert.Equal(t, upstream.ID, disabled.ID, "should be the same upstream row, not a new one")
}

func TestE2E_DeploySync_DeleteRemovesUpstream(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sync-delete-user", "e2e/sync-delete-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-delete")

	// Sync running first to create the upstream.
	_, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	upstream, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream)

	// Now simulate a "deploy deleted" event.
	err = env.syncComp.DeleteDeployTarget(ctx, deploy.ID)
	require.NoError(t, err)

	// Verify the upstream is gone.
	deleted, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.Nil(t, deleted)
}

func TestE2E_DeploySync_ReSyncUpdatesUpstream(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sync-resync-user", "e2e/sync-resync-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-resync")

	// Initial sync.
	_, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	upstream1, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream1)

	// Update the deploy's endpoint and re-sync.
	deploy.Endpoint = "http://updated-endpoint.svc.cluster.local:9090/v1"
	err = env.deployStore.UpdateDeploy(ctx, deploy)
	require.NoError(t, err)

	_, err = syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	// Verify the upstream was updated (same row, new URL).
	upstream2, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream2)
	assert.Equal(t, upstream1.ID, upstream2.ID, "should be the same upstream row after re-sync")
	assert.Equal(t, "http://updated-endpoint.svc.cluster.local:9090/v1", upstream2.URL)
}

func TestE2E_DeploySync_ServerlessDeploy(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sync-serverless-user", "e2e/sync-serverless-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.ServerlessType, "svc-e2e-serverless")

	_, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	upstream, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, upstream)
	assert.True(t, upstream.Enabled)
	require.NotNil(t, upstream.Metadata.InternalModelInfo)
	assert.Equal(t, commontypes.ServerlessType, upstream.Metadata.InternalModelInfo.SvcType)
	assert.Equal(t, commontypes.ProviderTypeServerless, upstream.Provider)
}

// enableLLMConfig simulates an admin enabling an llm_config via the API.
// Internal models created by the deploy sync start disabled; this helper
// flips the llm_config to enabled so the model becomes visible/routable.
func enableLLMConfig(ctx context.Context, t *testing.T, env *e2eTestEnv, modelName string) {
	t.Helper()
	cfg, err := env.llmConfigStore.GetByModelName(ctx, modelName)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	cfg.Enabled = true
	_, err = env.llmConfigStore.Update(ctx, *cfg)
	require.NoError(t, err)
}

// =================================================================
// E2E Test: AIGateway Model Read Path (internal + external)
// =================================================================

func TestE2E_ModelRead_InternalModelAfterSync(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "read-internal-user", "e2e/read-internal-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-read-internal")

	// Sync the deploy to create upstream + llm_config.
	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	legacyModelID := info.LegacyModelID

	// Simulate admin enabling the llm_config so the model is visible.
	enableLLMConfig(ctx, t, env, legacyModelID)

	// GetModelByID should return the internal model with correct formatting.
	model, err := env.openaiComp.GetModelByID(ctx, user.UUID, legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.NotEmpty(t, model.ID, "model ID should be generated by ModelIDBuilder.To()")
	assert.Equal(t, commontypes.ProviderTypeInference, model.Metadata[types.MetaKeyLLMType])
	assert.Equal(t, repo.Path, model.Metadata[types.MetaKeyRepoPath])
	assert.Equal(t, user.UUID, model.OwnerUUID)
	assert.Equal(t, user.Username, model.OwnedBy)
	assert.Equal(t, deploy.SvcName, model.SvcName)
	assert.Equal(t, deploy.ID, model.SourceDeployID)
	assert.Len(t, model.Upstreams, 1)
	assert.Equal(t, deploy.Endpoint, model.Upstreams[0].URL)

	// GetAvailableModels: owner should see their inference model.
	models, err := env.openaiComp.GetAvailableModels(ctx, user.UUID)
	require.NoError(t, err)
	var foundInternal bool
	for _, m := range models {
		if m.ID == model.ID {
			foundInternal = true
			break
		}
	}
	assert.True(t, foundInternal, "owner should see their own inference model in GetAvailableModels")

	// Non-owner should NOT see the inference model.
	otherModels, err := env.openaiComp.GetAvailableModels(ctx, "non-owner-uuid")
	require.NoError(t, err)
	for _, m := range otherModels {
		assert.NotEqual(t, model.ID, m.ID, "non-owner should not see other's inference model")
	}
}

func TestE2E_ModelRead_ServerlessModelVisibleToAll(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "read-serverless-user", "e2e/read-serverless-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.ServerlessType, "svc-e2e-read-serverless")

	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	legacyModelID := info.LegacyModelID

	// Simulate admin enabling the llm_config so the model is visible.
	enableLLMConfig(ctx, t, env, legacyModelID)

	// Any user should see the serverless model.
	models, err := env.openaiComp.GetAvailableModels(ctx, "any-random-user-uuid")
	require.NoError(t, err)
	var found bool
	for _, m := range models {
		if m.Metadata != nil && m.Metadata[types.MetaKeyLLMType] == commontypes.ProviderTypeServerless {
			assert.Equal(t, "OpenCSG", m.OwnedBy, "serverless model should be owned by OpenCSG")
			found = true
			break
		}
	}
	assert.True(t, found, "serverless model should be visible to all users")

	// Also verify GetModelByID works.
	model, err := env.openaiComp.GetModelByID(ctx, "any-random-user-uuid", legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, commontypes.ProviderTypeServerless, model.Metadata[types.MetaKeyLLMType])
}

func TestE2E_ModelRead_ExternalModel(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	// Create an external llm_config with an external upstream directly.
	createExternalLLMConfig(ctx, t, env, "e2e-external-gpt", "openai", "http://external-openai-api/v1")

	// GetModelByID should return the external model.
	model, err := env.openaiComp.GetModelByID(ctx, "any-user", "e2e-external-gpt")
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "e2e-external-gpt", model.ID)
	assert.Equal(t, "openai", model.OwnedBy)
	assert.Equal(t, commontypes.ProviderTypeExternalLLM, model.Metadata[types.MetaKeyLLMType])
	assert.Equal(t, "openai", model.Provider)
	assert.Equal(t, "Bearer ext-key", model.AuthHead)
	assert.Len(t, model.Upstreams, 1)
	assert.Equal(t, "http://external-openai-api/v1", model.Upstreams[0].URL)

	// GetAvailableModels should include the external model for any user.
	models, err := env.openaiComp.GetAvailableModels(ctx, "any-user-uuid")
	require.NoError(t, err)
	var found bool
	for _, m := range models {
		if m.ID == "e2e-external-gpt" {
			found = true
			assert.Equal(t, commontypes.ProviderTypeExternalLLM, m.Metadata[types.MetaKeyLLMType])
			break
		}
	}
	assert.True(t, found, "external model should be visible in GetAvailableModels")
}

func TestE2E_ModelRead_MixedInternalAndExternal(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	// Create an internal model via deploy sync.
	user, repo := createTestUserAndRepo(ctx, t, env, "read-mixed-user", "e2e/read-mixed-internal")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.ServerlessType, "svc-e2e-mixed")
	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	// Simulate admin enabling the llm_config so the internal model is visible.
	mixedLegacyID := info.LegacyModelID
	enableLLMConfig(ctx, t, env, mixedLegacyID)

	// Create an external model directly.
	createExternalLLMConfig(ctx, t, env, "e2e-mixed-external", "anthropic", "http://anthropic-api/v1")

	// GetAvailableModels as any user should see both (serverless + external are both visible).
	models, err := env.openaiComp.GetAvailableModels(ctx, "any-user-uuid")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(models), 2, "should see at least the serverless + external models")

	var foundServerless, foundExternal bool
	for _, m := range models {
		if m.Metadata != nil {
			llmType := m.Metadata[types.MetaKeyLLMType]
			if llmType == commontypes.ProviderTypeServerless {
				foundServerless = true
			}
			if llmType == commontypes.ProviderTypeExternalLLM && m.ID == "e2e-mixed-external" {
				foundExternal = true
			}
		}
	}
	assert.True(t, foundServerless, "should find the serverless model")
	assert.True(t, foundExternal, "should find the external model")
}

// =================================================================
// E2E Test: Stop/Delete visibility impact on model read
// =================================================================

func TestE2E_ModelRead_DisabledUpstreamNotInAvailableModels(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "read-disabled-user", "e2e/read-disabled-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.ServerlessType, "svc-e2e-read-disabled")

	// Sync running → upstream enabled (llm_config starts disabled).
	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	legacyModelID := info.LegacyModelID

	// Simulate admin enabling the llm_config so the model is visible.
	enableLLMConfig(ctx, t, env, legacyModelID)

	// Should be visible initially.
	model, err := env.openaiComp.GetModelByID(ctx, user.UUID, legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, model)

	// Stop the deploy → upstream disabled.
	err = env.syncComp.DisableDeployTarget(ctx, deploy.ID)
	require.NoError(t, err)

	// GetModelByID should still find the llm_config (it queries by model_name,
	// not by upstream enabled state), but the model's upstream will be disabled.
	// The model itself is still "readable" — routing logic decides availability.
	modelAfterStop, err := env.openaiComp.GetModelByID(ctx, user.UUID, legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, modelAfterStop, "GetModelByID should still find the model after stop")
	assert.Len(t, modelAfterStop.Upstreams, 1)
	assert.False(t, modelAfterStop.Upstreams[0].Enabled, "upstream should be disabled after stop")

	// GetAvailableModels uses IndexWithRepo which queries enabled llm_configs.
	// The llm_config itself is still enabled (only the upstream is disabled),
	// so the model still appears in the list, but with a disabled upstream.
	models, err := env.openaiComp.GetAvailableModels(ctx, user.UUID)
	require.NoError(t, err)
	var foundDisabled bool
	for _, m := range models {
		if m.ID == modelAfterStop.ID {
			foundDisabled = true
			assert.False(t, m.Upstreams[0].Enabled, "model's upstream should be disabled in list")
		}
	}
	assert.True(t, foundDisabled, "model with disabled upstream should still appear in list (llm_config is enabled)")

	// Delete the deploy → upstream removed.
	err = env.syncComp.DeleteDeployTarget(ctx, deploy.ID)
	require.NoError(t, err)

	// GetModelByID: the llm_config still exists (DeleteDeployTarget only removes the upstream),
	// but the model will have no upstreams.
	modelAfterDelete, err := env.openaiComp.GetModelByID(ctx, user.UUID, legacyModelID)
	require.NoError(t, err)
	// The llm_config still exists, so the model is returned but with empty upstreams.
	if modelAfterDelete != nil {
		assert.Empty(t, modelAfterDelete.Upstreams, "upstreams should be empty after delete")
	}
}

func TestE2E_DeploySync_ReconcileDisablesGhostUpstreams(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user1, repo1 := createTestUserAndRepo(ctx, t, env, "reconcile-user1", "e2e/reconcile-model-1")
	deploy1 := createTestDeploy(ctx, t, env, user1, repo1, commontypes.InferenceType, "svc-e2e-reconcile-1")

	user2, repo2 := createTestUserAndRepo(ctx, t, env, "reconcile-user2", "e2e/reconcile-model-2")
	deploy2 := createTestDeploy(ctx, t, env, user2, repo2, commontypes.InferenceType, "svc-e2e-reconcile-2")

	// Sync both deploys.
	_, err := syncDeployByID(ctx, t, env, deploy1.ID)
	require.NoError(t, err)
	_, err = syncDeployByID(ctx, t, env, deploy2.ID)
	require.NoError(t, err)

	// Both upstreams should be enabled.
	up1, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy1.ID)
	require.NoError(t, err)
	require.True(t, up1.Enabled)
	up2, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy2.ID)
	require.NoError(t, err)
	require.True(t, up2.Enabled)

	// Stop deploy2 (simulating it no longer running) — update its status to stopped.
	deploy2.Status = deploycommon.Stopped // 26 — no longer matches ListAllRunningDeploys
	require.NoError(t, env.deployStore.UpdateDeploy(ctx, deploy2))

	// Reconcile: should sync deploy1 (still running) and disable deploy2's upstream (ghost).
	// Note: ListAllRunningDeploys filters by status IN (Running=23, Sleeping=25), so deploy2 (Stopped=26) won't be in the running set.
	err = env.syncComp.ReconcileUpstreams(ctx)
	require.NoError(t, err)

	// deploy1's upstream should still be enabled.
	up1After, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy1.ID)
	require.NoError(t, err)
	assert.True(t, up1After.Enabled, "deploy1 upstream should remain enabled")

	// deploy2's upstream should be disabled (ghost).
	up2After, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy2.ID)
	require.NoError(t, err)
	assert.False(t, up2After.Enabled, "deploy2 upstream should be disabled as ghost")
}

func TestE2E_DeploySync_SleepingDeployTreatedAsRunning(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "sleep-user", "e2e/sleep-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-sleep")

	// Sync the running deploy to create its upstream.
	_, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	up, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	require.True(t, up.Enabled)

	// Simulate the deploy going to sleep (status = Sleeping = 25).
	deploy.Status = deploycommon.Sleeping
	require.NoError(t, env.deployStore.UpdateDeploy(ctx, deploy))

	// Reconcile: Sleeping deploys should be treated as running, so the upstream
	// should remain enabled and not be classified as a ghost.
	err = env.syncComp.ReconcileUpstreams(ctx)
	require.NoError(t, err)

	upAfter, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy.ID)
	require.NoError(t, err)
	assert.True(t, upAfter.Enabled, "sleeping deploy upstream should remain enabled after reconcile")
}

func TestE2E_DeploySync_MultipleDeploysForSameModel(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "multi-deploy-user", "e2e/multi-deploy-model")

	// Create two deploys for the same model with different svc names.
	deploy1 := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-multi-1")
	deploy2 := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-multi-2")

	// Sync both.
	info1, err := syncDeployByID(ctx, t, env, deploy1.ID)
	require.NoError(t, err)
	info2, err := syncDeployByID(ctx, t, env, deploy2.ID)
	require.NoError(t, err)

	// Each deploy should have its own upstream.
	up1, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy1.ID)
	require.NoError(t, err)
	require.NotNil(t, up1)
	assert.Equal(t, "svc-multi-1", up1.Metadata.InternalModelInfo.SvcName)

	up2, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy2.ID)
	require.NoError(t, err)
	require.NotNil(t, up2)
	assert.Equal(t, "svc-multi-2", up2.Metadata.InternalModelInfo.SvcName)

	// Each should have a different legacy model ID and thus a different llm_config.
	legacy1 := info1.LegacyModelID
	legacy2 := info2.LegacyModelID
	assert.NotEqual(t, legacy1, legacy2)

	cfg1, err := env.llmConfigStore.GetByModelName(ctx, legacy1)
	require.NoError(t, err)
	require.NotNil(t, cfg1)
	cfg2, err := env.llmConfigStore.GetByModelName(ctx, legacy2)
	require.NoError(t, err)
	require.NotNil(t, cfg2)
	assert.NotEqual(t, cfg1.ID, cfg2.ID, "each deploy should have its own llm_config")

	// Stop one deploy — the other should remain active.
	require.NoError(t, env.syncComp.DisableDeployTarget(ctx, deploy1.ID))

	up1After, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy1.ID)
	require.NoError(t, err)
	assert.False(t, up1After.Enabled)

	up2After, err := env.upstreamStore.GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, deploy2.ID)
	require.NoError(t, err)
	assert.True(t, up2After.Enabled, "deploy2 upstream should still be enabled")
}

func TestE2E_ModelRead_GetModelByIDNotFound(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	model, err := env.openaiComp.GetModelByID(ctx, "user-uuid", "nonexistent-model-id")
	require.NoError(t, err)
	assert.Nil(t, model, "should return nil model for nonexistent model ID")
}

func TestE2E_ModelRead_EmptyDB(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	models, err := env.openaiComp.GetAvailableModels(ctx, "any-user-uuid")
	require.NoError(t, err)
	assert.Empty(t, models, "empty DB should return empty model list")
}

// Verify the full sync-then-read pipeline produces a model whose ID can be
// resolved back via GetModelByID. This catches end-to-end formatting mismatches
// between the write path (sync) and the read path (openai component).
func TestE2E_FullPipeline_SyncThenReadRoundTrip(t *testing.T) {
	env := newE2ETestEnv(t)
	defer env.db.Close()
	ctx := context.TODO()

	user, repo := createTestUserAndRepo(ctx, t, env, "pipeline-user", "e2e/pipeline-model")
	deploy := createTestDeploy(ctx, t, env, user, repo, commontypes.InferenceType, "svc-e2e-pipeline")

	// Write path: sync creates upstream + llm_config.
	info, err := syncDeployByID(ctx, t, env, deploy.ID)
	require.NoError(t, err)

	// The llm_config is stored under the legacy model ID (pre-computed by
	// BuildModelID on the trigger side, matching the public model ID).
	legacyModelID := info.LegacyModelID
	t.Logf("legacy model ID: %s", legacyModelID)

	// Simulate admin enabling the llm_config so the model is visible.
	enableLLMConfig(ctx, t, env, legacyModelID)

	// Read path: GetModelByID uses the legacy model ID (same as llm_config.model_name).
	model, err := env.openaiComp.GetModelByID(ctx, user.UUID, legacyModelID)
	require.NoError(t, err)
	require.NotNil(t, model)

	// The model's ID is info.LegacyModelID, which was pre-computed by
	// BuildModelID on the trigger side: {repo.Name}:{base36(deployID)}.
	expectedModelID := fmt.Sprintf("%s:%s", repo.Name, toBase36(deploy.ID))
	assert.Equal(t, expectedModelID, model.ID, "model ID should match BuildModelID output")
	assert.Equal(t, legacyModelID, model.ID, "model ID should match the llm_config model_name")

	// The model should also appear in GetAvailableModels for the owner.
	models, err := env.openaiComp.GetAvailableModels(ctx, user.UUID)
	require.NoError(t, err)
	var found bool
	for _, m := range models {
		if m.ID == expectedModelID {
			found = true
			break
		}
	}
	assert.True(t, found, "model from sync should appear in GetAvailableModels for owner")
}

// toBase36 converts an int64 to base36 string, matching strconv.FormatInt(n, 36).
func toBase36(n int64) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = digits[n%36]
		n /= 36
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
