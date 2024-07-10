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
)

type AccountStatementStore struct {
	db *DB
}

func NewAccountStatementStore() *AccountStatementStore {
	return &AccountStatementStore{
		db: defaultDB,
	}
}

type AccountStatement struct {
	ID               int64           `bun:",pk,autoincrement" json:"id"`
	EventUUID        uuid.UUID       `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID         string          `bun:",notnull" json:"user_uuid"`
	Value            float64         `bun:",notnull" json:"value"`
	Scene            types.SceneType `bun:",notnull" json:"scene"`
	OpUID            string          `bun:",nullzero" json:"op_uid"`
	CreatedAt        time.Time       `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	CustomerID       string          `json:"customer_id"`
	EventDate        time.Time       `bun:"type:date" json:"event_date"`
	Price            float64         `json:"price"`
	PriceUnit        string          `json:"price_unit"`
	Consumption      float64         `json:"consumption"`
	ValueType        int             `json:"value_type"`
	ResourceID       string          `json:"resource_id"`
	ResourceName     string          `json:"resource_name"`
	SkuID            int64           `json:"sku_id"`
	RecordedAt       time.Time       `json:"recorded_at"`
	SkuUnit          int64           `json:"sku_unit"`
	SkuUnitType      string          `json:"sku_unit_type"`
	SkuPriceCurrency string          `json:"sku_price_currency"`
}

type AccountStatementRes struct {
	Data []AccountStatement `json:"data"`
	types.ACCT_SUMMARY
}

func (as *AccountStatementStore) Create(ctx context.Context, input AccountStatement) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return fmt.Errorf("insert statement, error:%w", err)
		}
		runSql := "update account_users set balance=balance + ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, (input.Value / 100), input.UserUUID)); err != nil {
			return fmt.Errorf("update balance, error:%w", err)
		}

		if input.Scene == types.SceneModelInference || input.Scene == types.SceneSpace || input.Scene == types.SceneModelFinetune || input.Scene == types.SceneStarship {
			// calculate bill
			bill := AccountBill{
				BillDate:    input.EventDate,
				UserUUID:    input.UserUUID,
				Scene:       input.Scene,
				CustomerID:  input.CustomerID,
				Value:       input.Value,
				Consumption: input.Consumption,
			}
			err := tx.NewSelect().Model(&bill).Where("bill_date = ? and user_uuid = ? and scene = ? and customer_id = ?", input.EventDate, input.UserUUID, input.Scene, input.CustomerID).Scan(ctx)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("select statement, error:%w", err)
			}
			if errors.Is(err, sql.ErrNoRows) {
				_, err = tx.NewInsert().Model(&bill).Exec(ctx)
				if err != nil {
					return fmt.Errorf("create statement, error:%w", err)
				}
			} else {
				_, err = tx.NewUpdate().Model(&bill).Where("bill_date = ? and user_uuid = ? and scene = ? and customer_id = ?", input.EventDate, input.UserUUID, input.Scene, input.CustomerID).Set("value = value + ?, consumption = consumption + ?, updated_at=current_timestamp", input.Value, input.Consumption).Exec(ctx)
				if err != nil {
					return fmt.Errorf("update statement, error:%w", err)
				}
			}
		}
		return nil
	})

	return err
}

func (as *AccountStatementStore) ListByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (AccountStatementRes, error) {
	var accountStatment []AccountStatement
	q := as.db.Operator.Core.NewSelect().Model(&accountStatment).Where("user_uuid = ? and scene = ? and customer_id = ? and created_at >= ? and created_at <= ?", req.UserUUID, req.Scene, req.InstanceName, req.StartTime, req.EndTime)

	count, err := q.Count(ctx)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("count statement, error:%w", err)
	}

	var totalResult TotalResult
	err = as.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value, SUM(consumption) as total_consumption").Scan(ctx, &totalResult)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("group statement, error:%w", err)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountStatment)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("list statement, error:%w", err)
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
	if err != nil {
		return result, fmt.Errorf("get statement, error:%w", err)
	}
	return result, nil
}
