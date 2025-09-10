package database

import (
	"context"

	"github.com/uptrace/bun"
)

type UserTag struct {
	ID     int64 `bun:"column:pk,autoincrement"`
	UserID int64 `bun:",notnull"`
	TagID  int64 `bun:",notnull"`

	Tag Tag `bun:"rel:belongs-to,join:tag_id=id"`

	times
}

type UserTagStore interface {
	ResetUserTags(ctx context.Context, userId int64, tagIDs []int64) error
	GetUserTags(ctx context.Context, userId int64) ([]*Tag, error)
}

type userTagStoreImpl struct {
	db *DB
}

func NewUserTagStore() UserTagStore {
	return &userTagStoreImpl{
		db: defaultDB,
	}
}

func NewUserTagStoreWithDB(db *DB) UserTagStore {
	return &userTagStoreImpl{
		db: db,
	}
}

func (s *userTagStoreImpl) ResetUserTags(ctx context.Context, userId int64, tagIDs []int64) error {
	userTags := make([]UserTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		userTags = append(userTags, UserTag{
			UserID: userId,
			TagID:  tagID,
		})
	}

	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewDelete().Model((*UserTag)(nil)).Where("user_id = ?", userId).Exec(ctx)
		if err != nil {
			return err
		}

		if len(userTags) > 0 {
			_, err := tx.NewInsert().Model(&userTags).Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *userTagStoreImpl) GetUserTags(ctx context.Context, userId int64) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.Operator.Core.NewSelect().Model((*Tag)(nil)).
		Join("INNER JOIN user_tags ON user_tags.tag_id = tag.id").
		Where("user_tags.user_id = ?", userId).
		Scan(ctx, &tags)
	if err != nil {
		return nil, err
	}

	return tags, nil
}
