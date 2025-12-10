package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/common/types"
)

type accountStatementStoreImpl struct {
	db *DB
}

type AccountStatementStore interface {
	Create(ctx context.Context, input AccountStatement) error
	ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (AccountStatementRes, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error)
	ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (AccountStatementRes, error)
	ListStatementByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error)
}

func NewAccountStatementStore() AccountStatementStore {
	return &accountStatementStoreImpl{
		db: defaultDB,
	}
}

func NewAccountStatementStoreWithDB(db *DB) AccountStatementStore {
	return &accountStatementStoreImpl{
		db: db,
	}
}

type AccountStatement struct {
	ID               int64                 `bun:",pk,autoincrement" json:"id"`
	EventUUID        uuid.UUID             `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID         string                `bun:",notnull" json:"user_uuid"`
	Value            float64               `bun:",notnull" json:"value"`
	Scene            types.SceneType       `bun:",notnull" json:"scene"`
	OpUID            string                `bun:",nullzero" json:"op_uid"`
	CreatedAt        time.Time             `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	CustomerID       string                `json:"customer_id"`
	EventDate        time.Time             `bun:"type:date" json:"event_date"`
	Price            float64               `json:"price"`
	PriceUnit        string                `json:"price_unit"`
	Consumption      float64               `json:"consumption"`
	ValueType        types.ChargeValueType `json:"value_type"`
	ResourceID       string                `json:"resource_id"`
	ResourceName     string                `json:"resource_name"`
	SkuID            int64                 `json:"sku_id"`
	RecordedAt       time.Time             `json:"recorded_at"`
	SkuUnit          int64                 `json:"sku_unit"`
	SkuUnitType      types.SkuUnitType     `json:"sku_unit_type"`
	SkuPriceCurrency string                `json:"sku_price_currency"`
	BalanceType      string                `json:"balance_type"`
	BalanceValue     float64               `json:"balance_value"`
	IsCancel         bool                  `json:"is_cancel"`
	EventValue       float64               `json:"event_value"`
	Present          *AccountPresent       `bun:"rel:has-one,join:event_uuid=event_uuid"`
	Quota            float64               `bun:",nullzero" json:"quota"`
	SubBillID        int64                 `bun:",nullzero" json:"sub_bill_id"`
	Discount         float64               `json:"discount"`
	RegularValue     float64               `json:"regular_value"`
}

type AccountStatementRes struct {
	Data []AccountStatement `json:"data"`
	types.AcctSummary
}

func (as *accountStatementStoreImpl) Create(ctx context.Context, input AccountStatement) error {
	if input.Scene == types.ScenePortalCharge || input.Scene == types.SceneCashCharge {
		return as.chargeFeeStatement(ctx, input)
	} else {
		return as.deductFeeStatement(ctx, input)
	}
}

func (as *accountStatementStoreImpl) chargeFeeStatement(ctx context.Context, input AccountStatement) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.Value <= 0 {
			return fmt.Errorf("charge fee statement value must be positive, got: %f", input.Value)
		}
		var err error
		var acctUser AccountUser

		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				newAcctUser := AccountUser{
					UserUUID:    input.UserUUID,
					Balance:     0,
					CashBalance: 0,
				}
				res, err := tx.NewInsert().Model(&newAcctUser).Exec(ctx, &newAcctUser)
				if err := assertAffectedOneRow(res, err); err != nil {
					return fmt.Errorf("insert user account failed, error:%w", err)
				}
			} else {
				return fmt.Errorf("failed to get account user %s, err: %w", input.UserUUID, err)
			}
		}

		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE").Scan(ctx, &acctUser)
		if err != nil {
			return fmt.Errorf("failed to get account user %s for lock: %w", input.UserUUID, err)
		}

		err = CheckDuplicatedEvent(ctx, tx, input)
		if err != nil {
			if errors.Is(err, types.ErrDuplicatedEvent) {
				slog.Warn("skip duplicated charge fee by event uuid", slog.Any("input", input))
				return nil
			}
			return fmt.Errorf("check duplicated charge event failed, error: %w", err)
		}

		runSql := ""
		if input.Scene == types.SceneCashCharge {
			input.BalanceType = types.ChargeCashBalance
			runSql = "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
		} else {
			input.BalanceType = types.ChargeBalance
			runSql = "update account_users set balance=balance + ? where user_uuid=?"
		}

		err = assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID))
		if err != nil {
			return fmt.Errorf("update %s, error:%w", input.BalanceType, err)
		}

		acctUser = AccountUser{}
		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return fmt.Errorf("failed to get account user: %w", err)
		}

		if acctUser.Balance+acctUser.CashBalance >= 0 {
			runSql := "update account_users set negative_balance_warn_at=null where user_uuid=?"
			err = assertAffectedOneRow(tx.Exec(runSql, input.UserUUID))
			if err != nil {
				return fmt.Errorf("update account user %s negative balance warn at to null, error: %w", input.UserUUID, err)
			}
		}

		if input.Scene == types.SceneCashCharge {
			input.BalanceValue = acctUser.CashBalance
		} else {
			input.BalanceValue = acctUser.Balance
		}

		err = assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
		if err != nil {
			return fmt.Errorf("insert statement, error:%w", err)
		}
		return nil
	})

	return err
}

