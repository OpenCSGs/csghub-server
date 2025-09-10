package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestModelStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelStoreWithDB(db)
	_, err := store.Create(ctx, database.Model{
		RepositoryID: 123,
	})
	require.Nil(t, err)

	m := &database.Model{}
	err = db.Core.NewSelect().Model(m).Where("repository_id=?", 123).Scan(ctx)
	require.Nil(t, err)

	m, err = store.ByID(ctx, m.ID)
	require.Nil(t, err)
	require.Equal(t, int64(123), m.RepositoryID)

	m.RepositoryID = 456
	_, err = store.Update(ctx, *m)
	require.Nil(t, err)
	m = &database.Model{}
	err = db.Core.NewSelect().Model(m).Where("repository_id=?", 456).Scan(ctx)
	require.Nil(t, err)

	m, err = store.ByRepoID(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, int64(456), m.RepositoryID)

	ms, err := store.ByRepoIDs(ctx, []int64{456})
	require.Nil(t, err)
	require.Equal(t, int64(456), ms[0].RepositoryID)

	_, err = store.CreateIfNotExist(ctx, database.Model{
		RepositoryID: 789,
	})
	require.Nil(t, err)
	m, err = store.ByRepoID(ctx, 789)
	require.Nil(t, err)
	require.Equal(t, int64(789), m.RepositoryID)

	repo := &database.Repository{
		Path:    "foo/bar",
		GitPath: "foo/bar2",
		Private: true,
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)
	m.RepositoryID = repo.ID
	_, err = store.Update(ctx, *m)
	require.Nil(t, err)

	ms, total, err := store.ByUsername(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, len(ms), 1)

	ms, total, err = store.ByUsername(ctx, "foo", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, len(ms), 0)

	ms, total, err = store.ByOrgPath(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, len(ms), 1)

	ms, total, err = store.ByOrgPath(ctx, "foo", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, len(ms), 0)

	m, err = store.FindByPath(ctx, "foo", "bar")
	require.Nil(t, err)
	require.Equal(t, repo.ID, m.RepositoryID)

	err = store.Delete(ctx, *m)
	require.Nil(t, err)
	_, err = store.FindByPath(ctx, "foo", "bar")
	require.NotNil(t, err)
}

func TestModelStore_ListByPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelStoreWithDB(db)

	dt := &database.Tag{}
	err := db.Core.NewInsert().Model(&database.Tag{
		Name:     "tag1",
		Category: "evaluation",
	}).Scan(ctx, dt)
	require.Nil(t, err)
	tag1pk := dt.ID

	err = db.Core.NewInsert().Model(&database.Tag{
		Name:     "tag2",
		Category: "foo",
	}).Scan(ctx, dt)
	require.Nil(t, err)
	tag2pk := dt.ID

	dr := &database.Repository{}
	err = db.Core.NewInsert().Model(&database.Repository{
		Name:    "repo",
		Path:    "foo/bar",
		GitPath: "a",
	}).Scan(ctx, dr)
	require.Nil(t, err)
	repopk := dr.ID

	for _, tpk := range []int64{tag1pk, tag2pk} {
		_, err = db.Core.NewInsert().Model(&database.RepositoryTag{
			RepositoryID: repopk,
			TagID:        tpk,
		}).Exec(ctx)
		require.Nil(t, err)
	}

	_, err = store.Create(ctx, database.Model{
		RepositoryID: repopk,
	})
	require.Nil(t, err)

	dr2 := &database.Repository{}
	err = db.Core.NewInsert().Model(&database.Repository{
		Name:    "repo2",
		Path:    "bar/foo",
		GitPath: "b",
	}).Scan(ctx, dr2)
	require.Nil(t, err)
	_, err = store.Create(ctx, database.Model{
		RepositoryID: dr2.ID,
	})
	require.Nil(t, err)

	dr3 := &database.Repository{}
	err = db.Core.NewInsert().Model(&database.Repository{
		Name:           "repo3",
		Path:           "foo/bar",
		GitPath:        "c",
		RepositoryType: types.ModelRepo,
	}).Scan(ctx, dr3)
	require.Nil(t, err)
	_, err = store.Create(ctx, database.Model{
		RepositoryID: dr3.ID,
	})
	require.Nil(t, err)

	dss, err := store.ListByPath(ctx, []string{"bar/foo", "foo/bar"})
	require.Nil(t, err)
	require.Equal(t, 3, len(dss))

	tags := []string{}
	for _, t := range dss[1].Repository.Tags {
		tags = append(tags, t.Name)
	}
	require.Equal(t, []string{}, tags)

	names := []string{}
	for _, ds := range dss {
		names = append(names, ds.Repository.Name)
	}
	require.Equal(t, []string{"repo2", "repo", "repo3"}, names)

}

func TestModelStore_UserLikes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", Path: "p1", GitPath: "p1"},
		{Name: "repo2", Path: "p2", GitPath: "p2"},
		{Name: "repo3", Path: "p3", GitPath: "p3"},
	}

	for _, repo := range repos {
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
		_, err = store.Create(ctx, database.Model{
			RepositoryID: repo.ID,
		})
		require.Nil(t, err)

	}

	_, err := db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repos[0].ID,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repos[2].ID,
	}).Exec(ctx)
	require.Nil(t, err)

	dss, total, err := store.UserLikesModels(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, total)

	names := []string{}
	for _, ds := range dss {
		names = append(names, ds.Repository.Name)
	}
	require.Equal(t, []string{"repo1", "repo3"}, names)

}

func TestModelStore_UserLikes_WithSoftDeleted(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", Path: "p1", GitPath: "p1"},
		{Name: "repo2", Path: "p2", GitPath: "p2"},
		{Name: "repo3", Path: "p3", GitPath: "p3"},
	}

	for _, repo := range repos {
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
		_, err = store.Create(ctx, database.Model{
			RepositoryID: repo.ID,
		})
		require.Nil(t, err)

	}

	_, err := db.Core.NewInsert().Model(&database.UserLike{
		UserID:    123,
		RepoID:    repos[0].ID,
		DeletedAt: time.Now(),
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repos[2].ID,
	}).Exec(ctx)
	require.Nil(t, err)

	dss, total, err := store.UserLikesModels(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, total)

	names := []string{}
	for _, ds := range dss {
		names = append(names, ds.Repository.Name)
	}
	require.Equal(t, []string{"repo3"}, names)

}
