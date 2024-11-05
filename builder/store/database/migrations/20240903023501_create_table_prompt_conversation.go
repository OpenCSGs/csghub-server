package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type PromptConversation struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	UserID         int64  `bun:",notnull" json:"user_id"`
	ConversationID string `bun:",notnull" json:"conversation_id"`
	Title          string `bun:",notnull" json:"title"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, PromptConversation{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*PromptConversation)(nil)).
			Index("idx_unique_prompt_conversation_conversationid").
			Column("conversation_id").
			Unique().
			Exec(ctx)
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*PromptConversation)(nil)).
			Index("idx_prompt_conversation_userid").
			Column("user_id").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, PromptConversation{})
	})
}
