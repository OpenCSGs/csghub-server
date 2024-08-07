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
	ID           int64 `bun:",pk,autoincrement" json:"id"`
	UserID       int64 `bun:",notnull" json:"user_id"`
	RepoID       int64 `bun:",notnull" json:"repo_id"`
	CollectionID int64 `bun:",notnull" json:"collection_id"`
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

func (r *UserLikesStore) LikeCollection(ctx context.Context, userId, collectionId int64) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		userLikes := &UserLike{
			UserID:       userId,
			CollectionID: collectionId,
		}
		if err := assertAffectedOneRow(tx.NewInsert().Model(userLikes).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Exec("update collections set likes=COALESCE(likes, 0)+1 where id=?", collectionId)); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (r *UserLikesStore) UnLikeCollection(ctx context.Context, userId, collectionId int64) error {
	err := r.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var userLikes UserLike
		if err := assertAffectedOneRow(r.db.Core.NewDelete().Model(&userLikes).Where("user_id = ? and collection_id = ?", userId, collectionId).Exec(ctx)); err != nil {
			return err
		}

		if err := assertAffectedOneRow(tx.Exec("update collections set likes=COALESCE(likes, 1)-1 where id=?", collectionId)); err != nil {
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

func (r *UserLikesStore) IsExist(ctx context.Context, username string, repoId int64) (exists bool, err error) {
	var userLike UserLike
	exists, err = r.db.Operator.Core.
		NewSelect().
		Model(&userLike).
		Join("JOIN users ON users.id = user_like.user_id").
		Where("user_like.repo_id = ? and users.username = ?", repoId, username).
		Exists(ctx)
	return
}

func (r *UserLikesStore) IsExistCollection(ctx context.Context, username string, collectionId int64) (exists bool, err error) {
	var userLike UserLike
	exists, err = r.db.Operator.Core.
		NewSelect().
		Model(&userLike).
		Join("JOIN users ON users.id = user_like.user_id").
		Where("user_like.collection_id = ? and users.username = ?", collectionId, username).
		Exists(ctx)
	return
}
