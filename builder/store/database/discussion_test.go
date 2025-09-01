package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
)

func TestDiscussionStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewDiscussionStoreWithDB(db)
	_, err := store.Create(ctx, database.Discussion{
		Title:              "dis",
		DiscussionableType: "zzz",
		DiscussionableID:   123,
	})
	require.Nil(t, err)
	ds := &database.Discussion{}
	err = db.Core.NewSelect().Model(ds).Where("title=?", "dis").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "dis", ds.Title)

	ds, err = store.FindByID(ctx, ds.ID)
	require.Nil(t, err)
	require.Equal(t, "dis", ds.Title)

	err = store.UpdateByID(ctx, ds.ID, "foo")
	require.Nil(t, err)
	ds, err = store.FindByID(ctx, ds.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", ds.Title)

	dss, err := store.FindByDiscussionableID(ctx, "zzz", 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(dss))
	dss, err = store.FindByDiscussionableID(ctx, "zzz", 456)
	require.Nil(t, err)
	require.Equal(t, 0, len(dss))

	err = store.DeleteByID(ctx, ds.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, ds.ID)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrDatabaseNoRows))

	_, err = store.CreateComment(ctx, database.Comment{
		Content: "foobar",
	})
	require.Nil(t, err)
	cm := &database.Comment{}
	err = db.Core.NewSelect().Model(cm).Where("content=?", "foobar").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foobar", cm.Content)

	cm, err = store.FindCommentByID(ctx, cm.ID)
	require.Nil(t, err)
	require.Equal(t, "foobar", cm.Content)

	err = store.UpdateComment(ctx, cm.ID, "barfoo")
	require.Nil(t, err)
	cm, err = store.FindCommentByID(ctx, cm.ID)
	require.Nil(t, err)
	require.Equal(t, "barfoo", cm.Content)

	err = store.DeleteComment(ctx, cm.ID)
	require.Nil(t, err)
	_, err = store.FindCommentByID(ctx, cm.ID)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrDatabaseNoRows))
}
