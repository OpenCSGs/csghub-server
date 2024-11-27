package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepositoryFileCheckStore_CreateUpsert(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoFileCheckStoreWithDB(db)

	err := store.Create(ctx, database.RepositoryFileCheck{
		RepoFileID: 123,
		Message:    "foo",
		Status:     types.SensitiveCheckPass,
	})
	require.Nil(t, err)
	rf := &database.RepositoryFileCheck{}
	err = db.Core.NewSelect().Model(rf).Where("repo_file_id=?", 123).Scan(ctx, rf)
	require.Nil(t, err)
	require.Equal(t, "foo", rf.Message)

	rf.Message = "bar"
	err = store.Upsert(ctx, *rf)
	require.Nil(t, err)
	rf = &database.RepositoryFileCheck{}
	err = db.Core.NewSelect().Model(rf).Where("repo_file_id=?", 123).Scan(ctx, rf)
	require.Nil(t, err)
	require.Equal(t, "bar", rf.Message)

}
