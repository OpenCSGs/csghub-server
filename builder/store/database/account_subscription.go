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
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccountSubscriptionStore interface {
	List(ctx context.Context, req *types.SubscriptionListReq) (*SubscriptionListResult, error)
	CreateOrUpdate(ctx context.Context, req *types.SubscriptionUpdateReq) (*AccountSubscription, error)
	Update(ctx context.Context, sub AccountSubscription) (*AccountSubscription, error)
	GetByID(ctx context.Context, id int64) (*AccountSubscription, error)
	StatusByUserUUID(ctx context.Context, userUUID string, skuType types.SKUType) (*AccountSubscription, error)
	ListRenews(ctx context.Context) ([]AccountSubscription, error)
	Renew(ctx context.Context, sub *AccountSubscription, eventUUID uuid.UUID) error
}

type SubscriptionListResult struct {
	Data            []AccountSubscription `json:"data"`
	Total           int                   `json:"total"`
	PaidTotalAmount float64               `json:"paid_total_amount"`
	PaidTotalCount  int                   `json:"paid_total_count"`
}

type accountSubscriptionStoreImpl struct {
	db *DB
}

func NewAccountSubscriptionStore() AccountSubscriptionStore {
	return &accountSubscriptionStoreImpl{
		db: defaultDB,
	}
}

func NewAccountSubscriptionWithDB(db *DB) AccountSubscriptionStore {
	return &accountSubscriptionStoreImpl{
		db: db,
	}
}

type AccountSubscription struct {
	ID              int64                    `bun:",pk,autoincrement" json:"id"`
	UserUUID        string                   `bun:",notnull" json:"user_uuid"`
	SkuType         types.SKUType            `bun:",notnull" json:"sku_type"`
	PriceID         int64                    `bun:",notnull" json:"price_id"`
	ResourceID      string                   `bun:",notnull" json:"resource_id"`
	Status          types.SubscriptionStatus `bun:",notnull" json:"status"`
	ActionUser      string                   `bun:",notnull" json:"action_user"`
	StartAt         time.Time                `bun:",notnull" json:"start_at"`
	EndAt           time.Time                `bun:",nullzero" json:"end_at"`
	LastBillID      int64                    `bun:",notnull,unique" json:"last_bill_id"`
	LastPeriodStart time.Time                `bun:",notnull" json:"last_period_start"`
	LastPeriodEnd   time.Time                `bun:",notnull" json:"last_period_end"`
	AmountPaidTotal float64                  `bun:",notnull" json:"amount_paid_total"`
	AmountPaidCount int64                    `bun:",notnull" json:"amount_paid_count"`
	NextPriceID     int64                    `bun:",nullzero" json:"next_price_id"`
	NextResourceID  string                   `bun:",nullzero" json:"next_resource_id"`
	times
}

func (a *accountSubscriptionStoreImpl) GetByID(ctx context.Context, id int64) (*AccountSubscription, error) {
	var sub AccountSubscription
	err := a.db.Operator.Core.NewSelect().Model(&sub).Where("id = ?", id).Scan(ctx, &sub)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return &sub, nil
}

func (a *accountSubscriptionStoreImpl) Update(ctx context.Context, sub AccountSubscription) (*AccountSubscription, error) {
	_, err := a.db.Operator.Core.NewUpdate().Model(&sub).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &sub, nil
}

func (a *accountSubscriptionStoreImpl) StatusByUserUUID(ctx context.Context, userUUID string, skuType types.SKUType) (*AccountSubscription, error) {
	var sub AccountSubscription
	err := a.db.Operator.Core.NewSelect().Model(&sub).
		Where("user_uuid = ?", userUUID).Where("sku_type = ?", skuType).
		Order("id desc").Limit(1).
		Scan(ctx, &sub)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &sub, nil
}

