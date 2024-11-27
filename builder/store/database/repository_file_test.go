package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoFileStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoFileStoreWithDB(db)

	err := store.Create(ctx, &database.RepositoryFile{
		Path:         "foo",
		RepositoryID: 123,
	})
	require.Nil(t, err)

	rf := &database.RepositoryFile{}
	err = db.Core.NewSelect().Model(rf).Where("path=?", "foo").Scan(ctx, rf)
	require.Nil(t, err)
	require.Equal(t, "foo", rf.Path)

}

func TestRepoFileStore_BatchGet(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoFileStoreWithDB(db)

	repo := &database.Repository{}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	// check failed file
	err = store.Create(ctx, &database.RepositoryFile{
		RepositoryID: repo.ID,
		Path:         "foo",
		Branch:       "main",
	})
	require.Nil(t, err)
	rf := &database.RepositoryFile{}
	err = db.Core.NewSelect().Model(rf).Where("path=?", "foo").Scan(ctx, rf)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RepositoryFileCheck{
		RepoFileID: rf.ID,
		Status:     types.SensitiveCheckFail,
	}).Exec(ctx)
	require.Nil(t, err)

	// check pass file
	err = store.Create(ctx, &database.RepositoryFile{
		RepositoryID: repo.ID,
		Path:         "bar",
		Branch:       "main",
	})
	require.Nil(t, err)
	rf = &database.RepositoryFile{}
	err = db.Core.NewSelect().Model(rf).Where("path=?", "bar").Scan(ctx, rf)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.RepositoryFileCheck{
		RepoFileID: rf.ID,
		Status:     types.SensitiveCheckPass,
	}).Exec(ctx)
	require.Nil(t, err)

	rfs, err := store.BatchGet(ctx, repo.ID, 0, 10)
	require.Nil(t, err)
	ps := []string{}
	for _, rf := range rfs {
		ps = append(ps, rf.Path)
	}
	require.Equal(t, []string{"foo", "bar"}, ps)

	rfs, err = store.BatchGetUnchcked(ctx, repo.ID, 0, 10)
	require.Nil(t, err)
	ps = []string{}
	for _, rf := range rfs {
		ps = append(ps, rf.Path)
	}
	require.Equal(t, []string{}, ps)

	exist, err := store.ExistsSensitiveCheckRecord(ctx, repo.ID, "main", types.SensitiveCheckPass)
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.ExistsSensitiveCheckRecord(ctx, repo.ID, "main", types.SensitiveCheckFail)
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.ExistsSensitiveCheckRecord(ctx, repo.ID, "main", types.SensitiveCheckSkip)
	require.Nil(t, err)
	require.False(t, exist)
}

func TestRepoFileStore_Exists(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoFileStoreWithDB(db)

	err := store.Create(ctx, &database.RepositoryFile{
		Path:         "foo",
		RepositoryID: 123,
		Branch:       "main",
		CommitSha:    "12321",
	})
	require.Nil(t, err)

	exist, err := store.Exists(ctx, database.RepositoryFile{
		Path:         "foo",
		RepositoryID: 123,
		Branch:       "main",
		CommitSha:    "12321",
	})
	require.Nil(t, err)
	require.True(t, exist)

	exist, err = store.Exists(ctx, database.RepositoryFile{
		Path:         "foo",
		RepositoryID: 123,
		Branch:       "main",
		CommitSha:    "12322",
	})
	require.Nil(t, err)
	require.False(t, exist)

}
