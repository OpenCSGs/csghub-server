package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

type AccountSubscriptionUsageStore interface {
	GetByBillID(ctx context.Context, billID int64, userUUID string) ([]AccountSubscriptionUsage, error)
	GetByBillMonth(ctx context.Context, billMonth string, userUUID string) ([]AccountSubscriptionUsage, error)
}

type accountSubscriptionUsageStoreImpl struct {
	db *DB
}

func NewAccountSubscriptionUsageStore() AccountSubscriptionUsageStore {
	return &accountSubscriptionUsageStoreImpl{
		db: defaultDB,
	}
}

func NewAccountSubscriptionUsageWithDB(db *DB) AccountSubscriptionUsageStore {
	return &accountSubscriptionUsageStoreImpl{
		db: db,
	}
}

type AccountSubscriptionUsage struct {
	ID           int64   `bun:",pk,autoincrement" json:"id"`
	UserUUID     string  `bun:",notnull" json:"user_uuid"`
	ResourceID   string  `bun:",notnull" json:"resource_id"`
	ResourceName string  `bun:",notnull" json:"resource_name"`
	CustomerID   string  `bun:",notnull" json:"customer_id"`
	Used         float64 `bun:",notnull" json:"used"`
	Quota        float64 `bun:",notnull" json:"quota"`
	BillID       int64   `bun:",nullzero" json:"bill_id"`    // for pro or team ver
	BillMonth    string  `bun:",nullzero" json:"bill_month"` // YYYY-MM format for free ver
	times
}

func (a *accountSubscriptionUsageStoreImpl) GetByBillID(ctx context.Context, billID int64, userUUID string) ([]AccountSubscriptionUsage, error) {
	var usages []AccountSubscriptionUsage
	err := a.db.Operator.Core.NewSelect().Model(&usages).
		Where("bill_id = ? AND user_uuid = ?", billID, userUUID).
		Scan(ctx, &usages)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return usages, nil
}

func (a *accountSubscriptionUsageStoreImpl) GetByBillMonth(ctx context.Context, billMonth string, userUUID string) ([]AccountSubscriptionUsage, error) {
	var usages []AccountSubscriptionUsage
	err := a.db.Operator.Core.NewSelect().Model(&usages).
		Where("bill_month = ? AND user_uuid = ?", billMonth, userUUID).
		Scan(ctx, &usages)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return usages, nil
}

func UpdateSubscriptionUsage(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	var (
		usageErr error
		usage    AccountSubscriptionUsage
	)

	billMonth := time.Now().Format("2006-01")
	q := tx.NewSelect().Model(&usage)
	if input.SubBillID > 0 {
		q = q.Where("bill_id = ?", input.SubBillID)
	} else {
		q = q.Where("bill_month = ?", billMonth)
	}
	usageErr = q.Where("user_uuid = ?", input.UserUUID).
		Where("resource_id = ?", input.ResourceID).
		Where("resource_name = ?", input.ResourceName).
		Where("customer_id = ?", input.CustomerID).
		Scan(ctx, &usage)
	if usageErr != nil && !errors.Is(usageErr, sql.ErrNoRows) {
		return errorx.HandleDBError(usageErr, errorx.Ctx().Set("query_sub_usage_user_uuid", input.UserUUID))
	}

	if usageErr == nil {
		usage.Used += input.Consumption
		usage.Quota = input.Quota
		_, err := tx.NewUpdate().Model(&usage).WherePK().Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("update_sub_usage_user_uuid", input.UserUUID))
		}
	} else {
		usage = AccountSubscriptionUsage{
			UserUUID:     input.UserUUID,
			ResourceID:   input.ResourceID,
			ResourceName: input.ResourceName,
			CustomerID:   input.CustomerID,
			Used:         input.Consumption,
			Quota:        input.Quota,
		}
		if input.SubBillID > 0 {
			usage.BillID = input.SubBillID
		} else {
			usage.BillMonth = billMonth
		}
		_, err := tx.NewInsert().Model(&usage).Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("insert_sub_usage_user_uuid", input.UserUUID))
		}
	}

	return nil
}
