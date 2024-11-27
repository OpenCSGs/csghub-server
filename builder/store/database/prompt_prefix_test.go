package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestPromptPrefixStore_Get(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPromptPrefixStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.PromptPrefix{
		EN: "foo",
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.PromptPrefix{
		EN: "bar",
	}).Exec(ctx)
	require.Nil(t, err)

	prefix, err := store.Get(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", prefix.EN)

}
