package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
)

func newLogicalAgentTestInstance(owner, contentID string) *database.AgentInstance {
	mode := database.AgentServingModeReadOnly
	storagePolicy := database.AgentStoragePolicyNone
	return &database.AgentInstance{
		UserUUID: owner, Type: "csgclaw", ContentID: contentID, Name: contentID,
		Kind: database.AgentInstanceKindLogical, Metadata: map[string]any{},
		EffectiveMode: &mode, StoragePolicy: &storagePolicy,
		Blueprint: map[string]any{"image_id": "image:tag"}, BlueprintSHA256: "blueprint-hash",
	}
}

func createPublicLogicalAgent(t *testing.T, ctx context.Context, db *database.DB, owner, contentID string) *database.AgentInstance {
	t.Helper()
	store := database.NewAgentCatalogStoreWithDB(db)
	logical := newLogicalAgentTestInstance(owner, contentID)
	logical.Public = true
	created, err := store.CreateLogicalAgent(ctx, logical)
	require.NoError(t, err)
	return created
}

func TestAgentCatalogStoreCreateLogical(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	instance := newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString())

	created, err := store.CreateLogicalAgent(ctx, instance)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Equal(t, database.AgentInstanceKindLogical, created.Kind)
	require.Equal(t, database.AgentServingModeReadOnly, *created.EffectiveMode)
	require.Equal(t, database.AgentStoragePolicyNone, *created.StoragePolicy)
	require.Equal(t, "blueprint-hash", created.BlueprintSHA256)
}

func TestAgentCatalogStoreUpdateLogicalIsLastWriteWins(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := store.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)

	public := true
	updated, err := store.UpdateLogical(ctx, database.UpdateLogicalAgentInput{
		LogicalInstanceID: logical.ID, OwnerUUID: owner, Public: &public,
	})
	require.NoError(t, err)
	require.True(t, updated.Public)

	updated, err = store.UpdateLogical(ctx, database.UpdateLogicalAgentInput{
		LogicalInstanceID: logical.ID, OwnerUUID: owner, Public: &public,
	})
	require.NoError(t, err)
	require.True(t, updated.Public)
}

func TestAgentCatalogStoreRejectsUpdateAfterDeletionStarts(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := store.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)
	deletionStartedAt := time.Now()
	_, err = db.Core.NewUpdate().Model(logical).Set("deletion_started_at = ?", deletionStartedAt).WherePK().Exec(ctx)
	require.NoError(t, err)

	public := true
	_, err = store.UpdateLogical(ctx, database.UpdateLogicalAgentInput{
		LogicalInstanceID: logical.ID, OwnerUUID: owner, Public: &public,
	})

	require.ErrorIs(t, err, errorx.ErrBadRequest)
}

func TestAgentOrchestratorStoreDeleteLogicalWithoutAllocationsIsIdempotent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	catalogStore := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := catalogStore.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)

	result, err := store.BeginLogicalAgentDeletion(ctx, logical.ID, owner)
	require.NoError(t, err)
	require.False(t, result.Pending)
	_, err = store.BeginLogicalAgentDeletion(ctx, logical.ID, owner)
	require.NoError(t, err)
}

func TestAgentOrchestratorStoreDeleteLogicalRepairsMissingTombstone(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentOrchestratorStoreWithDB(db)
	catalogStore := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := catalogStore.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)

	// Simulate a stuck deletion: fence set, allocations already gone, no tombstone.
	deletionStartedAt := time.Now()
	_, err = db.Core.NewUpdate().Model(logical).
		Set("deletion_started_at = ?", deletionStartedAt).Set("public = FALSE").WherePK().Exec(ctx)
	require.NoError(t, err)

	result, err := store.BeginLogicalAgentDeletion(ctx, logical.ID, owner)
	require.NoError(t, err)
	require.False(t, result.Pending)
	require.False(t, result.Logical.DeletedAt.IsZero())

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	_, err = instanceStore.FindByID(ctx, logical.ID)
	require.Error(t, err)
}

func TestAgentInstanceStoreFindByIDWithDeletedReturnsTombstonedLogicalAgent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	orchestratorStore := database.NewAgentOrchestratorStoreWithDB(db)
	catalogStore := database.NewAgentCatalogStoreWithDB(db)
	owner := uuid.NewString()
	logical, err := catalogStore.CreateLogicalAgent(ctx, newLogicalAgentTestInstance(owner, "logical-"+uuid.NewString()))
	require.NoError(t, err)

	_, err = orchestratorStore.BeginLogicalAgentDeletion(ctx, logical.ID, owner)
	require.NoError(t, err)

	_, err = instanceStore.FindByID(ctx, logical.ID)
	require.Error(t, err)
	tombstoned, err := instanceStore.FindByIDWithDeleted(ctx, logical.ID)
	require.NoError(t, err)
	require.Equal(t, logical.ID, tombstoned.ID)
	require.Equal(t, database.AgentInstanceKindLogical, tombstoned.Kind)
	require.False(t, tombstoned.DeletedAt.IsZero())
}
