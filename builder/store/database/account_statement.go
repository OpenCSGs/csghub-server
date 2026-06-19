package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
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
	Create(ctx context.Context, input AccountStatement, checkBalance ...bool) error
	ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (AccountStatementRes, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error)
	ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (AccountStatementRes, error)
	ListStatementByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error)
	ListPortalRecharges(ctx context.Context, req types.AcctRechargeListReq) ([]AccountStatement, int, float64, error)
	HasUserPurchasedDataset(ctx context.Context, userUUID string, datasetID int64) (bool, error)
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
	PromptToken      float64               `json:"prompt_token"`
	CompletionToken  float64               `json:"completion_token"`
	APIKey           string                `bun:",notnull,default:''" json:"api_key"`
	TokenID          int64                 `bun:",notnull,default:0" json:"token_id"`
	Purpose          types.RechargePurpose `bun:",nullzero" json:"purpose"`
	PurposeDesc      string                `bun:",nullzero" json:"purpose_desc"`
	DataType         string                `bun:",notnull,default:''" json:"data_type"`
	Resolution       string                `bun:",notnull,default:''" json:"resolution"`
	Duration         float64               `bun:",notnull,default:0" json:"duration"`
	VoucherNo        string                `bun:",nullzero" json:"voucher_no"`
}

type AccountStatementRes struct {
	Data []AccountStatement `json:"data"`
	types.AcctSummary
}

type ChangedValue struct {
	Cash    float64
	Voucher float64
}

func (as *accountStatementStoreImpl) Create(ctx context.Context, input AccountStatement, checkBalance ...bool) error {
	check := len(checkBalance) > 0 && checkBalance[0]
	if input.Scene == types.ScenePortalCharge || input.Scene == types.SceneCashCharge {
		return as.chargeFeeStatement(ctx, input)
	} else {
		return as.deductFeeStatement(ctx, input, check)
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

func (as *accountStatementStoreImpl) deductFeeStatement(ctx context.Context, input AccountStatement, checkBalance bool) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.Value > 0 {
			return fmt.Errorf("deduct fee statement value must be negative or zero")
		}

		var err error
		var changed ChangedValue
		initEventValue := input.Value

		if utils.IsGetTokenID(input.Scene) && len(input.APIKey) > 0 {
			token, err := findByTokenValue(ctx, tx, input.APIKey)
			if err != nil {
				slog.ErrorContext(ctx, "find token by value failed, error", slog.Any("error", err), slog.Any("input", input))
			}
			if token != nil {
				input.TokenID = token.ID
			} else {
				slog.WarnContext(ctx, "token is nil", slog.Any("input", input))
			}
		}

		changed, err = DeductAccountFee(ctx, tx, input, checkBalance)
		if err != nil {
			if errors.Is(err, types.ErrDuplicatedEvent) {
				slog.Warn("skip duplicated deduct fee by event uuid", slog.Any("input", input))
				return nil
			}
			return fmt.Errorf("deduct account fee, error: %w", err)
		}

		calcValue := initEventValue - changed.Voucher
		consumption := 0.0
		if initEventValue != 0 {
			consumption = math.Abs(calcValue/initEventValue) * input.Consumption
		} else {
			consumption = input.Consumption
		}

		err = updateFeeBill(ctx, tx, input, BillValues{
			TotalValue:   calcValue,
			VoucherValue: 0,
			CashValue:    changed.Cash,
			Consumption:  consumption,
		})

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

func DeductAccountFee(ctx context.Context, tx bun.Tx, input AccountStatement, checkBalance bool) (ChangedValue, error) {
	var err error
	var acctUser AccountUser
	initEventValue := input.Value
	changedValue := ChangedValue{
		Cash:    0.0,
		Voucher: 0.0,
	}

	// add lock
	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE").Scan(ctx, &acctUser)
	if err != nil {
		return changedValue, fmt.Errorf("failed to get account user: %w", err)
	}

	err = CheckDuplicatedEvent(ctx, tx, input)
	if err != nil {
		if errors.Is(err, types.ErrDuplicatedEvent) {
			// return duplicated error for check
			return changedValue, err
		}
		return changedValue, fmt.Errorf("check duplicated event failed, error: %w", err)
	}

	// Check balance if needed
	if checkBalance && input.Value < 0 {
		if acctUser.Balance+acctUser.CashBalance < -input.Value {
			return changedValue, fmt.Errorf("insufficient balance")
		}
	}

	remainValue := input.Value
	cashChangePart1 := 0.0
	cashChangePart2 := 0.0

	if remainValue == 0 {
		// only save statement
		err = assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
		if err != nil {
			return changedValue, fmt.Errorf("insert statement, error:%w", err)
		}
		return changedValue, nil
	}

	// 1. check and reduce voucher
	if remainValue < 0 && utils.IsUseVoucher(input.Scene) {
		remainValue, err = deductFeeToVouchers(ctx, tx, input, remainValue)
		if err != nil {
			return changedValue, fmt.Errorf("deduct voucher, error:%w", err)
		}
		changedValue.Voucher = initEventValue - remainValue
	}

	// 2. check and reduce cash
	if remainValue < 0 && acctUser.CashBalance > 0 {
		remainValue, cashChangePart1, err = deductFeeToCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct cash balance, error:%w", err)
		}
	}

	// 3. check and reduce bonus
	if remainValue < 0 && acctUser.Balance > 0 {
		remainValue, err = deductFeeToBalance(ctx, tx, input, remainValue, acctUser.Balance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct balance, error:%w", err)
		}
	}

	// 4. check and reduce all in cash
	if remainValue < 0 {
		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return changedValue, fmt.Errorf("failed to get account user: %w", err)
		}

		_, cashChangePart2, err = deductFeeAllInCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct all in balance, error:%w", err)
		}
	}
	changedValue.Cash = cashChangePart1 + cashChangePart2
	return changedValue, nil
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

func deductFeeToCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, float64, error) {
	if cashBalanceValue <= 0 {
		return 0, 0, fmt.Errorf("cash balance is not enough")
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
		return 0, 0, fmt.Errorf("update cash balance, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, 0, fmt.Errorf("insert statement for deduct cash balance, error:%w", err)
	}
	return remainValue, statementValue, nil
}

func deductFeeAllInCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, float64, error) {
	remainValue := cashBalanceValue + feeValue

	input.Value = feeValue
	input.BalanceValue = remainValue
	input.BalanceType = types.ChargeCashBalance

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, 0, fmt.Errorf("update balance for allin, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, 0, fmt.Errorf("insert statement for deduct allin balance, error:%w", err)
	}

	return remainValue, feeValue, nil
}

func deductFeeToVouchers(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue float64) (float64, error) {
	var err error
	remainValue := feeValue

	err = updateVouchersStatus(ctx, tx)
	if err != nil {
		return remainValue, fmt.Errorf("refresh vouchers status, error: %w", err)
	}

	resID, err := strconv.ParseInt(input.ResourceID, 10, 64)
	if err != nil {
		return remainValue, fmt.Errorf("bad request resource id format %s for voucher, error: %w", input.ResourceID, err)
	}

	var res SpaceResource
	res.ID = resID

	err = tx.NewSelect().Model(&res).WherePK().Scan(ctx)
	if err != nil {
		return remainValue, fmt.Errorf("query resource id %s for voucher, error: %w", input.ResourceID, err)
	}

	reqClusterID := strings.TrimSpace(res.ClusterID)
	reqXpuModel, err := utils.GetResXPUMode(res.Resources)
	if err != nil {
		return remainValue, fmt.Errorf("get resource xpu model for resource id %s for voucher, error: %w", input.ResourceID, err)
	}
	reqXpuModel = strings.TrimSpace(reqXpuModel)

	if len(reqClusterID) < 1 {
		return remainValue, fmt.Errorf("cluster id is empty for resource id %s for voucher", input.ResourceID)
	}

	if len(reqXpuModel) < 1 {
		return remainValue, nil
	}

	var vouchers []AccountVoucher
	err = tx.NewSelect().Model(&vouchers).
		Where("target_uuid = ?", input.UserUUID).
		Where("status = ?", types.VoucherStatusActive).
		Where("begin_date <= ?", time.Now()).
		Where("end_date > ?", time.Now()).
		Where("(total - ABS(used)) > 0").
		Order("end_date ASC", "created_at ASC").
		Scan(ctx)
	if err != nil {
		return remainValue, fmt.Errorf("query active vouchers for target %s: %w", input.UserUUID, err)
	}

	if len(vouchers) < 1 {
		return remainValue, nil
	}

	priorityVouchers := checkAndSortVouchers(vouchers, reqClusterID, reqXpuModel)

	if len(priorityVouchers) < 1 {
		return remainValue, nil
	}

	for _, v := range priorityVouchers {
		if remainValue >= 0 {
			break
		}
		remainValue, err = deductFeeToVoucher(ctx, tx, input, remainValue, v)
		if err != nil {
			return remainValue, fmt.Errorf("deduct single voucher id %d error: %w", v.ID, err)
		}
	}

	return remainValue, nil
}

