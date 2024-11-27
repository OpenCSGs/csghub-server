package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestTelemetryStore_Save(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewTelemetryStoreWithDB(db)
	err := store.Save(ctx, &database.Telemetry{
		UUID: "foo",
	})
	require.Nil(t, err)

}
