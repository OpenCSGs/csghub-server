package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestActivityLogStore_BatchCreateAndFind(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewActivityLogStoreWithDB(db)

	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	logs := []database.ActivityLog{
		{UserUUID: "uuid1", Username: "user1", Action: "create", ResourceType: "models", ResourceName: "ns/model1", IPAddress: "1.1.1.1", OperationTime: now.Add(-1 * time.Hour)},
		{UserUUID: "uuid2", Username: "user2", Action: "update", ResourceType: "datasets", ResourceName: "ns/ds1", IPAddress: "2.2.2.2", OperationTime: now},
		{UserUUID: "uuid1", Username: "user1", Action: "delete", ResourceType: "models", ResourceName: "ns/model2", IPAddress: "1.1.1.1", OperationTime: now.Add(1 * time.Hour)},
	}

	t.Run("BatchCreate", func(t *testing.T) {
		err := store.BatchCreate(ctx, logs)
		require.NoError(t, err)
	})

	t.Run("FindByTimeAfter_all", func(t *testing.T) {
		results, total, err := store.FindByTimeAfter(ctx, now.Add(-2*time.Hour), 10, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, results, 3)
	})

	t.Run("FindByTimeAfter_filter", func(t *testing.T) {
		results, total, err := store.FindByTimeAfter(ctx, now.Add(-30*time.Minute), 10, 1)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, results, 2)
	})

	t.Run("FindByTimeAfter_pagination", func(t *testing.T) {
		results, total, err := store.FindByTimeAfter(ctx, now.Add(-2*time.Hour), 1, 1)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, results, 1)
	})

	t.Run("FindByTimeAfter_empty", func(t *testing.T) {
		results, _, err := store.FindByTimeAfter(ctx, now.Add(2*time.Hour), 10, 1)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("BatchCreate_empty", func(t *testing.T) {
		err := store.BatchCreate(ctx, nil)
		require.NoError(t, err)
	})
}
