package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func createTestLLMConfigForUpstream(ctx context.Context, t *testing.T, db *database.DB, modelName string) *database.LLMConfig {
	t.Helper()
	cfg := &database.LLMConfig{
		ModelName: modelName,
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
	}
	_, err := db.Core.NewInsert().Model(cfg).Exec(ctx, cfg)
	require.NoError(t, err)
	return cfg
}

func TestUpstreamStore_GetBySourceID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "test-get-by-source-id")

	// No upstream exists initially.
	found, err := store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 99999)
	require.NoError(t, err)
	require.Nil(t, found)

	// Create an upstream with csghub source.
	up := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://internal-svc:8080/v1",
		Weight:      1,
		Enabled:     true,
		ModelName:   "model-a",
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    5001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "namespace/model-a",
				SourceDeployID: 5001,
				SvcName:        "svc-5001",
			},
		},
	}
	err = store.Create(ctx, up)
	require.NoError(t, err)

	// Should find it now.
	found, err = store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 5001)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, up.ID, found.ID)
	require.Equal(t, "http://internal-svc:8080/v1", found.URL)
	require.Equal(t, types.UpstreamSourceCSGHubDeploy, found.Source)
	require.NotNil(t, found.Metadata)
	require.NotNil(t, found.Metadata.InternalModelInfo)
	require.Equal(t, int64(5001), found.Metadata.InternalModelInfo.SourceDeployID)

	// External source should not match the csghub upstream.
	foundExt, err := store.GetBySourceID(ctx, types.UpstreamSourceExternal, 5001)
	require.NoError(t, err)
	require.Nil(t, foundExt)
}

func TestUpstreamStore_UpsertInternalDeployTarget_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "test-upsert-create")

	up := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://upsert-create:8080/v1",
		Weight:      1,
		Enabled:     true,
		ModelName:   "model-upsert-create",
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    7001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "ns/model-upsert-create",
				SourceDeployID: 7001,
			},
		},
	}

	err := store.UpsertInternalDeployTarget(ctx, up)
	require.NoError(t, err)
	require.NotZero(t, up.ID, "ID should be set after create")

	// Verify it was persisted.
	found, err := store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 7001)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "http://upsert-create:8080/v1", found.URL)
	require.True(t, found.Enabled)
}

func TestUpstreamStore_UpsertInternalDeployTarget_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "test-upsert-update")

	// First create.
	original := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://original:8080/v1",
		Weight:      1,
		Enabled:     true,
		ModelName:   "model-original",
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    8001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "ns/model-original",
				SourceDeployID: 8001,
			},
		},
	}
	err := store.UpsertInternalDeployTarget(ctx, original)
	require.NoError(t, err)
	require.NotZero(t, original.ID)

	// Now upsert with updated fields for the same source+source_id.
	updated := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://updated:9090/v1",
		Weight:      1,
		Enabled:     false,
		ModelName:   "model-updated",
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    8001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "ns/model-updated",
				SourceDeployID: 8001,
				SvcName:        "svc-8001",
			},
		},
	}
	err = store.UpsertInternalDeployTarget(ctx, updated)
	require.NoError(t, err)

	// ID should be the same as the original (update, not create).
	require.Equal(t, original.ID, updated.ID, "upsert should update existing row, not create new")

	// Verify the persisted values.
	found, err := store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 8001)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "http://updated:9090/v1", found.URL)
	require.Equal(t, "model-updated", found.ModelName)
	require.False(t, found.Enabled)
	require.NotNil(t, found.Metadata)
	require.Equal(t, "svc-8001", found.Metadata.InternalModelInfo.SvcName)
}

func TestUpstreamStore_UpsertInternalDeployTarget_UpdatePreservesAuthHeader(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "test-upsert-auth-preserve")

	// Create an upstream with an admin-configured auth_header.
	original := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://original:8080/v1",
		Weight:      1,
		Enabled:     true,
		ModelName:   "model-auth",
		AuthHeader:  "Bearer admin-secret-token",
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    9001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "ns/model-auth",
				SourceDeployID: 9001,
			},
		},
	}
	err := store.UpsertInternalDeployTarget(ctx, original)
	require.NoError(t, err)
	require.NotZero(t, original.ID)

	// Verify auth_header was persisted.
	found, err := store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 9001)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "Bearer admin-secret-token", found.AuthHeader)

	// Re-sync with empty auth_header — should NOT overwrite the admin-configured value.
	updated := &database.Upstream{
		LLMConfigID: cfg.ID,
		URL:         "http://updated:9090/v1",
		Weight:      1,
		Enabled:     true,
		ModelName:   "model-auth-updated",
		AuthHeader:  "", // sync always sends empty — admin config should survive.
		Source:      types.UpstreamSourceCSGHubDeploy,
		SourceID:    9001,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID:  "ns/model-auth",
				SourceDeployID: 9001,
				SvcName:        "svc-9001",
			},
		},
	}
	err = store.UpsertInternalDeployTarget(ctx, updated)
	require.NoError(t, err)

	// Verify auth_header was preserved, but URL was updated.
	found, err = store.GetBySourceID(ctx, types.UpstreamSourceCSGHubDeploy, 9001)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "http://updated:9090/v1", found.URL, "URL should be updated")
	require.Equal(t, "model-auth-updated", found.ModelName, "ModelName should be updated")
	require.Equal(t, "Bearer admin-secret-token", found.AuthHeader, "auth_header should be preserved across re-sync")
}

