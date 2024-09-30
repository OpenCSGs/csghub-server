package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type PromptConversationStore struct {
	db *DB
}

type PromptConversation struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	UserID         int64  `bun:",notnull" json:"user_id"`
	ConversationID string `bun:",notnull" json:"conversation_id"`
	Title          string `bun:",notnull" json:"title"`
	times
	Messages []PromptConversationMessage `bun:"rel:has-many,join:conversation_id=conversation_id" json:"messages"`
}

type PromptConversationMessage struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	ConversationID string `bun:",notnull" json:"conversation_id"`
	Role           string `bun:",notnull" json:"role"`
	Content        string `bun:",notnull" json:"content"`
	UserLike       bool   `bun:",notnull" json:"user_like"`
	UserHate       bool   `bun:",notnull" json:"user_hate"`
	times
}

func NewPromptConversationStore() *PromptConversationStore {
	return &PromptConversationStore{db: defaultDB}
}

func (p *PromptConversationStore) CreateConversation(ctx context.Context, conversation PromptConversation) error {
	err := p.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&conversation).Exec(ctx)); err != nil {
			return fmt.Errorf("save conversation, %v, error:%w", conversation, err)
		}
		return nil
	})
	return err
}

func (p *PromptConversationStore) SaveConversationMessage(ctx context.Context, message PromptConversationMessage) (*PromptConversationMessage, error) {
	res, err := p.db.Core.NewInsert().Model(&message).Exec(ctx, &message)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("insert message, %v, error:%w", message, err)
	}
	return &message, nil
}

func (p *PromptConversationStore) UpdateConversation(ctx context.Context, conversation PromptConversation) error {
	res, err := p.db.Core.NewUpdate().Model(&conversation).
		Where("user_id = ?", conversation.UserID).
		Where("conversation_id = ?", conversation.ConversationID).
		Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("update conversation, %v, error:%w", conversation, err)
	}
	return nil
}

func (p *PromptConversationStore) FindConversationsByUserID(ctx context.Context, userID int64) ([]PromptConversation, error) {
	var conversations []PromptConversation
	err := p.db.Operator.Core.NewSelect().Model(&conversations).Where("user_id = ?", userID).Order("id desc").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select conversation by userid %d, error: %w", userID, err)
	}
	return conversations, nil
}

func (p *PromptConversationStore) GetConversationByID(ctx context.Context, userID int64, uuid string, hasDetail bool) (*PromptConversation, error) {
	var conversation PromptConversation
	q := p.db.Operator.Core.NewSelect().Model(&conversation)
	if hasDetail {
		q = q.Relation("Messages")
	}
	err := q.Where("user_id = ? and conversation_id = ?", userID, uuid).Order("id desc").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select user conversation by userid %d, uuid %s, error: %w", userID, uuid, err)
	}
	return &conversation, nil
}

func (p *PromptConversationStore) DeleteConversationsByID(ctx context.Context, userID int64, uuid string) error {
	err := p.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().Model(&PromptConversation{}).Where("user_id = ? and conversation_id = ?", userID, uuid).Exec(ctx)
		err = assertAffectedOneRow(res, err)
		if err != nil {
			return fmt.Errorf("delete conversation by userid %d, %s, error: %w", userID, uuid, err)
		}

		_, err = tx.NewDelete().Model(&PromptConversationMessage{}).Where("conversation_id = ?", uuid).Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete conversation message by uuid, %s, error:%w", uuid, err)
		}
		return nil
	})
	return err
}

func (p *PromptConversationStore) LikeMessageByID(ctx context.Context, id int64) error {
	res, err := p.db.BunDB.Exec("update prompt_conversation_messages set user_like=NOT user_like where id = ?", id)
	if err != nil {
		return err
	}
	err = assertAffectedOneRow(res, err)
	return err
}

func (p *PromptConversationStore) HateMessageByID(ctx context.Context, id int64) error {
	res, err := p.db.BunDB.Exec("update prompt_conversation_messages set user_hate=NOT user_hate where id = ?", id)
	if err != nil {
		return err
	}
	err = assertAffectedOneRow(res, err)
	return err
}
