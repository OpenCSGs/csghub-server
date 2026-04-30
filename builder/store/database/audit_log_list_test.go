package database_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAuditLogStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	store := database.NewAuditLogStoreWithDB(db)
	require.NoError(t, store.Create(ctx, &database.AuditLog{
		TableName:   "models",
		Action:      "update",
		OperatorID:  "u1",
		UserName:    "alice",
		BearerToken: "t1",
		AuthType:    "jwt",
		Before:      json.RawMessage(`{"id":1}`),
		After:       json.RawMessage(`{"id":1,"name":"m1"}`),
	}))
	require.NoError(t, store.Create(ctx, &database.AuditLog{
		TableName:   "repositories",
		Action:      "deletion",
		OperatorID:  "u2",
		UserName:    "bob",
		BearerToken: "t2",
		AuthType:    "apikey",
		Before:      json.RawMessage(`{"id":2}`),
		After:       json.RawMessage(`null`),
	}))

	req := types.QueryAuditLogReq{
		UserName:  "ali",
		Token:     "t1",
		Action:    "update",
		TableName: "models",
		AuthType:  "jwt",
		Page:      1,
		Per:       10,
	}
	logs, total, err := store.List(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "alice", logs[0].UserName)

	start := time.Now().AddDate(0, 0, 1)
	req = types.QueryAuditLogReq{
		StartDate: &start,
		Page:      1,
		Per:       10,
	}
	logs, total, err = store.List(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Len(t, logs, 0)
}