func deductFeeToVoucher(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue float64, voucher AccountVoucher) (float64, error) {
	eventValue := input.Value
	remainValue := feeValue
	if remainValue >= 0 {
		return remainValue, nil
	}

	available := voucher.Total - math.Abs(voucher.Used)
	if available <= 0 {
		return remainValue, nil
	}

	statementValue := 0.0
	remainValue = available + remainValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - available
	}

	input.Value = statementValue
	input.BalanceValue = available + statementValue
	input.BalanceType = types.ChargeVoucher
	input.VoucherNo = voucher.VoucherNo

	err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("insert statement for deduct voucher id %d error:%w", voucher.ID, err)
	}

	_, err = tx.NewUpdate().Model(&AccountVoucher{}).
		Set("used = used + ?", statementValue).Set("updated_at = ?", time.Now()).
		Where("id = ?", voucher.ID).Exec(ctx)
	if err != nil {
		return remainValue, fmt.Errorf("update voucher id %d used amount error: %w", voucher.ID, err)
	}

	consumption := 0.0
	if eventValue != 0 {
		consumption = math.Abs(statementValue/eventValue) * input.Consumption
	}

	err = updateFeeBill(ctx, tx, input, BillValues{
		TotalValue:   statementValue,
		VoucherValue: statementValue,
		CashValue:    0.0,
		Consumption:  consumption,
	})
	if err != nil {
		return remainValue, fmt.Errorf("update voucher bill for voucher %s error: %w", input.VoucherNo, err)
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

func updateFeeBill(ctx context.Context, tx bun.Tx, input AccountStatement, billValues BillValues) error {
	if !utils.IsNeedCalculateBill(input.Scene) {
		return nil
	}
	if billValues.TotalValue != 0 {
		// calculate bill
		bill := AccountBill{
			BillDate:        input.EventDate,
			UserUUID:        input.UserUUID,
			Scene:           input.Scene,
			CustomerID:      input.CustomerID,
			Value:           billValues.TotalValue,
			Consumption:     billValues.Consumption,
			PromptToken:     input.PromptToken,
			CompletionToken: input.CompletionToken,
			Count:           1,
			TokenID:         input.TokenID,
			DataType:        input.DataType,
			Resolution:      input.Resolution,
			Duration:        input.Duration,
			VoucherNo:       input.VoucherNo,
			VoucherValue:    billValues.VoucherValue,
			CashValue:       billValues.CashValue,
		}
		if input.Scene == types.SceneMultiModalServerless {
			bill.UnitType = input.SkuUnitType
		}

		// depend on unique index for update
		_, err := tx.NewInsert().Model(&bill).
			On("CONFLICT (bill_date, user_uuid, scene, customer_id, token_id, data_type, resolution, voucher_no, unit_type) DO UPDATE").
			Set("value = account_bill.value + ?", billValues.TotalValue).
			Set("consumption = account_bill.consumption + ?", billValues.Consumption).
			Set("prompt_token = account_bill.prompt_token + ?", input.PromptToken).
			Set("completion_token = account_bill.completion_token + ?", input.CompletionToken).
			Set("duration = account_bill.duration + ?", input.Duration).
			Set("count = account_bill.count + ?", 1).
			Set("voucher_value = account_bill.voucher_value + ?", billValues.VoucherValue).
			Set("cash_value = account_bill.cash_value + ?", billValues.CashValue).
			Set("updated_at = current_timestamp").
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("update bill for %s user %s, error:%w", input.EventUUID, input.UserUUID, err)
		}
	}
	if len(input.APIKey) > 0 {
		err := UpdateAPIKeyUsage(ctx, tx, input)
		if err != nil {
			slog.WarnContext(ctx, "failed to update api key usage", slog.Any("err", err), slog.String("api_key", input.APIKey))
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
		Where("account_statement.user_uuid = ?", req.TargetUUID).
		Where("account_statement.scene = ?", req.Scene).
		Where("account_statement.customer_id = ?", "").
		Where("account_statement.created_at >= ? and account_statement.created_at <= ?", req.StartTime, req.EndTime)

	if req.ActivityID > 0 {
		q = q.Where("present.activity_id = ?", req.ActivityID)
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
	_, err = q.Order("account_statement.id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &accountStatment)
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
	ID               int64           `json:"id"`
	UserUUID         string          `json:"user_uuid"`
	SkuID            int64           `json:"sku_id"`
	Scene            types.SceneType `json:"scene"`
	CustomerID       string          `json:"customer_id"`
	CreatedAt        time.Time       `json:"created_at"`
	TotalValue       float64         `json:"total_value"`
	TotalConsumption float64         `json:"total_consumption"`
	TotalCount       int             `json:"-"`
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
		ColumnExpr("SUM(consumption) AS total_consumption")

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

	baseQuery = baseQuery.Group("user_uuid", "sku_id", "scene", "customer_id")

	selectQuery := as.db.Operator.Core.NewSelect().
		TableExpr("(?) AS grouped", baseQuery).
		Column("id", "user_uuid", "sku_id", "scene", "customer_id", "created_at", "total_value", "total_consumption").
		ColumnExpr("COUNT(*) OVER() AS total_count")

	if req.Per > 0 {
		selectQuery = selectQuery.
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per)
	}

	err := selectQuery.Scan(ctx, &results)
	if err != nil {
		return results, 0, fmt.Errorf("list grouped statements error: %w", err)
	}

	totalCount := 0
	if len(results) > 0 {
		totalCount = results[0].TotalCount
	}

	return results, totalCount, nil
}

func (as *accountStatementStoreImpl) ListPortalRecharges(ctx context.Context, req types.AcctRechargeListReq) ([]AccountStatement, int, float64, error) {
	var portalRecharges []AccountStatement
	q := as.db.Operator.Core.NewSelect().Model((*AccountStatement)(nil)).Relation("Present").
		Where("account_statement.scene = ?", req.Scene).
		Where("account_statement.created_at >= ? AND account_statement.created_at <= ?", req.StartTime, req.EndTime)
	if req.TargetUUID != "" {
		q = q.Where("account_statement.user_uuid = ?", req.TargetUUID)
	}
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("count portal recharges, error:%w", err)
	}
	var totalResult TotalResult
	err = as.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value").Scan(ctx, &totalResult)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("group portal recharges, error:%w", err)
	}
	_, err = q.Order("account_statement.id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &portalRecharges)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list portal recharges, error:%w", err)
	}
	return portalRecharges, count, totalResult.TotalValue, nil
}

