package upstream

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockupstream "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component/upstream"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func newSyncComponent(t *testing.T) (
	*aiGatewayUpstreamSyncComponentImpl,
	*mockdb.MockDeployTaskStore,
	*mockdb.MockUpstreamStore,
	*mockdb.MockLLMConfigStore,
	*mockupstream.MockModelIDBuilder,
) {
	t.Helper()
	mockDeploy := mockdb.NewMockDeployTaskStore(t)
	mockUpstream := mockdb.NewMockUpstreamStore(t)
	mockLLMConfig := mockdb.NewMockLLMConfigStore(t)
	mockModelID := mockupstream.NewMockModelIDBuilder(t)

	comp := NewAIGatewayUpstreamSyncComponent(AIGatewayUpstreamSyncComponentConfig{
		DeployStore:    mockDeploy,
		UpstreamStore:  mockUpstream,
		LLMConfigStore: mockLLMConfig,
		// ClusterStore is nil — sync falls back to using deploy.Endpoint directly.
		ModelIDBuilder: mockModelID,
	}).(*aiGatewayUpstreamSyncComponentImpl)

	return comp, mockDeploy, mockUpstream, mockLLMConfig, mockModelID
}

// testInfo builds a DeployUpstreamInfo for use in tests.
func testInfo(deployID int64, repoPath, svcName string) *commontypes.DeployUpstreamInfo {
	return &commontypes.DeployUpstreamInfo{
		DeployID:         deployID,
		RepoPath:         repoPath,
		RepoName:         repoPath,
		DeployType:       commontypes.InferenceType,
		Provider:         commontypes.ProviderTypeInference,
		Endpoint:         "http://internal:8080/v1",
		ClusterID:        "cluster-1",
		SvcName:          svcName,
		ImageID:          "img-" + svcName,
		RuntimeFramework: "vllm",
		UserUUID:         "user-uuid-1",
		OwnerUsername:    "user1",
		OwnerNamespace:   "user1",
		OwnerType:        string(database.UserNamespace),
		CreatedAt:        1700000000,
		// LegacyModelID matches the public ID format: {repoPath}:{base36(deployID)}
		LegacyModelID: repoPath + ":" + strconv.FormatInt(deployID, 36),
	}
}

// =====================================================================
// Comment 3: existing upstream + valid llm_config → reuse, update in place
// =====================================================================

func TestSyncRunningDeploy_ExistingUpstreamWithValidLLMConfig_ReusesLLMConfigID(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(100, "ns/model-a", "svc-100")

	// Step 1: upstream already exists.
	existing := &database.Upstream{
		ID:          999,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    100,
		Enabled:     true,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(100)).Return(existing, nil)

	// llm_config still exists → reuse, no GetByModelName or Create expected.
	mockLLMConfig.EXPECT().GetByID(ctx, int64(42)).Return(&database.LLMConfig{ID: 42, ModelName: info.LegacyModelID}, nil)

	// Update the upstream in place, keeping LLMConfigID=42.
	mockUpstream.EXPECT().Update(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.ID == 999 &&
			u.LLMConfigID == 42 &&
			u.Enabled &&
			u.URL == "http://internal:8080/v1" &&
			u.ModelName == "ns/model-a" &&
			u.Provider == commontypes.ProviderTypeInference &&
			u.Metadata != nil &&
			u.Metadata.InternalModelInfo != nil &&
			u.Metadata.InternalModelInfo.CSGHubModelID == "ns/model-a" &&
			u.Metadata.InternalModelInfo.SourceDeployID == 100
	})).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

