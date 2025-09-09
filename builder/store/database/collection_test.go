package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestCollectionStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	_, err := store.CreateCollection(ctx, database.Collection{
		Namespace: "n",
		Name:      "col",
		Nickname:  "loc",
		UserID:    123,
	})
	require.Nil(t, err)

	dbc := &database.Collection{}
	err = db.Core.NewSelect().Model(dbc).Where("name=?", "col").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "n", dbc.Namespace)
	require.Equal(t, "loc", dbc.Nickname)

	dbc.Nickname = "lll"
	_, err = store.UpdateCollection(ctx, *dbc)
	require.Nil(t, err)
	dbc = &database.Collection{}
	err = db.Core.NewSelect().Model(dbc).Where("name=?", "col").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "lll", dbc.Nickname)

	col, err := store.FindById(ctx, dbc.ID)
	require.Nil(t, err)
	require.Equal(t, "n", col.Namespace)
	require.Equal(t, "lll", col.Nickname)

	err = store.DeleteCollection(ctx, col.ID, 123)
	require.Nil(t, err)
	dbc = &database.Collection{}
	err = db.Core.NewSelect().Model(dbc).Where("name=?", "col").Scan(ctx)
	require.NotNil(t, err)
}

func TestCollectionStore_CollectionRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	dt := &database.Tag{}
	err := db.Core.NewInsert().Model(&database.Tag{
		Name:     "tag1",
		Category: "task",
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
		Path: "foo/bar",
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

	col, err := store.CreateCollection(ctx, database.Collection{
		Name:    "foo",
		Private: false,
	})
	require.Nil(t, err)
	err = store.AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: col.ID, RepositoryID: repopk},
	})
	require.Nil(t, err)

	col, err = store.GetCollection(ctx, col.ID)
	require.Nil(t, err)
	require.Equal(t, 1, len(col.Repositories))
	require.Equal(t, "foo/bar", col.Repositories[0].Path)
	tags := []string{}
	for _, t := range col.Repositories[0].Tags {
		tags = append(tags, t.Name)
	}
	require.Equal(t, []string{"tag1"}, tags)

	err = store.RemoveCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: col.ID, RepositoryID: repopk},
	})
	require.Nil(t, err)
	col, err = store.GetCollection(ctx, col.ID)
	require.Nil(t, err)
	require.Equal(t, 0, len(col.Repositories))

}

func TestCollectionStore_GetCollections(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	collections := []*database.Collection{
		{Name: "col1-1", Private: false, Likes: 20},
		{Name: "col1-2", Private: true, Likes: 40},
		{Name: "col1-3", Private: false, Likes: 50},
		{Name: "col2-1", Private: false, Likes: 30},
	}

	for _, col := range collections {
		dc, err := store.CreateCollection(ctx, *col)
		col.ID = dc.ID
		require.Nil(t, err)
	}

	names := func(cs []database.Collection) []string {
		names := []string{}
		for _, c := range cs {
			names = append(names, c.Name)
		}
		return names
	}
	cs, total, err := store.GetCollections(ctx, &types.CollectionFilter{}, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, []string{"col1-1", "col1-3", "col2-1"}, names(cs))

	// showPrivate param is not used here
	cs, total, err = store.GetCollections(ctx, &types.CollectionFilter{}, 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, []string{"col1-1", "col1-3", "col2-1"}, names(cs))

	cs, total, err = store.GetCollections(ctx, &types.CollectionFilter{
		Search: "cOl1",
	}, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, []string{"col1-1", "col1-3"}, names(cs))

	cs, total, err = store.GetCollections(ctx, &types.CollectionFilter{
		Sort: "most_favorite",
	}, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, []string{"col1-3", "col2-1", "col1-1"}, names(cs))

	weights := []int{3, 2, 11, 7}
	scores := []float64{150, 13, 12, 11}
	for i, col := range collections {

		repo := &database.Repository{
			Path:    fmt.Sprintf("foo/bar%d", i),
			GitPath: fmt.Sprintf("foo/bar%d", i),
		}
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
		err = store.AddCollectionRepos(ctx, []database.CollectionRepository{
			{CollectionID: col.ID, RepositoryID: repo.ID},
		})
		require.Nil(t, err)

		_, err = db.Core.NewInsert().Model(&database.RecomOpWeight{
			RepositoryID: repo.ID,
			Weight:       weights[i],
		}).Exec(ctx)
		require.Nil(t, err)

		_, err = db.Core.NewInsert().Model(&database.RecomRepoScore{
			RepositoryID: repo.ID,
			Score:        scores[i],
		}).Exec(ctx)
		require.Nil(t, err)

	}
	cs, total, err = store.GetCollections(ctx, &types.CollectionFilter{
		Sort: "trending",
	}, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.Equal(t, []string{"col1-1", "col1-3", "col2-1"}, names(cs))

}

func TestCollectionStore_ByUserLikesOrgs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	collections := []*database.Collection{
		{Name: "col1-1", Private: false, Namespace: "ns"},
		{Name: "col1-2", Private: false, Namespace: "ns"},
		{Name: "col1-3", Private: true, Namespace: "ns2"},
		{Name: "col2-1", Private: false, Namespace: "ns"},
	}

	for _, col := range collections {
		dc, err := store.CreateCollection(ctx, *col)
		col.ID = dc.ID
		require.Nil(t, err)
	}

	_, err := db.Core.NewInsert().Model(&database.UserLike{
		UserID:       123,
		CollectionID: collections[0].ID,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID:       123,
		CollectionID: collections[3].ID,
	}).Exec(ctx)
	require.Nil(t, err)

	cs, total, err := store.ByUserLikes(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	names := []string{}
	for _, c := range cs {
		names = append(names, c.Name)
	}
	require.Equal(t, []string{"col1-1", "col2-1"}, names)

	cs, total, err = store.ByUserOrgs(ctx, "ns2", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	names = []string{}
	for _, c := range cs {
		names = append(names, c.Name)
	}
	require.Equal(t, []string{"col1-3"}, names)

	_, total, err = store.ByUserOrgs(ctx, "ns2", 10, 1, true)
	require.Nil(t, err)
	require.Equal(t, 0, total)

}

func TestCollectionStore_GetCollectionRepos(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	collection := &database.Collection{
		Name:    "col1",
		Private: false,
	}
	dc, err := store.CreateCollection(ctx, *collection)
	collection.ID = dc.ID
	require.Nil(t, err)

	repo := &database.Repository{
		Path: "foo/bar",
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	err = store.AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: collection.ID, RepositoryID: repo.ID},
	})
	require.Nil(t, err)

	repos, err := store.GetCollectionRepos(ctx, collection.ID)
	require.Nil(t, err)
	require.Equal(t, 1, len(repos))
	require.Equal(t, collection.ID, repos[0].CollectionID)
	require.Equal(t, repo.ID, repos[0].RepositoryID)
	require.Equal(t, "", repos[0].Remark)
}

