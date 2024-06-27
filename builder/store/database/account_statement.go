package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
	commonTypes "opencsg.com/csghub-server/common/types"
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
	ID          int64     `bun:",pk,autoincrement" json:"id"`
	EventUUID   uuid.UUID `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID    string    `bun:",notnull" json:"user_uuid"`
	Value       float64   `bun:",notnull" json:"value"`
	Scene       SceneType `bun:",notnull" json:"scene"`
	OpUID       int64     `bun:",nullzero" json:"op_uid"`
	CreatedAt   time.Time `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	CustomerID  string    `json:"customer_id"`
	EventDate   time.Time `bun:"type:date" json:"event_date"`
	Price       float64   `json:"price"`
	PriceUnit   string    `json:"price_unit"`
	Consumption float64   `json:"consumption"`
}

type AccountStatementRes struct {
	Data []AccountStatement `json:"data"`
	types.ACCT_SUMMARY
}

func (as *AccountStatementStore) Create(ctx context.Context, input AccountStatement, changeValue float64) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return err
		}
		runSql := "update account_users set balance=balance + ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, changeValue, input.UserUUID)); err != nil {
			return err
		}

		if input.Scene == SceneModelInference || input.Scene == SceneSpace || input.Scene == SceneModelFinetune || input.Scene == SceneStarship {
			// calculate bill
			bill := AccountBill{
				BillDate:    input.EventDate,
				UserUUID:    input.UserUUID,
				Scene:       input.Scene,
				CustomerID:  input.CustomerID,
				Value:       changeValue,
				Consumption: input.Consumption,
			}
			err := tx.NewSelect().Model(&bill).Where("bill_date = ? and user_uuid = ? and scene = ? and customer_id = ?", input.EventDate, input.UserUUID, input.Scene, input.CustomerID).Scan(ctx)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if errors.Is(err, sql.ErrNoRows) {
				_, err = tx.NewInsert().Model(&bill).Exec(ctx)
			} else {
				_, err = tx.NewUpdate().Model(&bill).Where("bill_date = ? and user_uuid = ? and scene = ? and customer_id = ?", input.EventDate, input.UserUUID, input.Scene, input.CustomerID).Set("value = value + ?, consumption = consumption + ?, updated_at=current_timestamp", input.Value, input.Consumption).Exec(ctx)
			}
			return err
		}

		return nil
	})

	return err
}

func (as *AccountStatementStore) ListByUserIDAndTime(ctx context.Context, req commonTypes.ACCT_STATEMENTS_REQ) (AccountStatementRes, error) {
	var accountStatment []AccountStatement
	q := as.db.Operator.Core.NewSelect().Model(&accountStatment).Where("user_uuid = ? and scene = ? and customer_id = ? and created_at >= ? and created_at <= ?", req.UserUUID, req.Scene, req.InstanceName, req.StartTime, req.EndTime)

	count, err := q.Count(ctx)
	if err != nil {
		return AccountStatementRes{}, err
	}

	var totalResult TotalResult
	err = as.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value, SUM(consumption) as total_consumption").Scan(ctx, &totalResult)
	if err != nil {
		return AccountStatementRes{}, err
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountStatment)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("list all statement, error:%w", err)
	}
	return AccountStatementRes{
		Data: accountStatment,
		ACCT_SUMMARY: types.ACCT_SUMMARY{
			Total:            count,
			TotalValue:       totalResult.TotalValue,
			TotalConsumption: totalResult.TotalConsumption},
	}, nil
}

func (as *AccountStatementStore) GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error) {
	var result AccountStatement
	_, err := as.db.Core.NewSelect().Model(&result).Where("event_uuid = ?", eventID).Exec(ctx, &result)
	return result, err
}
