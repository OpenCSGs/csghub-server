package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type UserVerifyStoreImpl struct {
	db *DB
}

type UserVerifyStore interface {
	CreateUserVerify(ctx context.Context, user *UserVerify) (*UserVerify, error)
	UpdateUserVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*UserVerify, error)
	GetUserVerify(ctx context.Context, uuid string) (*UserVerify, error)
}

func NewUserVerifyStore() UserVerifyStore {
	return &UserVerifyStoreImpl{
		db: defaultDB,
	}
}

func NewUserVerifyStoreWithDB(db *DB) UserVerifyStore {
	return &UserVerifyStoreImpl{
		db: db,
	}
}

type UserVerify struct {
	ID          int64              `bun:",pk,autoincrement" json:"id"`
	UUID        string             `bun:",unique,notnull" json:"uuid"`
	RealName    string             `bun:",notnull" json:"real_name"`
	Username    string             `bun:",notnull" json:"username"`
	IDCardFront string             `bun:",notnull" json:"id_card_front"`
	IDCardBack  string             `bun:",notnull" json:"id_card_back"`
	Status      types.VerifyStatus `bun:",notnull,default:'pending'" json:"status"` // pending, approved, rejected
	Reason      string             `bun:",nullzero" json:"reason,omitempty"`
	times
}

func (uv *UserVerifyStoreImpl) CreateUserVerify(ctx context.Context, userVerify *UserVerify) (*UserVerify, error) {
	_, err := uv.db.Operator.Core.NewInsert().
		Model(userVerify).
		On("CONFLICT (uuid) DO UPDATE").
		Set("real_name = EXCLUDED.real_name").
		Set("username = EXCLUDED.username").
		Set("id_card_front = EXCLUDED.id_card_front").
		Set("id_card_back = EXCLUDED.id_card_back").
		Set("status = EXCLUDED.status").
		Set("reason = EXCLUDED.reason").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to insert or update user verify: %w", err)
	}
	return userVerify, nil
}

func (uv *UserVerifyStoreImpl) UpdateUserVerify(ctx context.Context, id int64, status types.VerifyStatus, reason string) (*UserVerify, error) {
	userVerify := &UserVerify{
		ID:     id,
		Status: status,
		Reason: reason,
	}

	_, err := uv.db.Operator.Core.NewUpdate().
		Model(userVerify).
		Column("status", "reason").
		WherePK().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user verify: %w", err)
	}

	return uv.GetUserVerifyById(ctx, id)
}

func (uv *UserVerifyStoreImpl) GetUserVerifyById(ctx context.Context, id int64) (*UserVerify, error) {
	userVerify := new(UserVerify)
	err := uv.db.Operator.Core.NewSelect().
		Model(userVerify).
		Where("id = ?", id).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return userVerify, nil
}

func (uv *UserVerifyStoreImpl) GetUserVerify(ctx context.Context, uuid string) (*UserVerify, error) {
	userVerify := new(UserVerify)
	err := uv.db.Operator.Core.NewSelect().
		Model(userVerify).
		Where("uuid = ?", uuid).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get organization verify: %w", err)
	}
	return userVerify, nil
}
