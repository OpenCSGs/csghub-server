package database

import (
	"context"
	"fmt"
)

type AccountUserStore struct {
	db *DB
}

func NewAccountUserStore() *AccountUserStore {
	return &AccountUserStore{
		db: defaultDB,
	}
}

type AccountUser struct {
	ID      int64   `bun:",pk,autoincrement" json:"id"`
	UserID  string  `bun:",notnull" json:"user_id"` // casdoor uuid
	Balance float64 `bun:",notnull" json:"balance"`
}

func (s *AccountUserStore) List(ctx context.Context) ([]AccountUser, error) {
	var result []AccountUser
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("list all accounts, error:%w", err)
	}
	return result, nil
}

func (s *AccountUserStore) Create(ctx context.Context, input AccountUser) error {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create user balance failed,error:%w", err)
	}
	return nil
}

func (s *AccountUserStore) FindUserByID(ctx context.Context, userID string) (*AccountUser, error) {
	user := &AccountUser{}
	err := s.db.Core.NewSelect().Model(user).Where("user_id = ?", userID).Scan(ctx, user)
	return user, err
}
