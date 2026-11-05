package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountStatementStoreImpl struct {
	db *DB
}

type AccountStatementStore interface {
	Create(ctx context.Context, input AccountStatement, extra types.AcctStatementExtra) error
	CreateTradeStatements(ctx context.Context, buyerStmt, sellerStmt AccountStatement) error
	ListByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (AccountStatementRes, error)
	GetByEventID(ctx context.Context, eventID uuid.UUID) (AccountStatement, error)
	GetDeductionSummaryByUserSceneCustomer(ctx context.Context, input AccountStatement) (*types.DeductionSummary, error)
	ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (AccountStatementRes, error)
	ListStatementByUserAndSku(ctx context.Context, req types.ActStatementsReq) ([]UserSkuStatement, int, error)
	ListPortalRecharges(ctx context.Context, req types.AcctRechargeListReq) ([]AccountStatement, int, float64, error)
	HasUserPurchasedDataset(ctx context.Context, userUUID string, datasetID int64) (bool, error)
	// MinCreatedAt returns the earliest created_at in account_statements, used as
	// the backfill origin when no checkpoint exists yet. Returns zero time if the
	// table is empty.
	MinCreatedAt(ctx context.Context) (time.Time, error)
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
	ID                int64                 `bun:",pk,autoincrement" json:"id"`
	EventUUID         uuid.UUID             `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID          string                `bun:",notnull" json:"user_uuid"`
	Value             float64               `bun:",notnull" json:"value"`
	Scene             types.SceneType       `bun:",notnull" json:"scene"`
	OpUID             string                `bun:",nullzero" json:"op_uid"`
	CreatedAt         time.Time             `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	CustomerID        string                `json:"customer_id"`
	EventDate         time.Time             `bun:"type:date" json:"event_date"`
	Price             float64               `json:"price"`
	PriceUnit         string                `json:"price_unit"`
	Consumption       float64               `json:"consumption"`
	ValueType         types.ChargeValueType `json:"value_type"`
	ResourceID        string                `json:"resource_id"`
	ResourceName      string                `json:"resource_name"`
	SkuID             int64                 `json:"sku_id"`
	RecordedAt        time.Time             `json:"recorded_at"`
	SkuUnit           int64                 `json:"sku_unit"`
	SkuUnitType       types.SkuUnitType     `json:"sku_unit_type"`
	SkuPriceCurrency  string                `json:"sku_price_currency"`
	BalanceType       string                `json:"balance_type"`
	BalanceValue      float64               `json:"balance_value"`
	IsCancel          bool                  `json:"is_cancel"`
	EventValue        float64               `json:"event_value"`
	Present           *AccountPresent       `bun:"rel:has-one,join:event_uuid=event_uuid"`
	Quota             float64               `bun:",nullzero" json:"quota"`
	SubBillID         int64                 `bun:",nullzero" json:"sub_bill_id"`
	Discount          float64               `json:"discount"`
	RegularValue      float64               `json:"regular_value"`
	PromptToken       float64               `json:"prompt_token"`
	PromptCachedToken float64               `json:"prompt_cached_token"`
	CompletionToken   float64               `json:"completion_token"`
	ReasoningToken    float64               `json:"reasoning_token"`
	APIKey            string                `bun:",notnull,default:''" json:"api_key"`
	TokenID           int64                 `bun:",notnull,default:0" json:"token_id"`
	Purpose           types.RechargePurpose `bun:",nullzero" json:"purpose"`
	PurposeDesc       string                `bun:",nullzero" json:"purpose_desc"`
	DataType          string                `bun:",notnull,default:''" json:"data_type"`
	Resolution        string                `bun:",notnull,default:''" json:"resolution"`
	Duration          float64               `bun:",notnull,default:0" json:"duration"`
	VoucherNo         string                `bun:",nullzero" json:"voucher_no"`
	// Cost snapshot for aigateway token usage (scenes 15/16), frozen at charge
	// time from SKUUpstreamCost prices; cents, >= 0. Sku ids are audit-only.
	CostAmount          float64 `bun:",notnull,default:0" json:"cost_amount"`
	CostPromptSkuID     *int64  `bun:",nullzero" json:"cost_prompt_sku_id"`
	CostCompletionSkuID *int64  `bun:",nullzero" json:"cost_completion_sku_id"`
	Provider            string  `bun:",notnull,default:''" json:"provider"`
	UpstreamID          int64   `bun:",notnull,default:0" json:"upstream_id"`
	// token plan
	BalanceValuePrev  float64 `bun:",nullzero" json:"balance_value_prev"`
	TokenPlanPeriodID int64   `bun:",nullzero" json:"token_plan_period_id"`
	RecordedAtUnix    int64   `bun:",nullzero" json:"recorded_at_unix"`
}

