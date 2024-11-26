package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestUserLikeStore_Add(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	r, err := repoStore.CreateRepo(ctx, database.Repository{
		ID: 2,
	})
	require.Nil(t, err)
	require.Equal(t, r.ID, int64(2))

	err = ulikeStore.Add(ctx, 1, 2)
	require.Nil(t, err)

	repo, err := repoStore.FindById(ctx, int64(2))
	require.Nil(t, err)
	require.Equal(t, repo.Likes, int64(1))
}

func TestUserLikeStore_LikeCollection(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	colStore := database.NewCollectionStoreWithDB(db)

	c, err := colStore.CreateCollection(ctx, database.Collection{
		ID: 3,
	})
	require.Nil(t, err)
	require.Equal(t, c.ID, int64(3))

	err = ulikeStore.LikeCollection(ctx, 1, 3)
	require.Nil(t, err)

	col, err := colStore.FindById(ctx, int64(3))
	require.Nil(t, err)
	require.Equal(t, col.Likes, int64(1))
}

func TestUserLikeStore_UnlikeCollection(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	colStore := database.NewCollectionStoreWithDB(db)

	c, err := colStore.CreateCollection(ctx, database.Collection{
		ID:    3,
		Likes: 1,
	})
	require.Nil(t, err)
	require.Equal(t, c.ID, int64(3))

	err = ulikeStore.LikeCollection(ctx, 1, 3)
	require.Nil(t, err)

	err = ulikeStore.UnLikeCollection(ctx, 1, 3)
	require.Nil(t, err)

	col, err := colStore.FindById(ctx, int64(3))
	require.Nil(t, err)
	require.Equal(t, col.Likes, int64(1))
}

func TestUserLikeStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	r, err := repoStore.CreateRepo(ctx, database.Repository{
		ID:    2,
		Likes: 1,
	})
	require.Nil(t, err)
	require.Equal(t, r.ID, int64(2))

	err = ulikeStore.Add(ctx, 1, 2)
	require.Nil(t, err)

	err = ulikeStore.Delete(ctx, 1, 2)
	require.Nil(t, err)

	repo, err := repoStore.FindById(ctx, int64(2))
	require.Nil(t, err)
	require.Equal(t, repo.Likes, int64(1))
}

func TestUserLikeStore_IsExist(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	r, err := repoStore.CreateRepo(ctx, database.Repository{
		ID: 2,
	})
	require.Nil(t, err)
	require.Equal(t, r.ID, int64(2))

	err = userStore.Create(ctx, &database.User{
		ID:       1,
		Username: "wanghh2000",
	}, &database.Namespace{
		Path: "wanghh2000",
	})
	require.Nil(t, err)

	err = ulikeStore.Add(ctx, 1, 2)
	require.Nil(t, err)

	isExist, err := ulikeStore.IsExist(ctx, "wanghh2000", 2)
	require.Nil(t, err)
	require.Equal(t, true, isExist)
}

func TestUserLikeStore_IsExistCollection(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ulikeStore := database.NewUserLikesStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	colStore := database.NewCollectionStoreWithDB(db)

	err := userStore.Create(ctx, &database.User{
		ID:       1,
		Username: "wanghh2000",
	}, &database.Namespace{
		Path: "wanghh2000",
	})
	require.Nil(t, err)

	c, err := colStore.CreateCollection(ctx, database.Collection{
		ID:    2,
		Likes: 1,
	})
	require.Nil(t, err)
	require.Equal(t, c.ID, int64(2))

	err = ulikeStore.LikeCollection(ctx, 1, 2)
	require.Nil(t, err)

	isExist, err := ulikeStore.IsExistCollection(ctx, "wanghh2000", 2)
	require.Nil(t, err)
	require.Equal(t, true, isExist)
}
