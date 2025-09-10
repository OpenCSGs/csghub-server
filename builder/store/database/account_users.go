package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

type accountUserStoreImpl struct {
	db *DB
}

type AccountUserStore interface {
	List(ctx context.Context, per, page int) ([]AccountUser, int, error)
	Create(ctx context.Context, input AccountUser) error
	FindUserByID(ctx context.Context, userID string) (*AccountUser, error)
	ListAllByUserUUID(ctx context.Context, userUUID string) ([]AccountUser, error)
	SetLowBalanceWarn(ctx context.Context, userUUID string, lowBalanceWarn float64) error
	SetLowBalanceWarnAtNow(ctx context.Context, userUUID string) error
	UpdateNegativeBalanceWarnAt(ctx context.Context, userUUID string, warnAt time.Time) error
}

func NewAccountUserStore() AccountUserStore {
	return &accountUserStoreImpl{
		db: defaultDB,
	}
}

func NewAccountUserStoreWithDB(db *DB) AccountUserStore {
	return &accountUserStoreImpl{
		db: db,
	}
}

type AccountUser struct {
	ID                    int64     `bun:",pk,autoincrement" json:"id"`
	UserUUID              string    `bun:",notnull" json:"user_uuid"`
	Balance               float64   `bun:",notnull" json:"balance"`
	CashBalance           float64   `bun:",notnull" json:"cash_balance"`
	LowBalanceWarn        float64   `bun:",notnull,default:0" json:"low_balance_warn"`
	LowBalanceWarnAt      time.Time `bun:",nullzero" json:"low_balance_warn_at"`
	NegativeBalanceWarnAt time.Time `bun:",nullzero" json:"negative_balance_warn_at"`
}

func (s *accountUserStoreImpl) List(ctx context.Context, per, page int) ([]AccountUser, int, error) {
	var result []AccountUser
	q := s.db.Operator.Core.NewSelect().Model(&result)
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	_, err = q.Order("user_uuid").Limit(per).Offset((page-1)*per).Exec(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("list all accounts, error:%w", err)
	}
	return result, count, nil
}

func (s *accountUserStoreImpl) Create(ctx context.Context, input AccountUser) error {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create user balance failed,error:%w", err)
	}
	return nil
}

func (s *accountUserStoreImpl) FindUserByID(ctx context.Context, userID string) (*AccountUser, error) {
	user := &AccountUser{}
	err := s.db.Core.NewSelect().Model(user).Where("user_uuid = ?", userID).Scan(ctx, user)
	return user, err
}

func (am *accountUserStoreImpl) ListAllByUserUUID(ctx context.Context, userUUID string) ([]AccountUser, error) {
	var accountUsers []AccountUser
	err := am.db.Operator.Core.NewSelect().Model(&accountUsers).Where("user_uuid = ?", userUUID).Scan(ctx, &accountUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to list all account users by user uuid: %w", err)
	}
	return accountUsers, nil
}

func (am *accountUserStoreImpl) SetLowBalanceWarn(ctx context.Context, userUUID string, lowBalanceWarn float64) error {
	_, err := am.db.Core.NewInsert().
		Model(&AccountUser{
			UserUUID:       userUUID,
			LowBalanceWarn: lowBalanceWarn,
			Balance:        0,
			CashBalance:    0,
		}).
		On("CONFLICT (user_uuid) DO UPDATE").
		Set("low_balance_warn = EXCLUDED.low_balance_warn").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert low balance warn for user %s: %w", userUUID, err)
	}
	return nil
}

func (am *accountUserStoreImpl) SetLowBalanceWarnAtNow(ctx context.Context, userUUID string) error {
	_, err := am.db.Core.NewUpdate().
		Model((*AccountUser)(nil)).
		Set("low_balance_warn_at = ?", time.Now()).
		Where("user_uuid = ?", userUUID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update low_balance_warn_at for user %s: %w", userUUID, err)
	}
	return nil
}

func CheckUserAccount(ctx context.Context, tx bun.Tx, userUUID string) error {
	var acctUser AccountUser
	err := tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", userUUID).Scan(ctx, &acctUser)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return errorx.HandleDBError(err, nil)
	}

	newAcctUser := AccountUser{
		UserUUID:    userUUID,
		Balance:     0,
		CashBalance: 0,
	}
	res, err := tx.NewInsert().Model(&newAcctUser).Exec(ctx, &newAcctUser)

	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("cause", "create user account"))
	}
	return nil
}

func (am *accountUserStoreImpl) UpdateNegativeBalanceWarnAt(ctx context.Context, userUUID string, warnAt time.Time) error {
	res, err := am.db.Core.NewUpdate().
		Model((*AccountUser)(nil)).
		Set("negative_balance_warn_at = ?", warnAt).
		Where("user_uuid = ?", userUUID).
		Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to update negative balance warnat for user %s: %w", userUUID, err)
	}
	return nil
}
