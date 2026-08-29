package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentShareStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAgentShareStoreWithDB(db)

	share := &database.AgentShare{
		ShareUUID:  uuid.NewString(),
		ShareName:  "s-" + uuid.NewString()[:16],
		Type:       "instance",
		UserUUID:   uuid.NewString(),
		InstanceID: 123,
	}
	created, err := store.Create(ctx, share)
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	byUUID, err := store.FindByShareUUID(ctx, share.ShareUUID)
	require.NoError(t, err)
	require.Equal(t, created.ID, byUUID.ID)
	require.Equal(t, share.ShareName, byUUID.ShareName)

	byName, err := store.FindByShareName(ctx, share.ShareName)
	require.NoError(t, err)
	require.Equal(t, created.ID, byName.ID)
	require.Equal(t, share.ShareUUID, byName.ShareUUID)
}