// =====================================================================
// Re-sync must preserve admin-configured metadata (ResponsesChatAdapter,
// Protocol) on an existing csghub upstream — only sync-owned fields
// (InternalModelInfo) are rebuilt.
// =====================================================================
func TestSyncRunningDeploy_ExistingUpstream_PreservesAdminConfiguredMetadata(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(100, "ns/model-a", "svc-100")

	adapter := &commontypes.ResponsesChatAdapter{
		ReasoningRequest: &commontypes.ReasoningRequestConfig{
			Enabled:     true,
			EffortField: "reasoning_effort",
		},
	}
	existing := &database.Upstream{
		ID:          999,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    100,
		Enabled:     true,
		Metadata: &commontypes.UpstreamMetadata{
			ResponsesChatAdapter: adapter,
			Protocol:             "responses",
		},
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(100)).Return(existing, nil)
	mockLLMConfig.EXPECT().GetByID(ctx, int64(42)).Return(&database.LLMConfig{ID: 42, ModelName: info.LegacyModelID}, nil)

	mockUpstream.EXPECT().Update(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		if u.Metadata == nil || u.Metadata.InternalModelInfo == nil {
			return false
		}
		// Sync-owned field rebuilt from current deploy info.
		if u.Metadata.InternalModelInfo.CSGHubModelID != "ns/model-a" ||
			u.Metadata.InternalModelInfo.SourceDeployID != 100 {
			return false
		}
		// Admin-configured fields preserved across re-sync.
		return u.Metadata.ResponsesChatAdapter == adapter &&
			u.Metadata.Protocol == "responses"
	})).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

// =====================================================================
// Comment 3: existing upstream + deleted llm_config → delete upstream, recreate
// =====================================================================

func TestSyncRunningDeploy_ExistingUpstreamWithDeletedLLMConfig_DeletesAndRecreates(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(200, "ns/model-b", "svc-200")

	// Step 1: upstream exists but its llm_config was deleted.
	existing := &database.Upstream{
		ID:          888,
		LLMConfigID: 77,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    200,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(200)).Return(existing, nil)
	mockLLMConfig.EXPECT().GetByID(ctx, int64(77)).Return(nil, nil)

	// Delete the orphaned upstream.
	mockUpstream.EXPECT().Delete(ctx, int64(888)).Return(nil)

	// Step 2: find or create llm_config.
	mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(nil, nil)
	mockLLMConfig.EXPECT().Create(ctx, mock.MatchedBy(func(c database.LLMConfig) bool {
		return c.ModelName == info.LegacyModelID &&
			c.Type == database.LLMTypeAigatewayExternal &&
			!c.NeedSensitiveCheck
	})).Return(&database.LLMConfig{ID: 99}, nil)

	// Upsert new upstream.
	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.SourceID == 200 && u.LLMConfigID == 99
	})).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

// =====================================================================
// No existing upstream → find/create llm_config, upsert
// =====================================================================

func TestSyncRunningDeploy_NoExistingUpstream_CreateNewLLMConfigAndUpstream(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(100, "ns/model-a", "svc-100")

	// Step 1: no upstream exists.
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(100)).Return(nil, nil)

	// Step 2: llm_config does not exist → create.
	mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(nil, nil)
	mockLLMConfig.EXPECT().Create(ctx, mock.MatchedBy(func(c database.LLMConfig) bool {
		return c.ModelName == info.LegacyModelID &&
			c.Type == database.LLMTypeAigatewayExternal &&
			!c.NeedSensitiveCheck
	})).Return(&database.LLMConfig{ID: 42}, nil)

	// Upsert upstream.
	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.Source == commontypes.UpstreamSourceCSGHubDeploy &&
			u.SourceID == 100 &&
			u.LLMConfigID == 42 &&
			u.URL == "http://internal:8080/v1" &&
			u.ModelName == "ns/model-a" &&
			u.Provider == commontypes.ProviderTypeInference &&
			u.Metadata != nil &&
			u.Metadata.InternalModelInfo != nil &&
			u.Metadata.InternalModelInfo.CSGHubModelID == "ns/model-a" &&
			u.Metadata.InternalModelInfo.LegacyModelID == info.LegacyModelID &&
			u.Metadata.InternalModelInfo.OwnerUUID == "user-uuid-1" &&
			u.Metadata.InternalModelInfo.OwnerUsername == "user1" &&
			u.Metadata.InternalModelInfo.SourceDeployID == 100
	})).Run(func(ctx context.Context, u *database.Upstream) {
		u.ID = 999
	}).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

