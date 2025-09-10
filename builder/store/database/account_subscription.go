package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccountSubscriptionStore interface {
	List(ctx context.Context, req *types.SubscriptionListReq) (*SubscriptionListResult, error)
	Create(ctx context.Context, req *types.SubscriptionCreateReq) (*AccountSubscription, error)
	Update(ctx context.Context, sub AccountSubscription) (*AccountSubscription, error)
	GetByID(ctx context.Context, id int64) (*AccountSubscription, error)
	StatusByUserUUID(ctx context.Context, userUUID string, skuType types.SKUType) (*AccountSubscription, error)
	UpdateResource(ctx context.Context, req *types.SubscriptionUpdateReq, sub *AccountSubscription) (*AccountSubscription, string, error)
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

func (a *accountSubscriptionStoreImpl) Create(ctx context.Context, req *types.SubscriptionCreateReq) (*AccountSubscription, error) {
	respSub := &AccountSubscription{}
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		var postPrice AccountPrice
		var acctSub AccountSubscription

		err = tx.NewSelect().Model(&acctSub).
			Where("status = ?", types.SubscriptionStatusActive).
			Where("user_uuid = ?", req.UserUUID).
			Where("sku_type = ?", req.SkuType).
			Scan(ctx, &acctSub)
		if err == nil {
			return errorx.ErrSubscriptionExist
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return errorx.HandleDBError(err, nil)
		}

		err = tx.NewSelect().Model(&postPrice).
			Where("sku_type = ?", req.SkuType).
			Where("sku_kind = ?", types.SKUTimeSpan).
			Where("resource_id = ?", req.ResourceID).
			Where("sku_unit_type = ?", req.SkuUnitType).
			Order("created_at desc").Limit(1).Scan(ctx, &postPrice)

		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("resource_id", req.ResourceID).Set("unit_type", req.SkuUnitType))
		}

		err = tx.NewSelect().Model(&acctSub).
			Where("user_uuid = ?", req.UserUUID).
			Where("sku_type = ?", req.SkuType).
			Order("id desc").Limit(1).
			Scan(ctx, &acctSub)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return errorx.HandleDBError(err, nil)
		}

		if errors.Is(err, sql.ErrNoRows) || acctSub.LastPeriodEnd.Before(time.Now()) {
			// build new
			return a.buildNewSubscription(ctx, tx, req, respSub, postPrice)
		}

		// reuse last available subscription
		err = a.reuseLastSubscription(ctx, tx, req, postPrice, &acctSub)
		if err != nil {
			return err
		}

		respSub = &acctSub
		return nil
	})

	if err != nil {
		return nil, err
	}
	return respSub, nil
}

func (a *accountSubscriptionStoreImpl) reuseLastSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionCreateReq, postPrice AccountPrice, lastSub *AccountSubscription) error {
	var err error
	var acctUser AccountUser
	var prePrice AccountPrice

	err = tx.NewSelect().Model(&prePrice).Where("id = ?", lastSub.PriceID).Scan(ctx, &prePrice)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("price", lastSub.PriceID))
	}

	if prePrice.SkuUnit != postPrice.SkuUnit || prePrice.SkuUnitType != postPrice.SkuUnitType {
		return errorx.ErrInvalidUnitType
	}

	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", req.UserUUID).Scan(ctx, &acctUser)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("user_uuid", req.UserUUID))
	}

	now := time.Now()
	feeGap := float64(0)

	feeGap, err = a.calculateFeeGap(prePrice, postPrice, now, lastSub)
	if err != nil {
		return err
	}

	if feeGap > 0 && feeGap > (acctUser.CashBalance+acctUser.Balance) {
		return errorx.ErrInsufficientBalance
	}

	amountValue := 0 - feeGap

	err = a.generateStatement(ctx, tx, req.EventUUID, req.UserUUID, &postPrice, now, amountValue, acctUser.UserUUID)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = lastSub.ID
	subBill.EventUUID = req.EventUUID.String()
	subBill.UserUUID = acctUser.UserUUID
	subBill.AmountPaid = feeGap
	subBill.Status = types.BillingStatusPaid
	if feeGap > 0 || postPrice.SkuPrice >= prePrice.SkuPrice {
		subBill.Reason = types.BillingReasionSubscriptionUpgrade
	} else {
		subBill.Reason = types.BillingReasionSubscriptionDowngrade
	}
	subBill.PeriodStart = lastSub.LastPeriodStart
	subBill.PeriodEnd = lastSub.LastPeriodEnd
	subBill.PriceID = postPrice.ID
	subBill.ResourceID = postPrice.ResourceID

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	if feeGap > 0 {
		lastSub.PriceID = postPrice.ID
		lastSub.ResourceID = postPrice.ResourceID
		lastSub.LastBillID = subBill.ID
		lastSub.LastPeriodStart = subBill.PeriodStart
		lastSub.LastPeriodEnd = subBill.PeriodEnd
	}
	lastSub.NextPriceID = postPrice.ID
	lastSub.NextResourceID = postPrice.ResourceID
	lastSub.AmountPaidTotal += feeGap
	lastSub.AmountPaidCount += 1
	lastSub.Status = types.SubscriptionStatusActive

	_, err = tx.NewUpdate().Model(lastSub).WherePK().Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	return nil
}