func (as *accountStatementStoreImpl) deductFeeStatement(ctx context.Context, input AccountStatement) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.Value > 0 {
			return fmt.Errorf("deduct fee statement value must be negative or zero")
		}

		var err error

		err = DeductAccountFee(ctx, tx, input)
		if err != nil {
			if errors.Is(err, types.ErrDuplicatedEvent) {
				slog.Warn("skip duplicated deduct fee by event uuid", slog.Any("input", input))
				return nil
			}
			return fmt.Errorf("deduct account fee, error: %w", err)
		}

		err = updateFeeBill(ctx, tx, input)
		if err != nil {
			return fmt.Errorf("update account bill, error: %w", err)
		}

		isCSGUsage := utils.IsNeedCheckUserSubscription(input.Scene)
		if isCSGUsage {
			reqSkuType := utils.GetSKUTypeByScene(types.SceneType(input.Scene))
			err = UpdateSubscriptionUsage(ctx, tx, input, reqSkuType)
			if err != nil {
				return fmt.Errorf("update subscription usage for %d, error: %w", reqSkuType, err)
			}
		}

		return nil
	})

	return err
}

func DeductAccountFee(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	var err error
	var acctUser AccountUser

	// add lock
	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE").Scan(ctx, &acctUser)
	if err != nil {
		return fmt.Errorf("failed to get account user: %w", err)
	}

	err = CheckDuplicatedEvent(ctx, tx, input)
	if err != nil {
		if errors.Is(err, types.ErrDuplicatedEvent) {
			// return duplicated error for check
			return err
		}
		return fmt.Errorf("check duplicated event failed, error: %w", err)
	}

	remainValue := input.Value

	if remainValue == 0 {
		// only save statement
		err = assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
		if err != nil {
			return fmt.Errorf("insert statement, error:%w", err)
		}
		return nil
	}

	// 1. check and reduce cash
	if remainValue < 0 && acctUser.CashBalance > 0 {
		remainValue, err = deductFeeToCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return fmt.Errorf("deduct cash balance, error:%w", err)
		}
	}

	// 2. check and reduce bonus
	if remainValue < 0 && acctUser.Balance > 0 {
		remainValue, err = deductFeeToBalance(ctx, tx, input, remainValue, acctUser.Balance)
		if err != nil {
			return fmt.Errorf("deduct balance, error:%w", err)
		}
	}

	// 3. check and reduce all in cash
	if remainValue < 0 {
		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return fmt.Errorf("failed to get account user: %w", err)
		}

		_, err = deductFeeAllInCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return fmt.Errorf("deduct all in balance, error:%w", err)
		}
	}

	return nil
}

func deductFeeToBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, balanceValue float64) (float64, error) {
	if balanceValue <= 0 {
		return 0, fmt.Errorf("balance is not enough")
	}
	statementValue := 0.0
	remainValue := balanceValue + feeValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - balanceValue
	}

	input.Value = statementValue
	input.BalanceValue = balanceValue + statementValue
	input.BalanceType = types.ChargeBalance

	runSql := "update account_users set balance=balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, fmt.Errorf("update balance, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, fmt.Errorf("insert statement for deduct balance, error:%w", err)
	}
	return remainValue, nil
}

func deductFeeToCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, error) {
	if cashBalanceValue <= 0 {
		return 0, fmt.Errorf("cash balance is not enough")
	}
	statementValue := 0.0
	remainValue := cashBalanceValue + feeValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - cashBalanceValue
	}

	input.Value = statementValue
	input.BalanceValue = cashBalanceValue + statementValue
	input.BalanceType = types.ChargeCashBalance

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, fmt.Errorf("update cash balance, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, fmt.Errorf("insert statement for deduct cash balance, error:%w", err)
	}
	return remainValue, nil
}

func deductFeeAllInCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, error) {
	remainValue := cashBalanceValue + feeValue

	input.Value = feeValue
	input.BalanceValue = remainValue
	input.BalanceType = types.ChargeCashBalance

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, fmt.Errorf("update balance for allin, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, fmt.Errorf("insert statement for deduct allin balance, error:%w", err)
	}

	return remainValue, nil
}

func CheckDuplicatedEvent(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	var err error
	var acctStatement AccountStatement

	err = tx.NewSelect().Model(&acctStatement).Where("event_uuid = ?", input.EventUUID).Scan(ctx, &acctStatement)
	if err == nil {
		return types.ErrDuplicatedEvent
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("verfiy account statement, event_uuid: %s, err: %w", input.EventUUID, err)
	}

	return nil
}

func updateFeeBill(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	if !utils.IsNeedCalculateBill(input.Scene) {
		return nil
	}
	// calculate bill
	bill := AccountBill{
		BillDate:    input.EventDate,
		UserUUID:    input.UserUUID,
		Scene:       input.Scene,
		CustomerID:  input.CustomerID,
		Value:       input.Value,
		Consumption: input.Consumption,
	}
	err := tx.NewSelect().Model(&bill).Where("bill_date = ? and user_uuid = ? and scene = ? and customer_id = ?", input.EventDate, input.UserUUID, input.Scene, input.CustomerID).For("UPDATE").Scan(ctx)
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

	return nil
}