func (a *accountSubscriptionStoreImpl) CreateOrUpdate(ctx context.Context, req *types.SubscriptionUpdateReq) (*AccountSubscription, error) {
	respSub := &AccountSubscription{}
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		var postPrice AccountPrice
		var acctSub AccountSubscription

		if req.ResourceID != types.SubscriptionFree {
			err = tx.NewSelect().Model(&postPrice).
				Where("sku_type = ?", req.SkuType).
				Where("sku_kind = ?", types.SKUTimeSpan).
				Where("resource_id = ?", req.ResourceID).
				Where("sku_unit_type = ?", req.SkuUnitType).
				Order("created_at desc").Limit(1).Scan(ctx, &postPrice)
			if err != nil {
				return errorx.HandleDBError(err,
					errorx.Ctx().
						Set("query_price_post_resource_id", req.ResourceID).
						Set("unit_type", req.SkuUnitType))
			}
		}

		err = tx.NewSelect().Model(&acctSub).
			Where("user_uuid = ?", req.UserUUID).
			Where("sku_type = ?", req.SkuType).
			Scan(ctx, &acctSub)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errorx.HandleDBError(err, errorx.Ctx().Set("query_acct_user_sub", req.UserUUID))
		}

		if errors.Is(err, sql.ErrNoRows) {
			// no subscription found
			if req.ResourceID == types.SubscriptionFree {
				respSub.UserUUID = req.UserUUID
				respSub.SkuType = req.SkuType
				respSub.ResourceID = req.ResourceID
				respSub.Status = types.SubscriptionStatusCanceled
				slog.Warn("no subscription found and no action required for create free subscription", slog.Any("req", req))
				return nil
			}

			// build new subscription
			return a.buildNewSubscription(ctx, tx, req, respSub, &postPrice)
		}

		if req.ResourceID == types.SubscriptionFree {
			// close subscription
			err = a.closeSubscription(ctx, tx, &acctSub)
		} else if acctSub.Status != types.SubscriptionStatusActive && acctSub.LastPeriodEnd.Unix() < time.Now().Unix() {
			// refresh expired subscription
			err = a.refreshSubscription(ctx, tx, req, &acctSub, &postPrice)
		} else {
			// update in use subscription
			err = a.updateInUseSubscription(ctx, tx, req, &postPrice, &acctSub)
		}

		if err != nil {
			return err
		}

		copySubscriptionValues(respSub, &acctSub)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return respSub, nil
}

func (a *accountSubscriptionStoreImpl) buildNewSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionUpdateReq, newSub *AccountSubscription, postPrice *AccountPrice) error {
	return a.beginNewSubscription(ctx, tx, req, newSub, postPrice, true)
}

func (a *accountSubscriptionStoreImpl) refreshSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionUpdateReq, curSub *AccountSubscription, postPrice *AccountPrice) error {
	return a.beginNewSubscription(ctx, tx, req, curSub, postPrice, false)
}