func TestSyncRunningDeploy_NoExistingUpstream_ReuseExistingLLMConfig(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(200, "ns/model-b", "svc-200")

	// No upstream exists.
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(200)).Return(nil, nil)

	// llm_config already exists — reuse it, no Create call expected.
	mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(&database.LLMConfig{ID: 77}, nil)

	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.LLMConfigID == 77 && u.SourceID == 200
	})).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

// Verify that a newly created llm_config is always enabled, regardless of
// deploy type. The deploy is a user-owned resource — a disabled llm_config
// would make the user's own inference endpoint unreachable until an admin
// intervenes. Admins can still disable it later via the admin API.
func TestSyncRunningDeploy_NewLLMConfigEnabledByDefault(t *testing.T) {
	deployTypes := []int{
		commontypes.InferenceType,
		commontypes.ServerlessType,
	}

	for _, deployType := range deployTypes {
		t.Run(strconv.Itoa(deployType), func(t *testing.T) {
			comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
			ctx := context.TODO()
			info := testInfo(300, "ns/model-d", "svc-300")
			info.DeployType = deployType

			// No upstream exists.
			mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(300)).Return(nil, nil)

			// llm_config does not exist → create with Enabled=true.
			mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(nil, nil)
			mockLLMConfig.EXPECT().Create(ctx, mock.MatchedBy(func(c database.LLMConfig) bool {
				return c.ModelName == info.LegacyModelID &&
					c.Type == database.LLMTypeAigatewayExternal &&
					c.Enabled && // must be enabled — user-owned inference must be accessible
					!c.NeedSensitiveCheck
			})).Return(&database.LLMConfig{ID: 55}, nil)

			// Upstream should be enabled as well.
			mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
				return u.LLMConfigID == 55 && u.SourceID == 300 && u.Enabled
			})).Return(nil)

			err := comp.SyncRunningDeploy(ctx, info)
			require.NoError(t, err)
		})
	}
}

// Verify that reusing an existing llm_config does not change its Enabled state.
// The sync should only create upstreams, not toggle llm_config.enabled.
func TestSyncRunningDeploy_DoesNotModifyExistingLLMConfigEnabled(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(400, "ns/model-e", "svc-400")

	// No upstream exists.
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(400)).Return(nil, nil)

	// An existing llm_config that an admin has already enabled — no Create
	// call should be made, and its Enabled state must not be touched.
	mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(&database.LLMConfig{
		ID:      88,
		Enabled: true,
	}, nil)

	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.LLMConfigID == 88 && u.SourceID == 400
	})).Return(nil)

	err := comp.SyncRunningDeploy(ctx, info)
	require.NoError(t, err)
}

