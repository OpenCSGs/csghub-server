package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountSyncQuotaStatementStore_CreateGet(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	_, err := db.Core.NewInsert().Model(&database.AccountSyncQuota{
		UserID:         123,
		RepoCountLimit: 1,
		RepoCountUsed:  4,
	}).Exec(ctx)
	require.Nil(t, err)
	store := database.NewAccountSyncQuotaStatementStoreWithDB(db)

	err = store.Create(ctx, database.AccountSyncQuotaStatement{
		UserID:   123,
		RepoPath: "foo/bar",
	})
	// download reach limit
	require.NotNil(t, err)

	_, err = db.Core.NewUpdate().Model(&database.AccountSyncQuota{
		RepoCountLimit: 4,
		RepoCountUsed:  1,
	}).Where("user_id=?", 123).Exec(ctx)
	require.Nil(t, err)
	err = store.Create(ctx, database.AccountSyncQuotaStatement{
		UserID:   123,
		RepoPath: "foo/bar",
	})
	require.Nil(t, err)

	d := &database.AccountSyncQuotaStatement{}
	err = db.Core.NewSelect().Model(d).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo/bar", d.RepoPath)

	s, err := store.Get(ctx, 123, types.AcctQuotaStatementReq{
		RepoPath: "foo/bar",
	})
	require.Nil(t, err)
	require.Equal(t, "foo/bar", s.RepoPath)
}
