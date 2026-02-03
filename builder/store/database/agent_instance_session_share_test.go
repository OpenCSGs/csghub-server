package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentInstanceSessionShareStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionShareStoreWithDB(db)

	shareUUID := uuid.New().String()
	sessionUUID := uuid.New().String()
	userUUID := uuid.New().String()
	instanceID := int64(123)
	expiresAt := time.Now().Add(2 * time.Hour).Unix()

	created, err := store.Create(ctx, &database.AgentInstanceSessionShare{
		ShareUUID:   shareUUID,
		InstanceID:  instanceID,
		SessionUUID: sessionUUID,
		MaxTurn:     7,
		UserUUID:    userUUID,
		ExpiresAt:   expiresAt,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotZero(t, created.ID)
	require.Equal(t, shareUUID, created.ShareUUID)
	require.Equal(t, instanceID, created.InstanceID)
	require.Equal(t, sessionUUID, created.SessionUUID)
	require.Equal(t, int64(7), created.MaxTurn)
	require.Equal(t, userUUID, created.UserUUID)
	require.Equal(t, expiresAt, created.ExpiresAt)
	require.False(t, created.CreatedAt.IsZero())
	require.False(t, created.UpdatedAt.IsZero())

	found, err := store.FindByShareUUID(ctx, shareUUID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, shareUUID, found.ShareUUID)
	require.Equal(t, instanceID, found.InstanceID)
	require.Equal(t, sessionUUID, found.SessionUUID)
	require.Equal(t, int64(7), found.MaxTurn)
	require.Equal(t, userUUID, found.UserUUID)
	require.Equal(t, expiresAt, found.ExpiresAt)
}
