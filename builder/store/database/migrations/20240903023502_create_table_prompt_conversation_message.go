package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type PromptConversationMessage struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	ConversationID string `bun:",notnull" json:"conversation_id"`
	Role           string `bun:",notnull" json:"role"`
	Content        string `bun:",notnull" json:"content"`
	UserLike       bool   `bun:",notnull" json:"user_like"`
	UserHate       bool   `bun:",notnull" json:"user_hate"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, PromptConversationMessage{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*PromptConversationMessage)(nil)).
			Index("idx_prompt_conversation_message_conversationid").
			Column("conversation_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, PromptConversationMessage{})
	})
}