func TestCollectionStore_ByUsername(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	collection := &database.Collection{
		Name:     "col1",
		Private:  false,
		Username: "user",
	}
	dc, err := store.CreateCollection(ctx, *collection)
	collection.ID = dc.ID
	require.Nil(t, err)

	repo := &database.Repository{
		Path: "foo/bar",
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	err = store.AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: collection.ID, RepositoryID: repo.ID},
	})
	require.Nil(t, err)

	cs, total, err := store.ByUsername(ctx, "user", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, collection.ID, cs[0].ID)
	require.Equal(t, repo.ID, cs[0].Repositories[0].ID)
	require.Equal(t, "user", cs[0].Username)
}

func TestCollectionStore_UpdateCollectionRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewCollectionStoreWithDB(db)

	collection := &database.Collection{
		Name:    "col1",
		Private: false,
	}
	dc, err := store.CreateCollection(ctx, *collection)
	collection.ID = dc.ID
	require.Nil(t, err)

	repo := &database.Repository{
		Path: "foo/bar",
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	err = store.AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: collection.ID, RepositoryID: repo.ID},
	})
	require.Nil(t, err)

	t.Run("add remark", func(t *testing.T) {
		err = store.UpdateCollectionRepo(ctx, database.CollectionRepository{
			CollectionID: collection.ID,
			RepositoryID: repo.ID,
			Remark:       "test remark",
		})
		require.Nil(t, err)

		repos, err := store.GetCollectionRepos(ctx, collection.ID)
		require.Nil(t, err)
		require.Equal(t, 1, len(repos))
		require.Equal(t, "test remark", repos[0].Remark)
	})

	t.Run("update remark", func(t *testing.T) {
		err = store.UpdateCollectionRepo(ctx, database.CollectionRepository{
			CollectionID: collection.ID,
			RepositoryID: repo.ID,
			Remark:       "test remark 2",
		})
		require.Nil(t, err)

		repos, err := store.GetCollectionRepos(ctx, collection.ID)
		require.Nil(t, err)
		require.Equal(t, 1, len(repos))
		require.Equal(t, "test remark 2", repos[0].Remark)
	})

	t.Run("delete remark", func(t *testing.T) {
		err = store.UpdateCollectionRepo(ctx, database.CollectionRepository{
			CollectionID: collection.ID,
			RepositoryID: repo.ID,
			Remark:       "",
		})
		require.Nil(t, err)

		repos, err := store.GetCollectionRepos(ctx, collection.ID)
		require.Nil(t, err)
		require.Equal(t, 1, len(repos))
		require.Equal(t, "", repos[0].Remark)
	})
}