func TestUpstreamStore_ListEnabledBySource(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "test-list-by-source")

	// Create two csghub upstreams (one enabled, one disabled) and one external.
	// Note: Upstream.Enabled has bun tag default:true, so when the Go zero value
	// (false) is inserted, the DB defaults it to true. We must explicitly update
	// the column afterward to set enabled=false.
	up1 := &database.Upstream{
		LLMConfigID: cfg.ID, URL: "http://csghub-1:8080/v1", Weight: 1, Enabled: true,
		ModelName: "m1", Source: types.UpstreamSourceCSGHubDeploy, SourceID: 1001,
	}
	up2 := &database.Upstream{
		LLMConfigID: cfg.ID, URL: "http://csghub-2:8080/v1", Weight: 1,
		ModelName: "m2", Source: types.UpstreamSourceCSGHubDeploy, SourceID: 1002,
	}
	up3 := &database.Upstream{
		LLMConfigID: cfg.ID, URL: "http://ext-1:8080/v1", Weight: 1, Enabled: true,
		ModelName: "m3", Source: types.UpstreamSourceExternal, SourceID: 0,
	}
	for _, u := range []*database.Upstream{up1, up2, up3} {
		require.NoError(t, store.Create(ctx, u))
	}
	// Explicitly disable up2.
	_, err := db.Core.NewUpdate().Model((*database.Upstream)(nil)).
		Set("enabled = false").
		Where("id = ?", up2.ID).
		Exec(ctx)
	require.NoError(t, err)

	// List enabled csghub upstreams — should only return up1.
	result, err := store.ListEnabledBySource(ctx, types.UpstreamSourceCSGHubDeploy)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, up1.ID, result[0].ID)

	// List enabled external upstreams — should only return up3.
	result, err = store.ListEnabledBySource(ctx, types.UpstreamSourceExternal)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, up3.ID, result[0].ID)
}

func TestUpstreamStore_ListEnabledByLogicalModelIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewUpstreamStoreWithDB(db, nil)
	cfg1 := createTestLLMConfigForUpstream(ctx, t, db, "test-list-by-llm-1")
	cfg2 := createTestLLMConfigForUpstream(ctx, t, db, "test-list-by-llm-2")

	up1 := &database.Upstream{
		LLMConfigID: cfg1.ID, URL: "http://list-1:8080/v1", Weight: 1, Enabled: true,
		ModelName: "m1", Source: types.UpstreamSourceExternal,
	}
	up2 := &database.Upstream{
		LLMConfigID: cfg1.ID, URL: "http://list-2:8080/v1", Weight: 1,
		ModelName: "m2", Source: types.UpstreamSourceExternal,
	}
	up3 := &database.Upstream{
		LLMConfigID: cfg2.ID, URL: "http://list-3:8080/v1", Weight: 1, Enabled: true,
		ModelName: "m3", Source: types.UpstreamSourceExternal,
	}
	for _, u := range []*database.Upstream{up1, up2, up3} {
		require.NoError(t, store.Create(ctx, u))
	}
	// Explicitly disable up2 (bun default:true overrides false on insert).
	_, err := db.Core.NewUpdate().Model((*database.Upstream)(nil)).
		Set("enabled = false").
		Where("id = ?", up2.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Query by cfg1.ID and cfg2.ID — should return up1 (enabled) and up3 (enabled), skip up2 (disabled).
	result, err := store.ListEnabledByLogicalModelIDs(ctx, []int64{cfg1.ID, cfg2.ID})
	require.NoError(t, err)
	require.Len(t, result, 2)

	// Empty modelIDs should return nil.
	result, err = store.ListEnabledByLogicalModelIDs(ctx, []int64{})
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestDeployTaskStore_GetDeployByIDWithRelations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewDeployTaskStoreWithDB(db)

	// Create a user first (needed for the User relation).
	user := &database.User{
		UUID:     "test-deploy-rel-user-uuid",
		Username: "test-deploy-rel-user",
		NickName: "test-deploy-rel-user",
		Email:    "test-deploy-rel@example.com",
		Password: "dummy",
		GitID:    100001,
	}
	_, err := db.Core.NewInsert().Model(user).Exec(ctx, user)
	require.NoError(t, err)

	// Create a repository.
	repoStore := database.NewRepoStoreWithDB(db)
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Path:           "namespace/test-deploy-rel-repo",
		GitPath:        "namespace/test-deploy-rel-repo",
		Name:           "test-deploy-rel-repo",
		Nickname:       "test-deploy-rel-repo",
		RepositoryType: types.ModelRepo,
		UserID:         user.ID,
		DefaultBranch:  "main",
	})
	require.NoError(t, err)

	// Create a deploy referencing the user and repo.
	deploy := &database.Deploy{
		SpaceID:   0,
		Status:    1,
		GitPath:   "namespace/test-deploy-rel-repo",
		GitBranch: "main",
		Template:  "default",
		Hardware:  "GPU",
		UserID:    user.ID,
		RepoID:    repo.ID,
		Type:      types.InferenceType,
		SvcName:   "svc-test-rel",
	}
	err = store.CreateDeploy(ctx, deploy)
	require.NoError(t, err)
	require.NotZero(t, deploy.ID)

	// Fetch with relations.
	found, err := store.GetDeployByIDWithRelations(ctx, deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, deploy.ID, found.ID)
	require.NotNil(t, found.Repository, "Repository relation should be loaded")
	require.Equal(t, repo.Path, found.Repository.Path)
	require.NotNil(t, found.User, "User relation should be loaded")
	require.Equal(t, user.Username, found.User.Username)

	// Non-existent deploy should return nil, nil.
	notFound, err := store.GetDeployByIDWithRelations(ctx, 99999999)
	require.NoError(t, err)
	require.Nil(t, notFound)
}
