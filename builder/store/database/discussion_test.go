package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
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

	dss, _, err := store.FindByDiscussionableID(ctx, "zzz", 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(dss))
	dss, _, err = store.FindByDiscussionableID(ctx, "zzz", 456, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 0, len(dss))

	_, err = store.CreateComment(ctx, database.Comment{
		CommentableID:   ds.ID,
		CommentableType: database.CommentableTypeDiscussion,
		Content:         "foobar",
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

	err = store.DeleteByID(ctx, ds.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, ds.ID)
	require.NotNil(t, err)
	require.True(t, errors.Is(err, errorx.ErrDatabaseNoRows))
}

func TestDiscussionStore_CreateCommentWithMediaIsAtomic(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewDiscussionStoreWithDB(db)

	discussion, err := store.Create(ctx, database.Discussion{
		Title: "media discussion", DiscussionableType: database.DiscussionableTypeRepo, DiscussionableID: 1,
	})
	require.NoError(t, err)
	items := []types.CommentMediaItem{{DataID: "data-1", TaskID: "task-1", MediaType: types.MediaTypeAudio}}
	_, err = db.Core.NewInsert().Model(&database.MediaModeration{
		ResourceKey: "resource-1", DataID: "data-1", Seed: "seed-1", TaskID: "task-1",
		MediaType: types.MediaTypeAudio, Status: database.MediaModerationStatusPending,
	}).Exec(ctx)
	require.NoError(t, err)

	comment, err := store.CreateCommentWithMedia(ctx, database.Comment{
		Content: "pending media", CommentableID: discussion.ID, CommentableType: database.CommentableTypeDiscussion,
	}, items)
	require.NoError(t, err)

	var link database.CommentMedia
	require.NoError(t, db.Core.NewSelect().Model(&link).Where("comment_id = ?", comment.ID).Scan(ctx))
	require.Equal(t, "data-1", link.DataID)

	_, err = store.CreateCommentWithMedia(ctx, database.Comment{
		Content: "must roll back", CommentableID: discussion.ID, CommentableType: database.CommentableTypeDiscussion,
	}, []types.CommentMediaItem{{DataID: "invalid-data", MediaType: types.MediaType("invalid")}})
	require.Error(t, err)
	count, countErr := db.Core.NewSelect().Model((*database.Comment)(nil)).
		Where("content = ?", "must roll back").Count(ctx)
	require.NoError(t, countErr)
	require.Zero(t, count, "comment insert must roll back when media linking fails")

	require.NoError(t, store.DeleteComment(ctx, comment.ID))
	linkCount, linkCountErr := db.Core.NewSelect().Model((*database.CommentMedia)(nil)).
		Where("comment_id = ?", comment.ID).Count(ctx)
	require.NoError(t, linkCountErr)
	require.Zero(t, linkCount, "deleting a comment must remove its media links")
}

func TestDiscussionStore_FindVisibleDiscussionCommentsPaginatesBeforeReturning(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewDiscussionStoreWithDB(db)

	discussion, err := store.Create(ctx, database.Discussion{
		Title: "visible pagination", DiscussionableType: database.DiscussionableTypeRepo, DiscussionableID: 1,
	})
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.MediaModeration{
		ResourceKey: "pending-resource", DataID: "pending-data", Seed: "seed",
		MediaType: types.MediaTypeVideo, Status: database.MediaModerationStatusPending,
	}).Exec(ctx)
	require.NoError(t, err)

	_, err = store.CreateComment(ctx, database.Comment{
		Content: "visible older", CommentableID: discussion.ID, CommentableType: database.CommentableTypeDiscussion,
	})
	require.NoError(t, err)
	_, err = store.CreateCommentWithMedia(ctx, database.Comment{
		Content: "hidden newest", CommentableID: discussion.ID, CommentableType: database.CommentableTypeDiscussion,
	}, []types.CommentMediaItem{{DataID: "pending-data", MediaType: types.MediaTypeVideo}})
	require.NoError(t, err)

	comments, total, err := store.FindVisibleDiscussionComments(ctx, database.FindVisibleCommentsReq{
		DiscussionID: discussion.ID, CurrentUser: "", Per: 1, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, comments, 1)
	require.Equal(t, "visible older", comments[0].Content)
}
