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
	ID       int64   `bun:",pk,autoincrement" json:"id"`
	UserUUID string  `bun:",notnull" json:"user_uuid"`
	Balance  float64 `bun:",notnull" json:"balance"`
}

func (s *AccountUserStore) List(ctx context.Context, per, page int) ([]AccountUser, int, error) {
	var result []AccountUser
	q := s.db.Operator.Core.NewSelect().Model(&result)
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	_, err = q.Order("user_id").Limit(per).Offset((page-1)*per).Exec(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("list all accounts, error:%w", err)
	}
	return result, count, nil
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
	err := s.db.Core.NewSelect().Model(user).Where("user_uuid = ?", userID).Scan(ctx, user)
	return user, err
}
