package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const subscriptionLicenseIssuer = "subscription"

func saveSubscriptionLicense(ctx context.Context, tx bun.Tx, sub *AccountSubscription, template *types.SubscriptionLicenseTemplate) error {
	if template == nil || sub.ID == 0 {
		return nil
	}
	var bill AccountSubscriptionBill
	if err := tx.NewSelect().Model(&bill).Where("id = ?", sub.LastBillID).Scan(ctx); err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("subscription_bill", sub.LastBillID))
	}
	if bill.Status != types.BillingStatusPaid {
		return nil
	}
	if bill.SubID != sub.ID || bill.UserUUID != sub.UserUUID || bill.SkuType != sub.SkuType {
		return fmt.Errorf("subscription bill ownership mismatch for subscription %d", sub.ID)
	}
	key := fmt.Sprintf("subscription:%d:%d", sub.SkuType, sub.ID)
	var license License
	err := tx.NewSelect().Model(&license).Where("key = ?", key).For("UPDATE").Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorx.HandleDBError(err, errorx.Ctx().Set("subscription_license", key))
	}
	if err == nil && (license.UserUUID != sub.UserUUID || license.Product != string(template.Product) || license.Issuer != subscriptionLicenseIssuer) {
		return fmt.Errorf("subscription license ownership mismatch for subscription %d", sub.ID)
	}
	license.Key = key
	license.UserUUID = sub.UserUUID
	license.Company = template.Company
	license.Email = template.Email
	license.Product = string(template.Product)
	license.Edition = template.Edition
	license.MaxUser = template.MaxUser
	license.StartTime = sub.LastPeriodStart
	license.ExpireTime = sub.LastPeriodEnd
	license.Issuer = subscriptionLicenseIssuer
	var result sql.Result
	if errors.Is(err, sql.ErrNoRows) {
		result, err = tx.NewInsert().Model(&license).Exec(ctx)
	} else {
		result, err = tx.NewUpdate().Model(&license).
			Column("company", "email", "edition", "max_user", "start_time", "expire_time", "updated_at").
			WherePK().Exec(ctx)
	}
	err = assertAffectedOneRow(result, err)
	return errorx.HandleDBError(err, errorx.Ctx().Set("subscription_license", key))
}