func (a *accountSubscriptionStoreImpl) buildNewSubscription(ctx context.Context, tx bun.Tx,
	req *types.SubscriptionCreateReq, sub *AccountSubscription, postPrice AccountPrice) error {
	var err error
	var acctUser AccountUser

	err = CheckUserAccount(ctx, tx, req.UserUUID)
	if err != nil {
		return err
	}

	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", req.UserUUID).Scan(ctx, &acctUser)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	if float64(postPrice.SkuPrice) > (acctUser.CashBalance + acctUser.Balance) {
		return errorx.ErrInsufficientBalance
	}

	now := time.Now()
	amountValue := 0 - float64(postPrice.SkuPrice)

	err = a.generateStatement(ctx, tx, req.EventUUID, acctUser.UserUUID, &postPrice, now, amountValue, acctUser.UserUUID)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	err = a.createNewSubscriptionAndBill(ctx, tx, sub, acctUser, postPrice, req.EventUUID)
	if err != nil {
		return err
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) createNewSubscriptionAndBill(ctx context.Context, tx bun.Tx,
	sub *AccountSubscription, acctUser AccountUser, price AccountPrice, eventUUID uuid.UUID) error {
	periodStart := time.Now()
	periodEnd, err := calculatePeriodEndTime(periodStart, price, periodStart, 0)
	if err != nil {
		return err
	}

	sub.UserUUID = acctUser.UserUUID
	sub.SkuType = price.SkuType
	sub.PriceID = price.ID
	sub.ResourceID = price.ResourceID
	sub.Status = types.SubscriptionStatusActive
	sub.ActionUser = acctUser.UserUUID
	sub.StartAt = time.Now()
	sub.LastBillID = 0
	sub.LastPeriodStart = periodStart
	sub.LastPeriodEnd = periodEnd
	sub.AmountPaidTotal = float64(price.SkuPrice)
	sub.AmountPaidCount = 1
	sub.NextPriceID = price.ID
	sub.NextResourceID = price.ResourceID

	res, err := tx.NewInsert().Model(sub).Exec(ctx, sub)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = sub.ID
	subBill.EventUUID = eventUUID.String()
	subBill.UserUUID = acctUser.UserUUID
	subBill.AmountPaid = float64(price.SkuPrice)
	subBill.Status = types.BillingStatusPaid
	subBill.Reason = types.BillingReasonSubscriptionCreate
	subBill.PeriodStart = periodStart
	subBill.PeriodEnd = periodEnd
	subBill.PriceID = price.ID
	subBill.ResourceID = price.ResourceID

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	sub.LastBillID = subBill.ID
	sub.LastPeriodStart = subBill.PeriodStart
	sub.LastPeriodEnd = subBill.PeriodEnd
	res, err = tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	return nil
}