func (a *accountSubscriptionStoreImpl) beginNewSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionUpdateReq, sub *AccountSubscription, postPrice *AccountPrice, newSubRecord bool) error {
	var (
		err            error
		res            sql.Result
		preLastBillID  int64
		postLastBillID int64
	)

	acctUser, err := CheckUserAccount(ctx, tx, req.UserUUID)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("query_acct_user", req.UserUUID).Set("newSub", newSubRecord))
	}

	if float64(postPrice.SkuPrice) > (acctUser.CashBalance + acctUser.Balance) {
		return errorx.HandleDBError(errorx.ErrInsufficientBalance,
			errorx.Ctx().Set("balance", acctUser.CashBalance+acctUser.Balance).
				Set("fee", postPrice.SkuPrice).Set("newSub", newSubRecord))
	}

	now := time.Now()
	amountValue := 0 - float64(postPrice.SkuPrice)

	err = a.generateAcctStatement(ctx, tx, req.EventUUID, acctUser.UserUUID, postPrice, now, amountValue, acctUser.UserUUID)
	if err != nil {
		return err
	}

	periodStart := now
	periodEnd, err := calculatePeriodEndTime(periodStart, postPrice, periodStart, 0)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("calc_period_end", periodStart).Set("newSub", newSubRecord))
	}

	sub.UserUUID = acctUser.UserUUID
	sub.SkuType = postPrice.SkuType
	sub.PriceID = postPrice.ID
	sub.ResourceID = postPrice.ResourceID
	sub.Status = types.SubscriptionStatusActive
	sub.ActionUser = acctUser.UserUUID
	sub.StartAt = periodStart
	sub.LastBillID = 0
	sub.LastPeriodStart = periodStart
	sub.LastPeriodEnd = periodEnd
	sub.AmountPaidTotal += float64(postPrice.SkuPrice)
	sub.AmountPaidCount += 1
	sub.NextPriceID = postPrice.ID
	sub.NextResourceID = postPrice.ResourceID

	if newSubRecord {
		res, err = tx.NewInsert().Model(sub).Exec(ctx, sub)
	} else {
		res, err = tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	}
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("gen_acct_sub", acctUser.UserUUID).Set("newSub", newSubRecord))
	}

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = sub.ID
	subBill.EventUUID = req.EventUUID.String()
	subBill.UserUUID = acctUser.UserUUID
	subBill.AmountPaid = float64(postPrice.SkuPrice)
	subBill.Status = types.BillingStatusPaid
	subBill.Reason = types.BillingReasonSubscriptionCreate
	subBill.PeriodStart = periodStart
	subBill.PeriodEnd = periodEnd
	subBill.PriceID = postPrice.ID
	subBill.ResourceID = postPrice.ResourceID
	subBill.SkuType = req.SkuType
	subBill.Discount = postPrice.Discount

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("gen_acct_sub_bill", acctUser.UserUUID).Set("newSub", newSubRecord))
	}

	sub.LastBillID = subBill.ID
	sub.LastPeriodStart = subBill.PeriodStart
	sub.LastPeriodEnd = subBill.PeriodEnd
	res, err = tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("mod_acct_sub", acctUser.UserUUID).Set("newSub", newSubRecord))
	}

	preLastBillID = 0
	postLastBillID = subBill.ID
	err = a.migrateSubUsageForUpgrade(ctx, tx, preLastBillID, postLastBillID, req, postPrice)
	if err != nil {
		return errorx.HandleDBError(err,
			errorx.Ctx().
				Set("upgrade_sub_usage", req.UserUUID).
				Set("pre_last_id", preLastBillID).
				Set("post_last_id", postLastBillID))
	}

	return nil
}