func TestSyncRunningDeploy_NilInfo(t *testing.T) {
	comp, _, _, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	err := comp.SyncRunningDeploy(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil info")
}

func TestSyncRunningDeploy_GetBySourceIDError(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(500, "ns/model-x", "svc-500")

	dbErr := errors.New("connection refused")
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(500)).Return(nil, dbErr)

	err := comp.SyncRunningDeploy(ctx, info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestSyncRunningDeploy_GetByModelNameError(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()
	info := testInfo(500, "ns/model-x", "svc-500")

	// No upstream exists.
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(500)).Return(nil, nil)

	dbErr := errors.New("connection refused")
	mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(nil, dbErr)

	err := comp.SyncRunningDeploy(ctx, info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// Verify that the deploy's SecureLevel is persisted on the upstream's
// InternalModelInfo, both when the upstream is created and when an existing
// upstream is updated in place (e.g. after a DeployUpdate changes
// secure_level from private to public).
func TestSyncRunningDeploy_PersistsSecureLevel(t *testing.T) {
	ctx := context.TODO()

	t.Run("create path stores secure level in upstream metadata", func(t *testing.T) {
		comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
		info := testInfo(600, "ns/model-f", "svc-600")
		info.SecureLevel = commontypes.EndpointPrivate

		mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(nil, nil)
		mockLLMConfig.EXPECT().GetByModelName(ctx, info.LegacyModelID).Return(nil, nil)
		mockLLMConfig.EXPECT().Create(ctx, mock.Anything).Return(&database.LLMConfig{ID: 61}, nil)

		mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
			return u.SourceID == 600 &&
				u.Metadata != nil &&
				u.Metadata.InternalModelInfo != nil &&
				u.Metadata.InternalModelInfo.SecureLevel == commontypes.EndpointPrivate
		})).Return(nil)

		err := comp.SyncRunningDeploy(ctx, info)
		require.NoError(t, err)
	})

	t.Run("update-in-place path refreshes secure level in upstream metadata", func(t *testing.T) {
		comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
		info := testInfo(100, "ns/model-a", "svc-100")
		info.SecureLevel = commontypes.EndpointPublic

		existing := &database.Upstream{
			ID:          999,
			LLMConfigID: 42,
			Source:      commontypes.UpstreamSourceCSGHubDeploy,
			SourceID:    100,
			Enabled:     true,
		}
		mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(100)).Return(existing, nil)
		mockLLMConfig.EXPECT().GetByID(ctx, int64(42)).Return(&database.LLMConfig{ID: 42, ModelName: info.LegacyModelID}, nil)

		mockUpstream.EXPECT().Update(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
			return u.ID == 999 &&
				u.Metadata != nil &&
				u.Metadata.InternalModelInfo != nil &&
				u.Metadata.InternalModelInfo.SourceDeployID == 100 &&
				u.Metadata.InternalModelInfo.SecureLevel == commontypes.EndpointPublic
		})).Return(nil)

		err := comp.SyncRunningDeploy(ctx, info)
		require.NoError(t, err)
	})
}

// =====================================================================
// DisableDeployTarget / DeleteDeployTarget
// =====================================================================

func TestDisableDeployTarget_Success(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{ID: 501, Source: commontypes.UpstreamSourceCSGHubDeploy, SourceID: 500, Enabled: true}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(500)).Return(existing, nil)
	mockUpstream.EXPECT().Update(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.ID == 501 && !u.Enabled
	})).Return(nil)

	err := comp.DisableDeployTarget(ctx, 500)
	require.NoError(t, err)
}

func TestDisableDeployTarget_NotFound(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(404)).Return(nil, nil)

	err := comp.DisableDeployTarget(ctx, 404)
	require.NoError(t, err, "should skip silently when upstream not found")
}

func TestDeleteDeployTarget_Success(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{ID: 601, Source: commontypes.UpstreamSourceCSGHubDeploy, SourceID: 600}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(existing, nil)
	mockUpstream.EXPECT().Delete(ctx, int64(601)).Return(nil)

	err := comp.DeleteDeployTarget(ctx, 600)
	require.NoError(t, err)
}