func calculatePeriodEndTime(startTime time.Time, price AccountPrice, createTime time.Time, months int) (time.Time, error) {
	switch types.SkuUnitType(price.SkuUnitType) {
	case types.UnitDay:
		return startTime.AddDate(0, 0, int(price.SkuUnit)), nil
	case types.UnitWeek:
		return startTime.AddDate(0, 0, 7*int(price.SkuUnit)), nil
	case types.UnitMonth:
		time1 := calculateEndTimeByMonth(startTime, int(price.SkuUnit))
		time2 := calculateEndTimeByMonth(createTime, months+int(price.SkuUnit))
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
	if req.SkuType >= 0 {
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

func (a *accountSubscriptionStoreImpl) UpdateResource(ctx context.Context, req *types.SubscriptionUpdateReq, sub *AccountSubscription) (*AccountSubscription, string, error) {
	reasion := ""
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		var prePrice AccountPrice
		var postPrice AccountPrice
		var acctUser AccountUser

		err = tx.NewSelect().Model(&prePrice).Where("id = ?", sub.PriceID).Scan(ctx, &prePrice)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("price", sub.PriceID))
		}

		err = tx.NewSelect().Model(&postPrice).
			Where("sku_type = ?", req.SkuType).
			Where("sku_kind = ?", types.SKUTimeSpan).
			Where("resource_id = ?", req.ResourceID).
			Where("sku_unit_type = ?", req.SkuUnitType).
			Order("created_at desc").Limit(1).Scan(ctx, &postPrice)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("resource_id", req.ResourceID).Set("unit_type", req.SkuUnitType))
		}

		if prePrice.SkuUnit != postPrice.SkuUnit || prePrice.SkuUnitType != postPrice.SkuUnitType {
			return errorx.ErrInvalidUnitType
		}

		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", req.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("user_uuid", req.UserUUID))
		}

		now := time.Now()
		feeGap := float64(0)

		feeGap, err = a.calculateFeeGap(prePrice, postPrice, now, sub)
		if err != nil {
			return err
		}

		if feeGap > 0 && feeGap > (acctUser.CashBalance+acctUser.Balance) {
			return errorx.ErrInsufficientBalance
		}

		amountValue := 0 - feeGap

		err = a.generateStatement(ctx, tx, req.EventUUID, req.UserUUID, &postPrice, now, amountValue, acctUser.UserUUID)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}

		subBill := &AccountSubscriptionBill{}
		subBill.SubID = sub.ID
		subBill.EventUUID = req.EventUUID.String()
		subBill.UserUUID = acctUser.UserUUID
		subBill.AmountPaid = feeGap
		subBill.Status = types.BillingStatusPaid
		if feeGap > 0 || postPrice.SkuPrice >= prePrice.SkuPrice {
			subBill.Reason = types.BillingReasionSubscriptionUpgrade
		} else {
			subBill.Reason = types.BillingReasionSubscriptionDowngrade
		}
		subBill.PeriodStart = sub.LastPeriodStart
		subBill.PeriodEnd = sub.LastPeriodEnd
		subBill.PriceID = postPrice.ID
		subBill.ResourceID = postPrice.ResourceID

		resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
		err = assertAffectedOneRow(resBill, err)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}

		if feeGap > 0 {
			sub.PriceID = postPrice.ID
			sub.ResourceID = postPrice.ResourceID
			sub.LastBillID = subBill.ID
			sub.LastPeriodStart = subBill.PeriodStart
			sub.LastPeriodEnd = subBill.PeriodEnd
		}
		sub.NextPriceID = postPrice.ID
		sub.NextResourceID = postPrice.ResourceID
		sub.AmountPaidTotal += feeGap
		sub.AmountPaidCount += 1

		_, err = tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}
		reasion = string(subBill.Reason)
		return nil
	})

	if err != nil {
		return nil, reasion, err
	}

	return sub, reasion, nil
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
			return a.cancelSub(ctx, tx, sub, &price, types.BillingReasonLostPrice, eventUUID)
		}

		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", sub.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return fmt.Errorf("renew sub id %d to select account user uuid %s error: %w", sub.ID, sub.UserUUID, err)
		}

		if float64(price.SkuPrice) > (acctUser.CashBalance + acctUser.Balance) {
			// cancel due to insufficient balance
			return a.cancelSub(ctx, tx, sub, &price, types.BillingReasonBalanceInsufficient, eventUUID)
		}

		now := time.Now()
		amountValue := 0 - float64(price.SkuPrice)

		err = a.generateStatement(ctx, tx, eventUUID, acctUser.UserUUID, &price,
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

func (a *accountSubscriptionStoreImpl) cancelSub(ctx context.Context, tx bun.Tx,
	sub *AccountSubscription, price *AccountPrice,
	reason types.BillingReasion, eventUUID uuid.UUID) error {

	now := time.Now()

	subBill := &AccountSubscriptionBill{}
	subBill.SubID = sub.ID
	subBill.EventUUID = eventUUID.String()
	subBill.UserUUID = sub.UserUUID
	if price != nil {
		subBill.AmountPaid = float64(price.SkuPrice)
	}
	subBill.Status = types.BillingStatusFailed
	subBill.Reason = reason
	subBill.PeriodStart = now
	subBill.PeriodEnd = now
	subBill.PriceID = sub.NextPriceID
	if price != nil {
		subBill.ResourceID = price.ResourceID
	}

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
	periodStart := sub.LastPeriodEnd
	monthGap := monthDiff(sub.StartAt, periodStart)
	periodEnd, err := calculatePeriodEndTime(periodStart, *price, sub.StartAt, monthGap)
	if err != nil {
		return err
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

	resBill, err := tx.NewInsert().Model(subBill).Exec(ctx, subBill)
	err = assertAffectedOneRow(resBill, err)
	if err != nil {
		return fmt.Errorf("insert bill %s for sub id %d error: %w", subBill.Reason, sub.ID, err)
	}

	sub.PriceID = price.ID
	sub.ResourceID = price.ResourceID
	sub.LastBillID = subBill.ID
	sub.LastPeriodStart = subBill.PeriodStart
	sub.LastPeriodEnd = subBill.PeriodEnd
	sub.AmountPaidTotal += float64(price.SkuPrice)
	sub.AmountPaidCount += 1
	res, err := tx.NewUpdate().Model(sub).WherePK().Exec(ctx)
	err = assertAffectedOneRow(res, err)
	if err != nil {
		return fmt.Errorf("update sub id %d to last bill id %d error: %w", sub.ID, sub.LastBillID, err)
	}

	return nil
}

func monthDiff(start, end time.Time) int {
	years := end.Year() - start.Year()
	months := int(end.Month()) - int(start.Month())
	totalMonths := years*12 + months
	return totalMonths
}

func (a *accountSubscriptionStoreImpl) generateStatement(ctx context.Context, tx bun.Tx,
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
		ValueType:        types.CountNumberType,
		ResourceID:       price.ResourceID,
		SkuID:            price.ID,
		RecordedAt:       now,
		SkuUnit:          price.SkuUnit,
		SkuUnitType:      price.SkuUnitType,
		SkuPriceCurrency: price.SkuPriceCurrency,
		IsCancel:         false,
		EventValue:       amountValue,
	}
	err := DeductAccountFee(ctx, tx, statement)
	if err != nil {
		return fmt.Errorf("deduct account fee error: %w", err)
	}
	return nil
}

func (a *accountSubscriptionStoreImpl) calculateFeeGap(prePrice AccountPrice, postPrice AccountPrice,
	now time.Time, sub *AccountSubscription) (float64, error) {

	feeGap := float64(0)
	priceGap := float64(postPrice.SkuPrice - prePrice.SkuPrice)
	if priceGap > 0 {
		// upgrade price
		startStamp := sub.LastPeriodStart.Unix()
		endStamp := sub.LastPeriodEnd.Unix()
		nowStamp := now.Unix()
		if nowStamp < startStamp || nowStamp > endStamp {
			return 0, errorx.ErrWrongTimeRange
		}
		feePercent := float64(endStamp-nowStamp) / float64(endStamp-startStamp)
		feeGap = priceGap * feePercent
	}

	return feeGap, nil
}