func (a *accountSubscriptionStoreImpl) updateInUseSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionUpdateReq, postPrice *AccountPrice, curSub *AccountSubscription) error {
	var (
		err            error
		acctUser       AccountUser
		prePrice       AccountPrice
		preLastBillID  int64
		postLastBillID int64
	)

	if (curSub.NextResourceID == postPrice.ResourceID || curSub.NextPriceID == postPrice.ID) &&
		curSub.Status == types.SubscriptionStatusActive {
		slog.Warn("no action required for update subscription",
			slog.Any("NextResourceID", curSub.NextResourceID),
			slog.Any("PostResourceID", postPrice.ResourceID),
			slog.Any("Status", curSub.Status))
		return nil
	}

	err = tx.NewSelect().Model(&prePrice).Where("id = ?", curSub.PriceID).Scan(ctx, &prePrice)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("query_pre_price", curSub.PriceID))
	}

	if prePrice.SkuUnitType != postPrice.SkuUnitType {
		return errorx.ErrInvalidUnitType
	}

	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", req.UserUUID).Scan(ctx, &acctUser)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("query_acct_user", req.UserUUID))
	}

	now := time.Now()
	monthGap := monthDiff(curSub.StartAt, curSub.LastPeriodStart)
	periodEnd, err := calculatePeriodEndTime(curSub.LastPeriodStart, postPrice, curSub.StartAt, monthGap)
	if err != nil {
		return errorx.HandleDBError(errorx.ErrWrongTimeRange, errorx.Ctx().Set("calc_period_end", curSub.StartAt))
	}

	feeGap := float64(0)
	feeGap, err = a.calculateFeeGap(prePrice, postPrice, now, curSub, periodEnd)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("calc_fee_with_end_time", periodEnd))
	}

	if feeGap > 0 && feeGap > (acctUser.CashBalance+acctUser.Balance) {
		return errorx.HandleDBError(errorx.ErrInsufficientBalance,
			errorx.Ctx().Set("balance", acctUser.CashBalance+acctUser.Balance).Set("fee", feeGap))
	}

	amountValue := 0 - feeGap

	err = a.generateAcctStatement(ctx, tx, req.EventUUID, req.UserUUID, postPrice, now, amountValue, acctUser.UserUUID)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("gen_acct_statement", req.UserUUID))
	}

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = curSub.ID
	subBill.EventUUID = req.EventUUID.String()
	subBill.UserUUID = acctUser.UserUUID
	subBill.AmountPaid = feeGap
	subBill.Status = types.BillingStatusPaid
	if feeGap > 0 || postPrice.SkuPrice >= prePrice.SkuPrice {
		subBill.Reason = types.BillingReasionSubscriptionUpgrade
	} else {
		subBill.Reason = types.BillingReasionSubscriptionDowngrade
	}
	subBill.PeriodStart = curSub.LastPeriodStart
	subBill.PeriodEnd = periodEnd
	subBill.PriceID = postPrice.ID
	subBill.ResourceID = postPrice.ResourceID
	subBill.SkuType = req.SkuType
	subBill.Discount = postPrice.Discount

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("new_sub_bill", req.UserUUID))
	}

	if feeGap > 0 {
		preLastBillID = curSub.LastBillID // save previous last bill id for upgrade
		postLastBillID = subBill.ID       // save new last bill id for upgrade

		curSub.PriceID = postPrice.ID
		curSub.ResourceID = postPrice.ResourceID
		curSub.LastBillID = subBill.ID
		curSub.LastPeriodStart = subBill.PeriodStart
		curSub.LastPeriodEnd = subBill.PeriodEnd
	}
	curSub.NextPriceID = postPrice.ID
	curSub.NextResourceID = postPrice.ResourceID
	curSub.AmountPaidTotal += feeGap
	curSub.AmountPaidCount += 1
	curSub.Status = types.SubscriptionStatusActive
	if curSub.Status != types.SubscriptionStatusActive {
		// reset subscription start time
		curSub.StartAt = now
	}

	resSub, err := tx.NewUpdate().Model(curSub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(resSub, err)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("mod_cur_sub", req.UserUUID))
	}

	// migrate usage record for upgrade subscription
	err = a.migrateSubUsageForUpgrade(ctx, tx, preLastBillID, postLastBillID, req, postPrice)
	if err != nil {
		return errorx.HandleDBError(err,
			errorx.Ctx().
				Set("upgrade_sub_usage", req.UserUUID).
				Set("pre_last_id", preLastBillID).
				Set("post_last_id", postLastBillID))
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) migrateSubUsageForUpgrade(ctx context.Context, tx bun.Tx,
	preBillID, postBillID int64, req *types.SubscriptionUpdateReq, postPrice *AccountPrice) error {

	if req.SkuType != types.SKUStarship {
		return nil
	}

	var (
		usageErr error
		preUsage AccountSubscriptionUsage
		preUsed  float64
	)

	usageErr = tx.NewSelect().Model(&preUsage).
		Where("bill_id = ?", preBillID).
		Where("user_uuid = ?", req.UserUUID).
		Where("sku_type = ?", req.SkuType).
		Where("value_type = ?", types.TokenNumberType).
		Scan(ctx, &preUsage)

	if usageErr != nil && !errors.Is(usageErr, sql.ErrNoRows) {
		return errorx.HandleDBError(usageErr, errorx.Ctx().Set("query_upgrade_last_preid", preBillID))
	}

	if usageErr != nil && errors.Is(usageErr, sql.ErrNoRows) {
		preUsed = 0
	} else {
		preUsed = preUsage.Used
	}

	postUsage := AccountSubscriptionUsage{
		UserUUID:     req.UserUUID,
		ResourceID:   "",
		ResourceName: "",
		CustomerID:   "",
		Used:         preUsed,
		Quota:        float64(postPrice.UseLimitPrice),
		BillID:       postBillID,
		SkuType:      req.SkuType,
		ValueType:    types.TokenNumberType,
	}
	res, err := tx.NewInsert().Model(&postUsage).Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err,
			errorx.Ctx().Set("insert_new_usage_user_uuid", req.UserUUID).
				Set("insert_new_usage_postid", postBillID).
				Set("insert_new_usage_preid", preBillID))
	}

	return nil
}