// After deleting the deploy's upstream, an llm_config left with zero
// upstreams is deleted as well so no unroutable orphan configs accumulate.
func TestDeleteDeployTarget_DeletesEmptyLLMConfig(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{
		ID:          601,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    600,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(existing, nil)
	mockUpstream.EXPECT().Delete(ctx, int64(601)).Return(nil)
	// No upstreams remain under the llm_config → delete it.
	mockUpstream.EXPECT().ListByLLMConfigID(ctx, int64(42)).Return([]*database.Upstream{}, nil)
	mockLLMConfig.EXPECT().Delete(ctx, int64(42)).Return(nil)

	err := comp.DeleteDeployTarget(ctx, 600)
	require.NoError(t, err)
}

// An llm_config that still has other upstreams (e.g. admin-attached external
// endpoints) must be kept.
func TestDeleteDeployTarget_KeepsLLMConfigWithRemainingUpstreams(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{
		ID:          601,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    600,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(existing, nil)
	mockUpstream.EXPECT().Delete(ctx, int64(601)).Return(nil)
	mockUpstream.EXPECT().ListByLLMConfigID(ctx, int64(42)).Return([]*database.Upstream{
		{ID: 602, LLMConfigID: 42, Source: commontypes.UpstreamSourceExternal},
	}, nil)
	// No llmConfigStore.Delete expectation — the strict mock fails the test
	// if the llm_config is wrongly deleted.

	err := comp.DeleteDeployTarget(ctx, 600)
	require.NoError(t, err)
}

func TestDeleteDeployTarget_NotFound(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(404)).Return(nil, nil)

	err := comp.DeleteDeployTarget(ctx, 404)
	require.NoError(t, err, "should skip silently when upstream not found")
}

func TestDeleteDeployTarget_ListRemainingError(t *testing.T) {
	comp, _, mockUpstream, _, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{
		ID:          601,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    600,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(existing, nil)
	mockUpstream.EXPECT().Delete(ctx, int64(601)).Return(nil)
	mockUpstream.EXPECT().ListByLLMConfigID(ctx, int64(42)).Return(nil, errors.New("connection refused"))

	err := comp.DeleteDeployTarget(ctx, 600)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestDeleteDeployTarget_LLMConfigDeleteError(t *testing.T) {
	comp, _, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()

	existing := &database.Upstream{
		ID:          601,
		LLMConfigID: 42,
		Source:      commontypes.UpstreamSourceCSGHubDeploy,
		SourceID:    600,
	}
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(600)).Return(existing, nil)
	mockUpstream.EXPECT().Delete(ctx, int64(601)).Return(nil)
	mockUpstream.EXPECT().ListByLLMConfigID(ctx, int64(42)).Return([]*database.Upstream{}, nil)
	mockLLMConfig.EXPECT().Delete(ctx, int64(42)).Return(errors.New("fk constraint"))

	err := comp.DeleteDeployTarget(ctx, 600)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fk constraint")
}

// =====================================================================
// ReconcileUpstreams
// =====================================================================

func TestReconcileUpstreams_SyncsRunningAndDisablesGhosts(t *testing.T) {
	comp, mockDeploy, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()

	// Two running deploys.
	runningDeploys := []database.Deploy{
		{ID: 1, SvcName: "svc-1", Endpoint: "http://svc-1:8080/v1"},
		{ID: 2, SvcName: "svc-2", Endpoint: "http://svc-2:8080/v1"},
	}
	mockDeploy.EXPECT().ListRunningDeploysByTypes(ctx, []int{commontypes.ServerlessType, commontypes.InferenceType}).Return(runningDeploys, nil)

	// SyncRunningDeploy for deploy 1: load relations → no upstream → create.
	deploy1Full := &database.Deploy{
		ID: 1, SvcName: "svc-1", Endpoint: "http://svc-1:8080/v1", Type: commontypes.InferenceType,
		Repository: &database.Repository{Path: "ns/model-1", Name: "model-1"},
		User:       &database.User{UUID: "uuid-1"},
	}
	mockDeploy.EXPECT().GetDeployByIDWithRelations(ctx, int64(1)).Return(deploy1Full, nil)
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(1)).Return(nil, nil)
	mockLLMConfig.EXPECT().GetByModelName(ctx, "model-1:1").Return(nil, nil)
	mockLLMConfig.EXPECT().Create(ctx, mock.Anything).Return(&database.LLMConfig{ID: 11}, nil)
	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.Anything).Return(nil)

	// SyncRunningDeploy for deploy 2: load relations → no upstream → create.
	deploy2Full := &database.Deploy{
		ID: 2, SvcName: "svc-2", Endpoint: "http://svc-2:8080/v1", Type: commontypes.InferenceType,
		Repository: &database.Repository{Path: "ns/model-2", Name: "model-2"},
		User:       &database.User{UUID: "uuid-2"},
	}
	mockDeploy.EXPECT().GetDeployByIDWithRelations(ctx, int64(2)).Return(deploy2Full, nil)
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(2)).Return(nil, nil)
	mockLLMConfig.EXPECT().GetByModelName(ctx, "model-2:2").Return(nil, nil)
	mockLLMConfig.EXPECT().Create(ctx, mock.Anything).Return(&database.LLMConfig{ID: 22}, nil)
	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.Anything).Return(nil)

	// ListEnabledBySource: upstream for deploy 1 (running), upstream for deploy 3 (ghost).
	csghubUpstreams := []*database.Upstream{
		{ID: 101, Source: commontypes.UpstreamSourceCSGHubDeploy, SourceID: 1, Enabled: true},
		{ID: 102, Source: commontypes.UpstreamSourceCSGHubDeploy, SourceID: 3, Enabled: true},
	}
	mockUpstream.EXPECT().ListEnabledBySource(ctx, commontypes.UpstreamSourceCSGHubDeploy).Return(csghubUpstreams, nil)

	// Deploy 3 is not in running set → disable.
	mockUpstream.EXPECT().Update(ctx, mock.MatchedBy(func(u *database.Upstream) bool {
		return u.ID == 102 && !u.Enabled
	})).Return(nil)

	err := comp.ReconcileUpstreams(ctx)
	require.NoError(t, err)
}

