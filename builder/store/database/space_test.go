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

func TestSpaceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceStoreWithDB(db)
	_, err := store.Create(ctx, database.Space{
		RepositoryID: 123,
		Sdk:          "sdk1",
	})
	require.Nil(t, err)

	sp := &database.Space{}
	err = db.Core.NewSelect().Model(sp).Where("repository_id=?", 123).Scan(ctx)
	require.Nil(t, err)

	sp, err = store.ByID(ctx, sp.ID)
	require.Nil(t, err)
	require.Equal(t, int64(123), sp.RepositoryID)

	sp.RepositoryID = 456
	err = store.Update(ctx, *sp)
	require.Nil(t, err)
	sp = &database.Space{}
	err = db.Core.NewSelect().Model(sp).Where("repository_id=?", 456).Scan(ctx)
	require.Nil(t, err)

	sp, err = store.ByRepoID(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, int64(456), sp.RepositoryID)

	sps, err := store.ByRepoIDs(ctx, []int64{456})
	require.Nil(t, err)
	require.Equal(t, int64(456), sps[0].RepositoryID)

	repo := &database.Repository{
		Path:           "foo/bar",
		GitPath:        "foo/bar2",
		Private:        true,
		RepositoryType: types.SpaceRepo,
	}

	req := &types.UserSpacesReq{
		Owner: "foo",
		PageOpts: types.PageOpts{
			PageSize: 10,
			Page:     1,
		},
	}

	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)
	sp.RepositoryID = repo.ID
	err = store.Update(ctx, *sp)
	require.Nil(t, err)

	sps, total, err := store.ByUsername(ctx, req, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, len(sps), 1)

	sps, total, err = store.ByUsername(ctx, req, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, len(sps), 0)

	req.SDK = "sdk1"
	sps, total, err = store.ByUsername(ctx, req, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, len(sps), 1)

	sps, total, err = store.ByOrgPath(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, len(sps), 1)

	sps, total, err = store.ByOrgPath(ctx, "foo", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, len(sps), 0)

	sp, err = store.FindByPath(ctx, "foo", "bar")
	require.Nil(t, err)
	require.Equal(t, repo.ID, sp.RepositoryID)

	err = store.Delete(ctx, *sp)
	require.Nil(t, err)
	_, err = store.FindByPath(ctx, "foo", "bar")
	require.NotNil(t, err)
}

func TestSpaceStore_ListByPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceStoreWithDB(db)

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

	_, err = store.Create(ctx, database.Space{
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
	_, err = store.Create(ctx, database.Space{
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
	_, err = store.Create(ctx, database.Space{
		RepositoryID: dr3.ID,
	})
	require.Nil(t, err)

	sps, err := store.ListByPath(ctx, []string{"bar/foo", "foo/bar"})
	require.Nil(t, err)
	require.Equal(t, 3, len(sps))

	tags := []string{}
	for _, t := range sps[1].Repository.Tags {
		tags = append(tags, t.Name)
	}
	require.Equal(t, []string{}, tags)

	names := []string{}
	for _, sp := range sps {
		names = append(names, sp.Repository.Name)
	}
	require.Equal(t, []string{"repo2", "repo", "repo3"}, names)

}

func TestSpaceStore_UserLikes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", Path: "p1", GitPath: "p1"},
		{Name: "repo2", Path: "p2", GitPath: "p2"},
		{Name: "repo3", Path: "p3", GitPath: "p3"},
		{Name: "repo4", Path: "p4", GitPath: "p4"},
	}

	for _, repo := range repos {
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
		_, err = store.Create(ctx, database.Space{
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
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID:    123,
		RepoID:    repos[3].ID,
		DeletedAt: time.Now(),
	}).Exec(ctx)
	require.Nil(t, err)

	sps, total, err := store.ByUserLikes(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, total)

	names := []string{}
	for _, sp := range sps {
		names = append(names, sp.Repository.Name)
	}
	require.Equal(t, []string{"repo1", "repo3"}, names)

}

func TestSpaceStore_CreateAndUpdateRepoPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceStoreWithDB(db)

	repo := &database.Repository{
		Name: "repo1",
		Path: "p1",
	}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	space, err := store.CreateAndUpdateRepoPath(ctx, database.Space{
		RepositoryID: repo.ID,
	}, "p2")

	require.Nil(t, err)
	require.Equal(t, "p2", space.Repository.Path)
}