func (a *accountSubscriptionStoreImpl) closeSubscription(ctx context.Context, tx bun.Tx, curSub *AccountSubscription) error {
	if curSub.Status != types.SubscriptionStatusActive {
		slog.Warn("no action required for close subscription", slog.Any("status", curSub.Status),
			slog.Any("user_uuid", curSub.UserUUID), slog.Any("sku_type", curSub.SkuType))
		return nil
	}

	curSub.Status = types.SubscriptionStatusClosed
	curSub.EndAt = time.Now()

	_, err := tx.NewUpdate().Model(curSub).WherePK().Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("close_user_sub", curSub.UserUUID))
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) List(ctx context.Context, req *types.SubscriptionListReq) (*SubscriptionListResult, error) {
	type SumResult struct {
		PaidTotalAmount float64 `bun:"paid_total_amount"`
		PaidTotalCount  int     `bun:"paid_total_count"`
	}

	var subs []AccountSubscription
	var sumResult SumResult

	q := a.db.Operator.Core.NewSelect().Model(&subs).
		Where("start_at >= ?", req.StartTime).
		Where("start_at <= ?", req.EndTime)

	sumQuery := a.db.Operator.Core.NewSelect().Model((*AccountSubscription)(nil)).
		ColumnExpr("COALESCE(SUM(amount_paid_total), 0) as paid_total_amount").
		ColumnExpr("COALESCE(SUM(amount_paid_count), 0) as paid_total_count").
		Where("start_at >= ?", req.StartTime).
		Where("start_at <= ?", req.EndTime)

	if len(req.Status) > 0 {
		q = q.Where("status = ?", req.Status)
		sumQuery = sumQuery.Where("status = ?", req.Status)
	}

	if req.SkuType > 0 {
		q = q.Where("sku_type = ?", req.SkuType)
		sumQuery.Where("sku_type = ?", req.SkuType)
	}

	if len(req.QueryUserUUID) > 0 {
		q = q.Where("user_uuid = ?", req.QueryUserUUID)
		sumQuery = sumQuery.Where("user_uuid = ?", req.QueryUserUUID)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	err = sumQuery.Scan(ctx, &sumResult)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &subs)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	result := &SubscriptionListResult{
		Data:            subs,
		Total:           count,
		PaidTotalAmount: sumResult.PaidTotalAmount,
		PaidTotalCount:  sumResult.PaidTotalCount,
	}

	return result, nil
}

func (a *accountSubscriptionStoreImpl) ListRenews(ctx context.Context) ([]AccountSubscription, error) {
	var subs []AccountSubscription
	err := a.db.Operator.Core.NewSelect().Model(&subs).
		Where("status = ?", types.SubscriptionStatusActive).
		Where("last_period_end <= now()").
		Order("id").Scan(ctx, &subs)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return subs, nil
}

func (a *accountSubscriptionStoreImpl) Renew(ctx context.Context, sub *AccountSubscription, eventUUID uuid.UUID) error {
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		var price AccountPrice
		var acctUser AccountUser

		err = tx.NewSelect().Model(&price).Where("id = ?", sub.NextPriceID).Scan(ctx, &price)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("renew sub id %d to select price id %d error: %w", sub.ID, sub.NextPriceID, err)
		}

		if errors.Is(err, sql.ErrNoRows) {
			// cancel due to lost price
			return a.renewCancelSub(ctx, tx, sub, &price, types.BillingReasonLostPrice, eventUUID)
		}

		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", sub.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return fmt.Errorf("renew sub id %d to select account user uuid %s error: %w", sub.ID, sub.UserUUID, err)
		}

		if float64(price.SkuPrice) > (acctUser.CashBalance + acctUser.Balance) {
			// cancel due to insufficient balance
			return a.renewCancelSub(ctx, tx, sub, &price, types.BillingReasonBalanceInsufficient, eventUUID)
		}

		now := time.Now()
		amountValue := 0 - float64(price.SkuPrice)

		err = a.generateAcctStatement(ctx, tx, eventUUID, acctUser.UserUUID, &price,
			now, amountValue, string(types.BillingReasonSubscriptionCycle))
		if err != nil {
			return fmt.Errorf("renew sub id %d to deduct account fee error: %w", sub.ID, err)
		}

		err = a.renewBill(ctx, tx, sub, &price, eventUUID)
		if err != nil {
			return fmt.Errorf("renew sub id %d to insert bill error: %w", sub.ID, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("renew sub id %d with last period end %v in TX error: %w", sub.ID, sub.LastPeriodEnd, err)
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) renewCancelSub(ctx context.Context, tx bun.Tx,
	sub *AccountSubscription, price *AccountPrice,
	reason types.BillingReasion, eventUUID uuid.UUID) error {

	now := time.Now()

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = sub.ID
	subBill.EventUUID = eventUUID.String()
	subBill.UserUUID = sub.UserUUID
	if price != nil {
		subBill.AmountPaid = float64(price.SkuPrice)
		subBill.Discount = price.Discount
	}
	subBill.Status = types.BillingStatusFailed
	subBill.Reason = reason
	subBill.PeriodStart = now
	subBill.PeriodEnd = now
	subBill.PriceID = sub.NextPriceID
	if price != nil && len(price.ResourceID) > 0 {
		subBill.ResourceID = price.ResourceID
	} else {
		subBill.ResourceID = sub.NextResourceID
	}
	subBill.SkuType = sub.SkuType

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return fmt.Errorf("insert bill of cancel reason %s for sub id %d, error: %w", subBill.Reason, sub.ID, err)
	}

	sub.Status = types.SubscriptionStatusCanceled
	sub.EndAt = now
	sub.LastBillID = subBill.ID
	sub.LastPeriodStart = subBill.PeriodStart
	sub.LastPeriodEnd = subBill.PeriodEnd
	res, err := tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return fmt.Errorf("update sub id %d to status %s error: %w", sub.ID, sub.Status, err)
	}

	return nil
}

