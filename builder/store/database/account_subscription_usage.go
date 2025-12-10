package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccountSubscriptionUsageStore interface {
	GetByBillID(ctx context.Context, billID int64, userUUID string, skuType types.SKUType) ([]AccountSubscriptionUsage, error)
	GetByBillMonth(ctx context.Context, billMonth string, userUUID string, skuType types.SKUType) ([]AccountSubscriptionUsage, error)
	Create(ctx context.Context, input *AccountSubscriptionUsage) (*AccountSubscriptionUsage, error)
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
	ID           int64                 `bun:",pk,autoincrement" json:"id"`
	UserUUID     string                `bun:",notnull" json:"user_uuid"`
	ResourceID   string                `bun:",notnull" json:"resource_id"`
	ResourceName string                `bun:",notnull" json:"resource_name"`
	CustomerID   string                `bun:",notnull" json:"customer_id"`
	Used         float64               `bun:",notnull" json:"used"`
	Quota        float64               `bun:",notnull" json:"quota"`
	BillID       int64                 `bun:",nullzero" json:"bill_id"`    // for pro or team ver
	BillMonth    string                `bun:",nullzero" json:"bill_month"` // YYYY-MM format for free ver
	ValueType    types.ChargeValueType `bun:",notnull" json:"value_type"`
	SkuType      types.SKUType         `bun:",notnull" json:"sku_type"`
	times
}

func (a *accountSubscriptionUsageStoreImpl) Create(ctx context.Context, input *AccountSubscriptionUsage) (*AccountSubscriptionUsage, error) {
	res, err := a.db.Core.NewInsert().Model(input).Exec(ctx, input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to insert subscription usage, error: %w", err)
	}
	return input, nil
}

func (a *accountSubscriptionUsageStoreImpl) GetByBillID(ctx context.Context, billID int64, userUUID string, skuType types.SKUType) ([]AccountSubscriptionUsage, error) {
	var usages []AccountSubscriptionUsage
	err := a.db.Operator.Core.NewSelect().Model(&usages).
		Where("bill_id = ? AND user_uuid = ? AND sku_type = ?", billID, userUUID, skuType).
		Scan(ctx, &usages)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return usages, nil
}

func (a *accountSubscriptionUsageStoreImpl) GetByBillMonth(ctx context.Context, billMonth string, userUUID string, skuType types.SKUType) ([]AccountSubscriptionUsage, error) {
	var usages []AccountSubscriptionUsage
	err := a.db.Operator.Core.NewSelect().Model(&usages).
		Where("bill_month = ? AND user_uuid = ? AND sku_type = ?", billMonth, userUUID, skuType).
		Scan(ctx, &usages)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return usages, nil
}

func UpdateSubscriptionUsage(ctx context.Context, tx bun.Tx, input AccountStatement, skuType types.SKUType) error {
	var (
		usageErr          error
		usage             AccountSubscriptionUsage
		queryResoureID    string
		queryResourceName string
		queryCustomerID   string
	)

	queryResoureID = input.ResourceID
	queryResourceName = input.ResourceName
	queryCustomerID = input.CustomerID

	if input.SubBillID > 0 && skuType == types.SKUStarship {
		queryResoureID = ""
		queryResourceName = ""
		queryCustomerID = ""
	}

	billMonth := time.Now().Format("2006-01")
	q := tx.NewSelect().Model(&usage)
	if input.SubBillID > 0 {
		q = q.Where("bill_id = ?", input.SubBillID)
	} else {
		q = q.Where("bill_month = ?", billMonth)
	}
	usageErr = q.Where("user_uuid = ?", input.UserUUID).
		Where("resource_id = ?", queryResoureID).
		Where("resource_name = ?", queryResourceName).
		Where("customer_id = ?", queryCustomerID).
		Where("sku_type = ?", skuType).
		Where("value_type = ?", input.ValueType).
		Scan(ctx, &usage)

	if usageErr != nil && !errors.Is(usageErr, sql.ErrNoRows) {
		return errorx.HandleDBError(usageErr, errorx.Ctx().Set("query_sub_usage_user_uuid", input.UserUUID))
	}

	if usageErr == nil {
		if skuType == types.SKUCSGHub {
			usage.Used += input.Value
		} else {
			usage.Used += input.RegularValue
		}
		usage.ValueType = input.ValueType
		res, err := tx.NewUpdate().Model(&usage).WherePK().Exec(ctx)
		err = assertAffectedOneRow(res, err)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("update_sub_usage_user_uuid", input.UserUUID))
		}
	} else {
		usage = AccountSubscriptionUsage{
			UserUUID:     input.UserUUID,
			ResourceID:   queryResoureID,
			ResourceName: queryResourceName,
			CustomerID:   queryCustomerID,
			Quota:        input.Quota,
			SkuType:      skuType,
			ValueType:    input.ValueType,
		}
		if skuType == types.SKUCSGHub {
			usage.Used = input.Value
		} else {
			usage.Used = input.RegularValue
		}
		if input.SubBillID > 0 {
			usage.BillID = input.SubBillID
		} else {
			usage.BillMonth = billMonth
		}
		res, err := tx.NewInsert().Model(&usage).Exec(ctx)
		err = assertAffectedOneRow(res, err)
		if err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("insert_sub_usage_user_uuid", input.UserUUID))
		}
	}

	return nil
}
