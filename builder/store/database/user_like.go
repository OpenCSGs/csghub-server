package database

import (
	"context"

	"github.com/uptrace/bun"
)

type UserLikesStore struct {
	db *DB
}

func NewUserLikesStore() *UserLikesStore {
	return &UserLikesStore{
		db: defaultDB,
	}
}

type UserLike struct {
	ID     int64 `bun:",pk,autoincrement" json:"id"`
	UserID int64 `bun:",notnull" json:"user_id"`
	RepoID int64 `bun:",notnull" json:"repo_id"`
}

func (r *UserLikesStore) Add(ctx context.Context, userId, repoId int64) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		userLikes := &UserLike{
			UserID: userId,
			RepoID: repoId,
		}
		if err := assertAffectedOneRow(tx.NewInsert().Model(userLikes).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Exec("update repositories set likes=COALESCE(likes, 0)+1 where id=?", repoId)); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (r *UserLikesStore) Delete(ctx context.Context, userId, repoId int64) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var userLikes UserLike
		if err := assertAffectedOneRow(r.db.Core.NewDelete().Model(&userLikes).Where("user_id = ? and repo_id = ?", userId, repoId).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Exec("update repositories set likes=COALESCE(likes, 1)-1 where id=?", repoId)); err != nil {
			return err
		}
		return nil
	})
	return err
}
