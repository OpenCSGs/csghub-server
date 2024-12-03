package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestTagRuleStore_FindByRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewTagRuleStoreWithDB(db)

	_, err := db.Core.NewInsert().Model(&database.TagRule{
		Category:  "foo",
		Namespace: "test",
		RepoName:  "bar",
		RepoType:  string(types.ModelRepo),
		TagName:   "t1",
	}).Exec(ctx)
	require.Nil(t, err)
	tr, err := store.FindByRepo(ctx, "foo", "test", "bar", string(types.ModelRepo))
	require.Nil(t, err)
	require.Equal(t, "t1", tr.TagName)

	_, err = store.FindByRepo(ctx, "foo", "test", "foo", string(types.ModelRepo))
	require.NotNil(t, err)
}