func TestReconcileUpstreams_NoGhosts(t *testing.T) {
	comp, mockDeploy, mockUpstream, mockLLMConfig, _ := newSyncComponent(t)
	ctx := context.TODO()

	runningDeploys := []database.Deploy{
		{ID: 10, SvcName: "svc-10", Endpoint: "http://svc-10:8080/v1"},
	}
	mockDeploy.EXPECT().ListRunningDeploysByTypes(ctx, []int{commontypes.ServerlessType, commontypes.InferenceType}).Return(runningDeploys, nil)

	deployFull := &database.Deploy{
		ID: 10, SvcName: "svc-10", Endpoint: "http://svc-10:8080/v1", Type: commontypes.InferenceType,
		Repository: &database.Repository{Path: "ns/m10", Name: "m10"},
		User:       &database.User{UUID: "uuid-10"},
	}
	mockDeploy.EXPECT().GetDeployByIDWithRelations(ctx, int64(10)).Return(deployFull, nil)
	mockUpstream.EXPECT().GetBySourceID(ctx, commontypes.UpstreamSourceCSGHubDeploy, int64(10)).Return(nil, nil)
	mockLLMConfig.EXPECT().GetByModelName(ctx, "m10:a").Return(&database.LLMConfig{ID: 5}, nil)
	mockUpstream.EXPECT().UpsertInternalDeployTarget(ctx, mock.Anything).Return(nil)

	mockUpstream.EXPECT().ListEnabledBySource(ctx, commontypes.UpstreamSourceCSGHubDeploy).Return([]*database.Upstream{
		{ID: 201, Source: commontypes.UpstreamSourceCSGHubDeploy, SourceID: 10, Enabled: true},
	}, nil)

	err := comp.ReconcileUpstreams(ctx)
	require.NoError(t, err)
}