func (as *accountStatementStoreImpl) HasUserPurchasedDataset(ctx context.Context, userUUID string, datasetID int64) (bool, error) {
	if as == nil || as.db == nil {
		return false, nil
	}
	var count int
	err := as.db.Operator.Core.NewSelect().
		Model((*AccountStatement)(nil)).
		ColumnExpr("COUNT(*)").
		Where("user_uuid = ?", userUUID).
		Where("scene = ?", types.SceneDatasetPurchase).
		Where("resource_id = ?", fmt.Sprintf("%d", datasetID)).
		Where("resource_name = ?", "dataset").
		Scan(ctx, &count)

	if err != nil {
		return false, fmt.Errorf("check if user purchased dataset error: %w", err)
	}

	return count > 0, nil
}

func checkAndSortVouchers(vouchers []AccountVoucher, clusterID, xpuModel string) []AccountVoucher {
	var (
		matchedBoth         []AccountVoucher
		matchedOnlyResource []AccountVoucher
		matchedOnlyCluster  []AccountVoucher
	)

	for _, voucher := range vouchers {
		matchedType := checkVoucherMatchType(voucher, clusterID, xpuModel)

		switch matchedType {
		case utils.VoucherMatchTypeBoth:
			matchedBoth = append(matchedBoth, voucher)
		case utils.VoucherMatchTypeXPU:
			matchedOnlyResource = append(matchedOnlyResource, voucher)
		case utils.VoucherMatchTypeCluster:
			matchedOnlyCluster = append(matchedOnlyCluster, voucher)
		default:
			continue
		}
	}

	sortedVouchers := []AccountVoucher{}
	sortedVouchers = append(sortedVouchers, matchedBoth...)
	sortedVouchers = append(sortedVouchers, matchedOnlyResource...)
	sortedVouchers = append(sortedVouchers, matchedOnlyCluster...)

	return sortedVouchers
}

func checkVoucherMatchType(voucher AccountVoucher, reqClusterID, reqXpuModel string) utils.VoucherMatchType {
	if len(voucher.Rules) < 1 {
		return utils.VoucherMatchTypeNone
	}

	voucherMatchResource := false
	voucherMatchCluster := false

	for _, rule := range voucher.Rules {
		singleRuleMatchCluster := false
		singleRuleMatchResource := false

		if len(rule.XPUModels) > 0 && slices.Contains(rule.XPUModels, reqXpuModel) {
			singleRuleMatchResource = true
		}

		if len(rule.ClusterIDs) > 0 && slices.Contains(rule.ClusterIDs, reqClusterID) {
			singleRuleMatchCluster = true
		}

		if len(rule.XPUModels) > 0 &&
			slices.Contains(rule.XPUModels, reqXpuModel) &&
			len(rule.ClusterIDs) < 1 {
			voucherMatchResource = true
		}

		if len(rule.ClusterIDs) > 0 &&
			slices.Contains(rule.ClusterIDs, reqClusterID) &&
			len(rule.XPUModels) < 1 {
			voucherMatchCluster = true
		}

		if singleRuleMatchCluster && singleRuleMatchResource {
			return utils.VoucherMatchTypeBoth
		}
	}

	if voucherMatchResource {
		return utils.VoucherMatchTypeXPU
	}

	if voucherMatchCluster {
		return utils.VoucherMatchTypeCluster
	}

	return utils.VoucherMatchTypeNone
}
