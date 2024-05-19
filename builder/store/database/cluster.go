package database

import (
	"context"

	"github.com/uptrace/bun"
)

type ClusterInfoStore struct {
	db *DB
}

func NewClusterInfoStore() *ClusterInfoStore {
	return &ClusterInfoStore{
		db: defaultDB,
	}
}

type ClusterInfo struct {
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	Region string `bun:",notnull" json:"region"`
	Config string `bun:",notnull" json:"repo_id"`
}

func (r *ClusterInfoStore) Add(ctx context.Context, userId, repoId int64) error {
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

func (r *ClusterInfoStore) Delete(ctx context.Context, userId, repoId int64) error {
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

func (r *ClusterInfoStore) IsExist(ctx context.Context, username string, repoId int64) (exists bool, err error) {
	var userLike UserLike
	exists, err = r.db.Operator.Core.
		NewSelect().
		Model(&userLike).
		Join("JOIN users ON users.id = user_like.user_id").
		Where("user_like.repo_id = ? and users.username = ?", repoId, username).
		Exists(ctx)
	return
}
