package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AccountStatementStore struct {
	db *DB
}

func NewAccountStatementStore() *AccountStatementStore {
	return &AccountStatementStore{
		db: defaultDB,
	}
}

type SceneType int

var (
	SceneReserve        SceneType = 0  // system reserve
	ScenePortalCharge   SceneType = 1  // portal charge fee
	SceneModelInference SceneType = 10 // model inference endpoint
	SceneSpace          SceneType = 11 // csghub space
	SceneModelFinetune  SceneType = 12 // model finetune
	SceneStarship       SceneType = 20 // starship
	SceneUnknow         SceneType = 99 // unknow
)

type AccountStatement struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	EventUUID uuid.UUID `bun:"type:uuid,notnull" json:"event_uuid"`
	UserID    string    `bun:",notnull" json:"user_id"`
	Value     float64   `bun:",notnull" json:"value"`
	Scene     SceneType `bun:",notnull" json:"scene"`
	OpUID     int64     `bun:",nullzero" json:"op_uid"`
	CreatedAt time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

func (as *AccountStatementStore) Create(ctx context.Context, input AccountStatement, changeValue float64) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return err
		}
		sql := "update account_users set balance=balance + ? where user_id=?"
		if err := assertAffectedOneRow(tx.Exec(sql, changeValue, input.UserID)); err != nil {
			return err
		}
		return nil
	})

	return err
}

func (as *AccountStatementStore) ListByUserIDAndTime(ctx context.Context, userID, startTime, endTime string) ([]AccountStatement, error) {
	var result []AccountStatement
	_, err := as.db.Operator.Core.NewSelect().Model(&result).Where("user_id = ? and created_at >= ? and created_at <= ?", userID, startTime, endTime).Exec(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("list all statement, error:%w", err)
	}
	return result, nil
}

func (as *AccountStatementStore) GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error) {
	var result AccountStatement
	_, err := as.db.Core.NewSelect().Model(&result).Where("event_uuid = ?", eventID).Exec(ctx, &result)
	return result, err
}
