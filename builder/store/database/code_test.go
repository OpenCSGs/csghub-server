package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestCodeStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCodeStoreWithDB(db)
	repo, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		Path:    "foo/bar",
		Private: true,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.Code{
		RepositoryID: 123,
	})
	require.Nil(t, err)

	code := &database.Code{}
	err = db.Core.NewSelect().Model(code).Where("repository_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(123), code.RepositoryID)

	code.RepositoryID = repo.ID
	err = store.Update(ctx, *code)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(code).Where("repository_id=?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, repo.ID, code.RepositoryID)

	cd, err := store.ByRepoID(ctx, repo.ID)
	require.Nil(t, err)
	require.Equal(t, repo.ID, cd.RepositoryID)

	cds, err := store.ByRepoIDs(ctx, []int64{repo.ID})
	require.Nil(t, err)
	require.Equal(t, 1, len(cds))
	require.Equal(t, repo.ID, cds[0].RepositoryID)

	cds, total, err := store.ByUsername(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(cds))
	require.Equal(t, repo.ID, cds[0].RepositoryID)

	cds, total, err = store.ByUsername(ctx, "foo", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, 0, len(cds))

	cd, err = store.FindByPath(ctx, "foo", "bar")
	require.Nil(t, err)
	require.Equal(t, repo.ID, cd.RepositoryID)

	cds, err = store.ListByPath(ctx, []string{"foo/bar"})
	require.Nil(t, err)
	require.Equal(t, 1, len(cds))
	require.Equal(t, repo.ID, cds[0].RepositoryID)

	cds, total, err = store.ByOrgPath(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(cds))
	require.Equal(t, repo.ID, cds[0].RepositoryID)

	cds, total, err = store.ByOrgPath(ctx, "foo", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, 0, len(cds))

}

func TestCodeStore_UserLikesCodes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCodeStoreWithDB(db)

	repo1, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		Path:    "foo/bar",
		GitPath: "p1",
	})
	require.Nil(t, err)
	repo2, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		Path:    "foo/bar2",
		GitPath: "p2",
	})
	require.Nil(t, err)

	repo3, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		Path:    "foo/bar3",
		GitPath: "p3",
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.Code{
		RepositoryID: repo1.ID,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.Code{
		RepositoryID: repo2.ID,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.Code{
		RepositoryID: repo3.ID,
	})
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repo2.ID,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID:    123,
		RepoID:    repo3.ID,
		DeletedAt: time.Now(),
	}).Exec(ctx)
	require.Nil(t, err)

	cs, total, err := store.UserLikesCodes(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, repo2.ID, cs[0].RepositoryID)

}

func TestCodeStore_CreateAndUpdateRepoPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCodeStoreWithDB(db)

	repo := &database.Repository{
		Name: "repo1",
		Path: "p1",
	}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	code, err := store.CreateAndUpdateRepoPath(ctx, database.Code{
		RepositoryID: repo.ID,
	}, "p2")

	require.Nil(t, err)
	require.Equal(t, "p2", code.Repository.Path)
}