type AccountStatementRes struct {
	Data []AccountStatement `json:"data"`
	types.AcctSummary
}

type ChangedValue struct {
	Cash    float64
	Voucher float64
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

func (as *accountStatementStoreImpl) Create(ctx context.Context, input AccountStatement, extra types.AcctStatementExtra) error {
	if input.Scene == types.ScenePortalCharge ||
		input.Scene == types.SceneCashCharge ||
		input.Scene == types.SceneDatasetSaleIncome {
		return as.chargeFeeStatement(ctx, input)
	} else {
		return as.deductFeeStatement(ctx, input, extra)
	}
}

// updateFeeBill writes the bill row of one real money movement (cash or
// voucher). Money rows always bill on period_id 0, so the default bills view
// (period_id = 0) shows every real spend — including the uncovered share of a
// partially covered token plan call and pure pay-as-you-go charges under a
// resolved period (cases 3/8). The plan-absorbed share is the only bill row
// that carries a token plan period; it is written separately through
// updateFeeBillRow.
//
// It intentionally does not touch the API key quotas: one statement can be
// split into several money rows (one per voucher plus the cash row) that all
// carry the same usage detail, so recording usage here would count it once per
// row. Usage is recorded once per statement by recordAPIKeyUsage instead.
func updateFeeBill(ctx context.Context, tx bun.Tx, input AccountStatement, billValues BillValues) error {
	// input is passed by value: zeroing the period here never leaks to callers
	input.TokenPlanPeriodID = 0
	return updateFeeBillRow(ctx, tx, input, billValues)
}

// recordAPIKeyUsage applies one statement's usage to its API key quotas exactly
// once. The caller passes the statement before its fee is split into money
// rows, so Value stays the charged fee and Consumption stays the full call
// detail; the per-voucher and cash bill rows all carry that same detail and
// must not each count it.
func recordAPIKeyUsage(ctx context.Context, tx bun.Tx, input AccountStatement) {
	if input.TokenID <= 0 {
		return
	}
	if err := UpdateAPIKeyUsage(ctx, tx, input); err != nil {
		// The bill itself must not fail because the quota side effect failed,
		// matching the pre-existing behaviour.
		slog.WarnContext(ctx, "failed to update api key usage", slog.Any("err", err), slog.Int64("token_id", input.TokenID))
	}
}

// updateFeeBillRow upserts one account_bills row for a fee movement without
// the API key usage side effect; a zero TotalValue writes no row (zero-value
// fee semantics). The row bills on input.TokenPlanPeriodID as-is: only the
// token plan split uses this to write a period-attributed plan row.
func updateFeeBillRow(ctx context.Context, tx bun.Tx, input AccountStatement, billValues BillValues) error {
	if !utils.IsNeedCalculateBill(input.Scene) {
		return nil
	}
	billRes, err := GetConsumeResource(ctx, tx, input)
	if err != nil {
		return fmt.Errorf("get consume resource for %s user %s, error:%w", input.EventUUID, input.UserUUID, err)
	}
	if billValues.TotalValue != 0 {
		// calculate bill
		bill := AccountBill{
			BillDate:          input.EventDate,
			UserUUID:          input.UserUUID,
			Scene:             input.Scene,
			CustomerID:        input.CustomerID,
			Value:             billValues.TotalValue,
			Consumption:       billValues.Consumption,
			PromptToken:       input.PromptToken,
			PromptCachedToken: input.PromptCachedToken,
			CompletionToken:   input.CompletionToken,
			ReasoningToken:    input.ReasoningToken,
			Count:             1,
			TokenID:           input.TokenID,
			DataType:          input.DataType,
			Resolution:        input.Resolution,
			Duration:          input.Duration,
			VoucherNo:         input.VoucherNo,
			VoucherValue:      billValues.VoucherValue,
			CashValue:         billValues.CashValue,
			ClusterID:         billRes.ClusterID,
			HardwareType:      billRes.HardwareType,
			PeriodID:          input.TokenPlanPeriodID,
		}
		if input.Scene == types.SceneMultiModalServerless {
			bill.UnitType = input.SkuUnitType
		}

		// depend on unique index for update
		_, err := tx.NewInsert().Model(&bill).
			On("CONFLICT (bill_date, user_uuid, scene, customer_id, token_id, data_type, resolution, voucher_no, unit_type, cluster_id, hardware_type, period_id) DO UPDATE").
			Set("value = account_bill.value + ?", billValues.TotalValue).
			Set("consumption = account_bill.consumption + ?", billValues.Consumption).
			Set("prompt_token = account_bill.prompt_token + ?", input.PromptToken).
			Set("prompt_cached_token = account_bill.prompt_cached_token + ?", input.PromptCachedToken).
			Set("completion_token = account_bill.completion_token + ?", input.CompletionToken).
			Set("reasoning_token = account_bill.reasoning_token + ?", input.ReasoningToken).
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
	return nil
}

func updateSubscription(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	isCSGUsage := utils.IsNeedCheckUserSubscription(input.Scene)
	if isCSGUsage {
		reqSkuType := utils.GetSKUTypeByScene(types.SceneType(input.Scene))
		err := UpdateSubscriptionUsage(ctx, tx, input, reqSkuType)
		if err != nil {
			return fmt.Errorf("update subscription usage for %d, error: %w", reqSkuType, err)
		}
	}
	return nil
}

func GetConsumeResource(ctx context.Context, tx bun.Tx, input AccountStatement) (BillResource, error) {
	var err error
	var res SpaceResource
	billResource := BillResource{}

	if !utils.IsNeedExtractHardware(input.Scene) {
		// skip extract hardware type for scene that not need it
		return billResource, nil
	}

	res.ID, err = strconv.ParseInt(input.ResourceID, 10, 64)
	if err != nil {
		return billResource, fmt.Errorf("parse resource id %s to int64 for bill, error:%w", input.ResourceID, err)
	}
	_, err = tx.NewSelect().Model(&res).WherePK().Exec(ctx, &res)
	if err != nil {
		return billResource, fmt.Errorf("query space resource id %d for bill, error:%w", res.ID, err)
	}
	hwType, err := types.GetHardwareType(res.Resources)
	if err != nil {
		return billResource, fmt.Errorf("extract hardware type from resource id %d for bill, error:%w", res.ID, err)
	}
	billResource.ClusterID = res.ClusterID
	billResource.HardwareType = hwType
	return billResource, nil
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
	if req.VoucherNo != "" {
		q = q.Where("voucher_no = ?", req.VoucherNo)
	}
	if req.PeriodID > 0 {
		q = q.Where("token_plan_period_id = ?", req.PeriodID)
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

func (as *accountStatementStoreImpl) GetDeductionSummaryByUserSceneCustomer(ctx context.Context, input AccountStatement) (*types.DeductionSummary, error) {
	var summary types.DeductionSummary
	err := as.db.Operator.Core.NewSelect().Model((*AccountStatement)(nil)).
		ColumnExpr("user_uuid, scene, customer_id, COALESCE(SUM(value), 0) as total_value, MIN(event_uuid::text)::uuid as event_uuid").
		Where("user_uuid = ?", input.UserUUID).
		Where("scene = ?", input.Scene).
		Where("customer_id = ?", input.CustomerID).
		Group("user_uuid", "scene", "customer_id").
		Scan(ctx, &summary)
	if err != nil {
		return nil, errorx.HandleDBError(err, map[string]interface{}{
			"user_uuid":   input.UserUUID,
			"scene":       input.Scene,
			"customer_id": input.CustomerID,
		})
	}
	return &summary, nil
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

func (as *accountStatementStoreImpl) MinCreatedAt(ctx context.Context) (time.Time, error) {
	// MIN(created_at) returns NULL on an empty table; scan into sql.NullTime so
	// the empty-table case returns a zero time (and the caller can skip) rather
	// than a NULL-scan error.
	var nullTime sql.NullTime
	err := as.db.Operator.Core.NewSelect().
		Model((*AccountStatement)(nil)).
		ColumnExpr("MIN(created_at)").
		Scan(ctx, &nullTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("get min created_at from account_statements, error: %w", err)
	}
	if !nullTime.Valid {
		return time.Time{}, nil
	}
	return nullTime.Time, nil
}
