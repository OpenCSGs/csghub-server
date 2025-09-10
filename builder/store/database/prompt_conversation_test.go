package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestPromptConversationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPromptConversationStoreWithDB(db)
	msg := &database.PromptConversationMessage{
		ConversationID: "cv",
		Content:        "msg",
	}
	err := db.Core.NewInsert().Model(msg).Scan(ctx, msg)
	require.Nil(t, err)
	err = store.CreateConversation(ctx, database.PromptConversation{
		UserID:         123,
		ConversationID: "cv",
		Title:          "foo",
	})
	require.Nil(t, err)

	pc := &database.PromptConversation{}
	err = db.Core.NewSelect().Model(pc).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", pc.Title)

	pc, err = store.GetConversationByID(ctx, 123, "cv", false)
	require.Nil(t, err)
	require.Equal(t, "foo", pc.Title)
	require.Nil(t, pc.Messages)
	pc, err = store.GetConversationByID(ctx, 123, "cv", true)
	require.Nil(t, err)
	require.Equal(t, "foo", pc.Title)
	require.Equal(t, 1, len(pc.Messages))
	require.Equal(t, "msg", pc.Messages[0].Content)

	pc.Title = "bar"
	err = store.UpdateConversation(ctx, *pc)
	require.Nil(t, err)
	pc = &database.PromptConversation{}
	err = db.Core.NewSelect().Model(pc).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "bar", pc.Title)

	pcs, err := store.FindConversationsByUserID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(pcs))
	require.Equal(t, "bar", pcs[0].Title)

	_, err = store.SaveConversationMessage(ctx, database.PromptConversationMessage{
		ConversationID: pc.ConversationID,
		Content:        "foobar",
	})
	require.Nil(t, err)

	pc, err = store.GetConversationByID(ctx, 123, "cv", true)
	require.Nil(t, err)
	require.Equal(t, 2, len(pc.Messages))

	err = store.LikeMessageByID(ctx, msg.ID)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(msg).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, true, msg.UserLike)
	err = store.LikeMessageByID(ctx, msg.ID)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(msg).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, false, msg.UserLike)

	err = store.HateMessageByID(ctx, msg.ID)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(msg).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, true, msg.UserHate)
	err = store.HateMessageByID(ctx, msg.ID)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(msg).WherePK().Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, false, msg.UserHate)

	err = store.DeleteConversationsByID(ctx, 123, pc.ConversationID)
	require.Nil(t, err)
	_, err = store.GetConversationByID(ctx, 123, "cv", false)
	require.NotNil(t, err)
}

func TestPromptConversationStore_GetByUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewPromptConversationStoreWithDB(db)
	msg := &database.PromptConversationMessage{
		ConversationID: "cv",
		Content:        "msg",
	}
	err := db.Core.NewInsert().Model(msg).Scan(ctx, msg)
	require.Nil(t, err)

	err = store.CreateConversation(ctx, database.PromptConversation{
		UserID:         123,
		ConversationID: "cv",
		Title:          "foo",
	})
	require.Nil(t, err)

	conversation, err := store.GetByUUID(ctx, "cv", true)
	require.Nil(t, err)
	require.Equal(t, 1, len(conversation.Messages))
}