func TestComputeURLAndHost(t *testing.T) {
	// useClusterStore controls whether a mock cluster store is set on the
	// component. When false, the field is left as a true nil interface (not
	// a nil typed pointer) so the fallback path is exercised.
	tests := []struct {
		name            string
		endpoint        string
		appEndpoint     string
		useClusterStore bool
		wantURL         string
		wantHost        string
	}{
		{
			name:            "bare AppEndpoint, deploy endpoint with /v1 path",
			endpoint:        "http://svc.default.svc.cluster.local:8080/v1",
			appEndpoint:     "http://127.0.0.1:9099",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099/v1",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "bare AppEndpoint, deploy endpoint with no path (e.g. Stable Diffusion)",
			endpoint:        "http://svc.default.svc.cluster.local:8080",
			appEndpoint:     "http://127.0.0.1:9099",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "bare AppEndpoint, deploy endpoint with custom path /api/v2",
			endpoint:        "http://svc.default.svc.cluster.local:8080/api/v2",
			appEndpoint:     "http://127.0.0.1:9099",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099/api/v2",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "AppEndpoint with path /proxy, deploy endpoint with /v1 path — paths joined",
			endpoint:        "http://svc.default.svc.cluster.local:8080/v1",
			appEndpoint:     "http://127.0.0.1:9099/proxy",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099/proxy/v1",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "AppEndpoint with path /proxy, deploy endpoint with no path — AppEndpoint path preserved",
			endpoint:        "http://svc.default.svc.cluster.local:8080",
			appEndpoint:     "http://127.0.0.1:9099/proxy",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099/proxy",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "AppEndpoint with trailing slash, deploy endpoint with /v1 path",
			endpoint:        "http://svc.default.svc.cluster.local:8080/v1",
			appEndpoint:     "http://127.0.0.1:9099/",
			useClusterStore: true,
			wantURL:         "http://127.0.0.1:9099/v1",
			wantHost:        "svc.default.svc.cluster.local",
		},
		{
			name:            "fallback (nil clusterStore) uses deploy endpoint as-is with path",
			endpoint:        "http://svc.default.svc.cluster.local:8080/v1",
			appEndpoint:     "",
			useClusterStore: false,
			wantURL:         "http://svc.default.svc.cluster.local:8080/v1",
			wantHost:        "",
		},
		{
			name:            "fallback (nil clusterStore) uses deploy endpoint as-is without path",
			endpoint:        "http://svc.default.svc.cluster.local:8080",
			appEndpoint:     "",
			useClusterStore: false,
			wantURL:         "http://svc.default.svc.cluster.local:8080",
			wantHost:        "",
		},
		{
			name:            "empty endpoint with nil clusterStore returns empty",
			endpoint:        "",
			appEndpoint:     "",
			useClusterStore: false,
			wantURL:         "",
			wantHost:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comp := &aiGatewayUpstreamSyncComponentImpl{}
			info := &commontypes.DeployUpstreamInfo{
				DeployID:  1,
				Endpoint:  tt.endpoint,
				ClusterID: "cluster-1",
				SvcName:   "svc",
			}
			if tt.useClusterStore {
				mockCluster := mockdb.NewMockClusterInfoStore(t)
				mockCluster.EXPECT().ByClusterID(mock.Anything, "cluster-1").
					Return(database.ClusterInfo{
						ClusterID:   "cluster-1",
						AppEndpoint: tt.appEndpoint,
					}, nil)
				comp.clusterStore = mockCluster
			}
			gotURL, gotHost := comp.computeURLAndHost(context.TODO(), info)
			assert.Equal(t, tt.wantURL, gotURL)
			assert.Equal(t, tt.wantHost, gotHost)
		})
	}
}

func TestComputeURLAndHost_ClusterLookupError_FallsBackToEndpoint(t *testing.T) {
	mockCluster := mockdb.NewMockClusterInfoStore(t)
	comp := &aiGatewayUpstreamSyncComponentImpl{
		clusterStore: mockCluster,
	}

	info := &commontypes.DeployUpstreamInfo{
		DeployID:  1,
		Endpoint:  "http://svc.default.svc.cluster.local:8080/v1",
		ClusterID: "cluster-bad",
		SvcName:   "svc",
	}

	mockCluster.EXPECT().ByClusterID(mock.Anything, "cluster-bad").
		Return(database.ClusterInfo{}, errors.New("cluster not found"))

	url, host := comp.computeURLAndHost(context.TODO(), info)
	// Falls back to deploy endpoint as-is, preserving its path
	assert.Equal(t, "http://svc.default.svc.cluster.local:8080/v1", url)
	assert.Equal(t, "", host)
}
