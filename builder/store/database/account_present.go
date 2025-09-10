package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type accountPresentStoreImpl struct {
	db *DB
}

type AccountPresentStore interface {
	AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error
	FindPresentByUserIDAndScene(ctx context.Context, userID string, activityID int64) (*AccountPresent, error)
}

func NewAccountPresentStore() AccountPresentStore {
	return &accountPresentStoreImpl{
		db: defaultDB,
	}
}

func NewAccountPresentStoreWithDB(db *DB) AccountPresentStore {
	return &accountPresentStoreImpl{
		db: db,
	}
}

type AccountPresent struct {
	ID         int64     `bun:",pk,autoincrement" json:"id"`
	EventUUID  uuid.UUID `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID   string    `bun:",notnull" json:"user_uuid"`
	ActivityID int64     `bun:",notnull" json:"activity_id"`
	Value      float64   `bun:",notnull" json:"value"`
	OpUID      string    `bun:",notnull" json:"op_uid"`
	OpDesc     string    `bun:",notnull" json:"op_desc"`
	times
}

func (ap *accountPresentStoreImpl) AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error {
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account present, error:%w", err)
		}

		if err := assertAffectedOneRow(tx.NewInsert().Model(&statement).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account statement, error:%w", err)
		}

		runSql := "update account_users set balance=balance + ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
			return fmt.Errorf("update account balance, error:%w", err)
		}

		return nil
	})

	return err
}

func (ap *accountPresentStoreImpl) FindPresentByUserIDAndScene(ctx context.Context, userID string, activityID int64) (*AccountPresent, error) {
	present := &AccountPresent{}
	err := ap.db.Core.NewSelect().Model(present).Where("user_uuid = ? and activity_id = ?", userID, activityID).Limit(1).Scan(ctx, present)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find account present by user id and activity id, error:%w", err)
	}
	return present, nil
}
