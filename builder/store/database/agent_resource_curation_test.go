package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentResourceCurationStore_UpsertFields(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentResourceCurationStoreWithDB(db)

	// No row yet: GetByEntity returns nil, nil
	curation, err := store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.Nil(t, curation)

	// Insert with only recommended set
	recommended := true
	err = store.UpsertFields(ctx, "agent_instance", "1", database.AgentResourceCurationFields{Recommended: &recommended})
	require.NoError(t, err)

	curation, err = store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.NotNil(t, curation)
	require.True(t, curation.Recommended)
	require.False(t, curation.Unlisted)
	require.Equal(t, int64(0), curation.Weight)

	// Upsert unlisted only: recommended is preserved
	unlisted := true
	err = store.UpsertFields(ctx, "agent_instance", "1", database.AgentResourceCurationFields{Unlisted: &unlisted})
	require.NoError(t, err)

	curation, err = store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.True(t, curation.Recommended)
	require.True(t, curation.Unlisted)

	// Numeric-looking IDs are normalized ("01" -> "1")
	err = store.UpsertFields(ctx, "agent_instance", "01", database.AgentResourceCurationFields{})
	require.NoError(t, err)
	curation, err = store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.NotNil(t, curation)
}

func TestAgentResourceCurationStore_BatchUpsertWeights(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentResourceCurationStoreWithDB(db)

	// Empty map is a no-op
	require.NoError(t, store.BatchUpsertWeights(ctx, "agent_instance", map[string]int64{}))

	// Seed a row with recommended set, then batch-update weights
	recommended := true
	require.NoError(t, store.UpsertFields(ctx, "agent_instance", "1", database.AgentResourceCurationFields{Recommended: &recommended}))

	err := store.BatchUpsertWeights(ctx, "agent_instance", map[string]int64{"1": 30, "2": 20, "3": 10})
	require.NoError(t, err)

	curation, err := store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.Equal(t, int64(30), curation.Weight)
	// Batch weight upsert must not clobber other fields
	require.True(t, curation.Recommended)

	curation, err = store.GetByEntity(ctx, "agent_instance", "2")
	require.NoError(t, err)
	require.Equal(t, int64(20), curation.Weight)
	require.False(t, curation.Recommended)
}

func TestAgentResourceCurationStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentResourceCurationStoreWithDB(db)

	recommended := true
	require.NoError(t, store.UpsertFields(ctx, "agent_instance", "1", database.AgentResourceCurationFields{Recommended: &recommended}))
	require.NoError(t, store.UpsertFields(ctx, "agent_instance", "2", database.AgentResourceCurationFields{Recommended: &recommended}))
	require.NoError(t, store.UpsertFields(ctx, "agent_template", "1", database.AgentResourceCurationFields{Recommended: &recommended}))

	// DeleteByEntity removes only the matching row
	require.NoError(t, store.DeleteByEntity(ctx, "agent_instance", "1"))
	curation, err := store.GetByEntity(ctx, "agent_instance", "1")
	require.NoError(t, err)
	require.Nil(t, curation)
	curation, err = store.GetByEntity(ctx, "agent_instance", "2")
	require.NoError(t, err)
	require.NotNil(t, curation)

	// DeleteAll with a type removes only that type
	require.NoError(t, store.DeleteAll(ctx, "agent_instance"))
	curation, err = store.GetByEntity(ctx, "agent_instance", "2")
	require.NoError(t, err)
	require.Nil(t, curation)
	curation, err = store.GetByEntity(ctx, "agent_template", "1")
	require.NoError(t, err)
	require.NotNil(t, curation)

	// DeleteAll with empty type resets everything
	require.NoError(t, store.DeleteAll(ctx, ""))
	curation, err = store.GetByEntity(ctx, "agent_template", "1")
	require.NoError(t, err)
	require.Nil(t, curation)
}
