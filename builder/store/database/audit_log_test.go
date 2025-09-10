package database_test

import (
	"context"
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
		ID: 1,
	}

	err := store.Create(ctx, &database.AuditLog{
		TableName:  "users",
		Action:     "soft_delete",
		OperatorID: user.ID,
		Before:     `{"id": 1}`,
		After:      `{"id": 1, "deleted_at": "2020-01-01T00:00:00Z"}`,
	})
	require.Nil(t, err)
}
