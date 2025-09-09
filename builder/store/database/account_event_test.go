package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAccountEventStore_CreateGet(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	as := database.NewAccountEventStoreWithDB(db)

	uid := uuid.New()
	err := as.Create(ctx, database.AccountEvent{
		EventUUID: uid,
		EventBody: map[string]string{
			"a": "b",
			"c": "d",
		},
		Duplicated: false,
	})
	require.Nil(t, err)

	event, err := as.GetByEventID(ctx, uid)
	require.Nil(t, err)

	require.Equal(t, "b", event.EventBody["a"])
	require.Equal(t, "d", event.EventBody["c"])
	require.False(t, event.Duplicated)
}