func (a *accountSubscriptionStoreImpl) renewBill(ctx context.Context, tx bun.Tx,
	sub *AccountSubscription, price *AccountPrice, eventUUID uuid.UUID) error {
	var (
		preLastBillID  int64
		postLastBillID int64
	)

	periodStart := sub.LastPeriodEnd
	monthGap := monthDiff(sub.StartAt, periodStart)
	periodEnd, err := calculatePeriodEndTime(periodStart, price, sub.StartAt, monthGap)
	if err != nil {
		return fmt.Errorf("calc period end for cycle sub id %d error: %w", sub.ID, err)
	}

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = sub.ID
	subBill.EventUUID = eventUUID.String()
	subBill.UserUUID = sub.UserUUID
	subBill.AmountPaid = float64(price.SkuPrice)
	subBill.Status = types.BillingStatusPaid
	subBill.Reason = types.BillingReasonSubscriptionCycle
	subBill.PeriodStart = periodStart
	subBill.PeriodEnd = periodEnd
	subBill.PriceID = price.ID
	subBill.ResourceID = price.ResourceID
	subBill.SkuType = sub.SkuType
	subBill.Discount = price.Discount

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return fmt.Errorf("insert cycle bill %s for sub id %d error: %w", subBill.Reason, sub.ID, err)
	}

	sub.PriceID = price.ID
	sub.ResourceID = price.ResourceID
	sub.LastBillID = subBill.ID
	postLastBillID = subBill.ID
	sub.LastPeriodStart = subBill.PeriodStart
	sub.LastPeriodEnd = subBill.PeriodEnd
	sub.AmountPaidTotal += float64(price.SkuPrice)
	sub.AmountPaidCount += 1
	res, err := tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return fmt.Errorf("update cycle sub id %d to last bill id %d error: %w", sub.ID, sub.LastBillID, err)
	}

	req := &types.SubscriptionUpdateReq{
		UserUUID: sub.UserUUID,
		SkuType:  sub.SkuType,
	}
	err = a.migrateSubUsageForUpgrade(ctx, tx, preLastBillID, postLastBillID, req, price)
	if err != nil {
		return errorx.HandleDBError(err,
			errorx.Ctx().
				Set("renew_sub_usage", req.UserUUID).
				Set("pre_last_id", preLastBillID).
				Set("post_last_id", postLastBillID))
	}

	return nil
}

