package database_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAuditLogStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAuditLogStoreWithDB(db)
	user := database.User{
		UUID: "123e4567-e89b-12d3-a456-426614174000",
	}

	err := store.Create(ctx, &database.AuditLog{
		TableName:  "users",
		Action:     "soft_delete",
		OperatorID: user.UUID,
		Before:     json.RawMessage(`{"id": 1}`),
		After:      json.RawMessage(`{"id": 1, "deleted_at": "2020-01-01T00:00:00Z"}`),
	})
	require.Nil(t, err)
}