func (as *accountStatementStoreImpl) ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (AccountStatementRes, error) {
	var accountStatment []AccountStatement
	q := as.db.Operator.Core.NewSelect().Model(&accountStatment)

	if req.UserUUID != "" {
		q = q.Where("user_uuid = ?", req.UserUUID)
	}

	if req.Scene != 0 {
		q = q.Where("scene = ?", req.Scene)
	}

	if req.InstanceName != "" {
		q = q.Where("customer_id = ?", req.InstanceName)
	}
	if req.StartTime != "" && req.EndTime != "" {
		q = q.Where("created_at >= ? and created_at <= ?", req.StartTime, req.EndTime)
	}

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
		AcctSummary: types.AcctSummary{
			Total:            count,
			TotalValue:       totalResult.TotalValue,
			TotalConsumption: totalResult.TotalConsumption},
	}, nil
}

func (as *accountStatementStoreImpl) GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error) {
	var result AccountStatement
	_, err := as.db.Core.NewSelect().Model(&result).Where("event_uuid = ?", eventID).Exec(ctx, &result)
	if err != nil {
		return result, fmt.Errorf("get statement, error:%w", err)
	}
	return result, nil
}

func (as *accountStatementStoreImpl) ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (AccountStatementRes, error) {
	var accountStatment []AccountStatement
	q := as.db.Operator.Core.NewSelect().Model(&accountStatment).Relation("Present").
		Where("account_statement.user_uuid = ?", req.UserUUID).
		Where("scene = ?", req.Scene).
		Where("customer_id = ?", "").
		Where("account_statement.created_at >= ? and account_statement.created_at <= ?", req.StartTime, req.EndTime)

	if req.ActivityID > 0 {
		q = q.Where("activity_id = ?", req.ActivityID)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("count statement, error:%w", err)
	}

	var totalResult TotalResult
	err = as.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value").Scan(ctx, &totalResult)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("group statement, error:%w", err)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountStatment)
	if err != nil {
		return AccountStatementRes{}, fmt.Errorf("list statement, error:%w", err)
	}
	return AccountStatementRes{
		Data: accountStatment,
		AcctSummary: types.AcctSummary{
			Total:      count,
			TotalValue: totalResult.TotalValue,
		},
	}, nil
}

type UserSkuStatement struct {
	ID               int64              `json:"id"`
	UserUUID         string             `json:"user_uuid"`
	SkuID            int64              `json:"sku_id"`
	Scene            types.SceneType    `json:"scene"`
	CustomerID       string             `json:"customer_id"`
	CreatedAt        time.Time          `json:"created_at"`
	TotalValue       float64            `json:"total_value"`
	TotalConsumption float64            `json:"total_consumption"`
}

type GroupedStatementRes struct {
	Data       []UserSkuStatement `json:"data"`
	TotalCount int                `json:"total_count"`
}

func (as *accountStatementStoreImpl) ListStatementByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error) {
	var results []UserSkuStatement
	baseQuery := as.db.Operator.Core.NewSelect().
		Model((*AccountStatement)(nil)).
		Column("user_uuid", "sku_id", "scene", "customer_id").
		ColumnExpr("MIN(id) AS id").
		ColumnExpr("MIN(created_at) AS created_at").
		ColumnExpr("SUM(value) AS total_value").
		ColumnExpr("SUM(consumption) AS total_consumption").
		Group("user_uuid", "sku_id", "scene", "customer_id")

	if req.UserUUID != "" {
		baseQuery = baseQuery.Where("user_uuid = ?", req.UserUUID)
	}

	if req.Scene != 0 {
		baseQuery = baseQuery.Where("scene = ?", req.Scene)
	}

	if req.InstanceName != "" {
		baseQuery = baseQuery.Where("customer_id = ?", req.InstanceName)
	}

	if req.StartTime != "" && req.EndTime != "" {
		baseQuery = baseQuery.Where("created_at >= ? AND created_at <= ?", req.StartTime, req.EndTime)
	}

	var totalCount int
	countQuery := as.db.Operator.Core.NewSelect().
		With("grouped_statements", baseQuery).
		TableExpr("grouped_statements").
		ColumnExpr("COUNT(*)")

	err := countQuery.Scan(ctx, &totalCount)
	if err != nil {
		return results, 0, fmt.Errorf("count grouped statements error: %w", err)
	}

	selectQuery := as.db.Operator.Core.NewSelect().
		With("grouped_statements", baseQuery).
		TableExpr("grouped_statements").
		Column("id", "user_uuid", "sku_id", "scene", "customer_id", "created_at", "total_value", "total_consumption").
		OrderExpr("user_uuid ASC, sku_id ASC")

	if req.Per > 0 {
		selectQuery = selectQuery.
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per)
	}

	err = selectQuery.Scan(ctx, &results)
	if err != nil {
		return results, totalCount, fmt.Errorf("list grouped statements error: %w", err)
	}

	return results, totalCount, nil
}