func (a *accountSubscriptionStoreImpl) generateAcctStatement(ctx context.Context, tx bun.Tx,
	eventUUID uuid.UUID, userUUID string, price *AccountPrice,
	now time.Time, amountValue float64, opUID string) error {

	statement := AccountStatement{
		EventUUID:        eventUUID,
		UserUUID:         userUUID,
		Value:            amountValue,
		Scene:            types.ScenePaySubscription,
		OpUID:            opUID,
		CreatedAt:        now,
		CustomerID:       price.ResourceID,
		EventDate:        now,
		Price:            float64(price.SkuPrice),
		ValueType:        types.TimeDurationMinType,
		ResourceID:       price.ResourceID,
		SkuID:            price.ID,
		RecordedAt:       now,
		SkuUnit:          price.SkuUnit,
		SkuUnitType:      price.SkuUnitType,
		SkuPriceCurrency: price.SkuPriceCurrency,
		IsCancel:         false,
		EventValue:       amountValue,
		Discount:         price.Discount,
	}
	err := DeductAccountFee(ctx, tx, statement)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("gen_acct_statement", userUUID))
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) calculateFeeGap(prePrice AccountPrice, postPrice *AccountPrice,
	now time.Time, sub *AccountSubscription, postEndTime time.Time) (float64, error) {

	feeGap := float64(0)
	priceGap := float64(postPrice.SkuPrice - prePrice.SkuPrice)
	if priceGap > 0 {
		// increasing price
		preStartStamp := sub.LastPeriodStart.Unix()
		preEndStamp := sub.LastPeriodEnd.Unix()
		nowStamp := now.Unix()
		if nowStamp < preStartStamp || nowStamp > preEndStamp {
			return 0, errorx.ErrWrongTimeRange
		}
		// unconsumed fee of current valid period
		unConsumedFee := float64(prePrice.SkuPrice) * (float64(preEndStamp-nowStamp) / float64(preEndStamp-preStartStamp))

		postEndStamp := postEndTime.Unix()
		// will pay fee of new period
		willPayFee := float64(postPrice.SkuPrice) * (float64(postEndStamp-nowStamp) / float64(postEndStamp-preStartStamp))
		feeGap = willPayFee - unConsumedFee
	}

	return feeGap, nil
}

func calculatePeriodEndTime(startTime time.Time, price *AccountPrice, createTime time.Time, monthsGap int) (time.Time, error) {
	switch types.SkuUnitType(price.SkuUnitType) {
	case types.UnitDay:
		return startTime.AddDate(0, 0, int(price.SkuUnit)), nil
	case types.UnitWeek:
		return startTime.AddDate(0, 0, 7*int(price.SkuUnit)), nil
	case types.UnitMonth:
		time1 := calculateEndTimeByMonth(startTime, int(price.SkuUnit))
		time2 := calculateEndTimeByMonth(createTime, monthsGap+int(price.SkuUnit))
		if time2.After(time1) {
			return time2, nil
		} else {
			return time1, nil
		}
	case types.UnitYear:
		return startTime.AddDate(int(price.SkuUnit), 0, 0), nil
	default:
		return startTime, errorx.ErrInvalidUnitType
	}
}

func calculateEndTimeByMonth(startTime time.Time, unit int) time.Time {
	var returnTime time.Time
	firstDayOfMonth := time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
	firstDayOfNextMonth := firstDayOfMonth.AddDate(0, unit, 0)
	lastDayofNextMonth := firstDayOfNextMonth.AddDate(0, 1, -1)
	endTime := startTime.AddDate(0, unit, 0)
	if endTime.After(lastDayofNextMonth) {
		returnTime = lastDayofNextMonth
	} else {
		returnTime = endTime
	}
	returnTime = time.Date(returnTime.Year(), returnTime.Month(), returnTime.Day(),
		startTime.Hour(), startTime.Minute(), startTime.Second(), 0, returnTime.Location())
	return returnTime
}

func monthDiff(start, end time.Time) int {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	totalMonths := years*12 + months
	return totalMonths
}

func copySubscriptionValues(dst, src *AccountSubscription) {
	dst.ID = src.ID
	dst.UserUUID = src.UserUUID
	dst.SkuType = src.SkuType
	dst.PriceID = src.PriceID
	dst.ResourceID = src.ResourceID
	dst.Status = src.Status
	dst.ActionUser = src.ActionUser
	dst.StartAt = src.StartAt
	dst.EndAt = src.EndAt
	dst.LastBillID = src.LastBillID
	dst.LastPeriodStart = src.LastPeriodStart
	dst.LastPeriodEnd = src.LastPeriodEnd
	dst.AmountPaidTotal = src.AmountPaidTotal
	dst.AmountPaidCount = src.AmountPaidCount
	dst.NextPriceID = src.NextPriceID
	dst.NextResourceID = src.NextResourceID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}
