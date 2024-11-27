package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestResourceModelStore_All(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewResourceModelStoreWithDB(db)
	_, err := db.Core.NewInsert().Model(&database.ResourceModel{
		ModelName: "foo",
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.ResourceModel{
		ModelName: "bar",
	}).Exec(ctx)
	require.Nil(t, err)

	ms, err := store.FindByModelName(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, "foo", ms[0].ModelName)

	_, err = db.Core.NewInsert().Model(&database.RepositoriesRuntimeFramework{
		RepoID: 123,
	}).Exec(ctx)
	require.Nil(t, err)

	m, err := store.CheckModelNameNotInRFRepo(ctx, "foo", 456)
	require.Nil(t, err)
	require.Equal(t, "foo", m.ModelName)

	m, err = store.CheckModelNameNotInRFRepo(ctx, "foo", 123)
	require.Nil(t, err)
	require.Nil(t, m)

}
