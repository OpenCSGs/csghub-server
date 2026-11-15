package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// SendInviterPresent serializes grants for an inviter with subscription changes.
func (ap *accountPresentStoreImpl) SendInviterPresent(ctx context.Context, gift NewUserPresentGift) (bool, error) {
	awarded := false
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockSubscription(ctx, tx, types.SKUCSGLite, gift.Present.UserUUID); err != nil {
			return err
		}
		if err := ensureAndLockInviterAccount(ctx, tx, gift.Present.UserUUID); err != nil {
			return err
		}
		var existing AccountPresent
		err := tx.NewSelect().Model(&existing).
			Where("user_uuid = ? AND activity_id = ? AND participant_uuid = ?", gift.Present.UserUUID, gift.Present.ActivityID, gift.Present.ParticipantUUID).
			Limit(1).Scan(ctx)
		if err == nil {
			if existing.Status != types.AccountPresentStatusCanceled {
				slog.WarnContext(ctx, "legacy inviter present already exists; skipping gift", "inviter", gift.Present.OpUID, "invitee", gift.Present.ParticipantUUID)
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if gift.Present.OpUID != gift.Present.UserUUID {
			err = tx.NewSelect().Model(&existing).
				Where("user_uuid = ? AND activity_id = ? AND participant_uuid = ?", gift.Present.OpUID, gift.Present.ActivityID, gift.Present.ParticipantUUID).
				Limit(1).Scan(ctx)
			if err == nil {
				slog.WarnContext(ctx, "legacy inviter present already exists under user UUID; skipping gift", "inviter", gift.Present.OpUID, "invitee", gift.Present.ParticipantUUID)
				return nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if err := assertAffectedOneRow(tx.NewInsert().Model(&gift.Present).Exec(ctx)); err != nil {
			return err
		}
		if err := insertGiftAuditStatement(ctx, tx, gift.Present); err != nil {
			return err
		}
		if err := applyInviterSubscription(ctx, tx, gift); err != nil {
			return err
		}
		for i := range gift.Vouchers {
			gift.Vouchers[i].VoucherNo, err = genVoucherNo(ctx, tx)
			if err != nil {
				return err
			}
			if err := assertAffectedOneRow(tx.NewInsert().Model(&gift.Vouchers[i]).Exec(ctx)); err != nil {
				return err
			}
		}
		awarded = true
		return nil
	})
	return awarded, err
}

func ensureAndLockInviterAccount(ctx context.Context, tx bun.Tx, userUUID string) error {
	account := AccountUser{UserUUID: userUUID, Balance: 0, CashBalance: 0}
	if _, err := tx.NewInsert().Model(&account).On("CONFLICT (user_uuid) DO NOTHING").Exec(ctx); err != nil {
		return fmt.Errorf("ensure inviter account: %w", err)
	}
	if err := tx.NewSelect().Model(&account).Where("user_uuid = ?", userUUID).For("UPDATE").Scan(ctx); err != nil {
		return fmt.Errorf("lock inviter account: %w", err)
	}
	return nil
}

func applyInviterSubscription(ctx context.Context, tx bun.Tx, gift NewUserPresentGift) error {
	var sub AccountSubscription
	err := tx.NewSelect().Model(&sub).Where("user_uuid = ? AND sku_type = ?", gift.Present.UserUUID, types.SKUCSGLite).Order("id DESC").Limit(1).For("UPDATE").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return insertNewUserPresentSubscription(ctx, tx, gift)
	}
	if err != nil {
		return err
	}
	now := time.Now()
	if sub.Status == types.SubscriptionStatusActive && sub.LastPeriodEnd.After(now) {
		var bill AccountSubscriptionBill
		if err := tx.NewSelect().Model(&bill).Where("id = ? AND sub_id = ? AND user_uuid = ? AND sku_type = ?", sub.LastBillID, sub.ID, sub.UserUUID, sub.SkuType).Scan(ctx); err != nil {
			return fmt.Errorf("load active inviter subscription bill: %w", err)
		}
		if bill.Status == types.BillingStatusPaid && bill.AmountPaid > 0 {
			return nil
		}
		giftStart := sub.LastPeriodEnd
		sub.LastPeriodEnd = calculateEndTimeByMonth(giftStart, 1)
		if bill.Status != types.BillingStatusPaid {
			giftBill := AccountSubscriptionBill{SubID: sub.ID, EventUUID: gift.Present.EventUUID.String(), UserUUID: sub.UserUUID, Status: types.BillingStatusPaid, Reason: types.BillingReasonSubscriptionCreate, PeriodStart: giftStart, PeriodEnd: sub.LastPeriodEnd, PriceID: sub.PriceID, ResourceID: sub.ResourceID, SkuType: sub.SkuType, Explain: gift.Present.OpDesc}
			if err := assertAffectedOneRow(tx.NewInsert().Model(&giftBill).Exec(ctx)); err != nil {
				return err
			}
			sub.LastBillID = giftBill.ID
		}
		if err := assertAffectedOneRow(tx.NewUpdate().Model(&sub).Column("last_period_end", "last_bill_id", "updated_at").WherePK().Exec(ctx)); err != nil {
			return err
		}
		return saveSubscriptionLicense(ctx, tx, &sub, gift.License)
	}
	return reactivateInviterSubscription(ctx, tx, &sub, gift, now)
}

func reactivateInviterSubscription(ctx context.Context, tx bun.Tx, sub *AccountSubscription, gift NewUserPresentGift, now time.Time) error {
	var price AccountPrice
	query := tx.NewSelect().Model(&price).Where("sku_type = ? AND sku_kind = ? AND resource_id = ? AND sku_unit_type = ?", types.SKUCSGLite, types.SKUTimeSpan, gift.PlanResourceID, types.UnitMonth)
	if err := filterSubscriptionPurchasePrice(query, types.SKUCSGLite).Order("created_at DESC").Limit(1).Scan(ctx); err != nil {
		return err
	}
	end := calculateEndTimeByMonth(now, 1)
	bill := AccountSubscriptionBill{SubID: sub.ID, EventUUID: gift.Present.EventUUID.String(), UserUUID: sub.UserUUID, Status: types.BillingStatusPaid, Reason: types.BillingReasonSubscriptionCreate, PeriodStart: now, PeriodEnd: end, PriceID: price.ID, ResourceID: price.ResourceID, SkuType: price.SkuType, Discount: price.Discount, Explain: gift.Present.OpDesc}
	if err := assertAffectedOneRow(tx.NewInsert().Model(&bill).Exec(ctx)); err != nil {
		return err
	}
	sub.Status = types.SubscriptionStatusActive
	sub.PriceID, sub.ResourceID, sub.LastBillID = price.ID, price.ResourceID, bill.ID
	sub.StartAt, sub.LastPeriodStart, sub.LastPeriodEnd = now, now, end
	sub.EndAt = time.Time{}
	if err := assertAffectedOneRow(tx.NewUpdate().Model(sub).WherePK().Exec(ctx)); err != nil {
		return err
	}
	return saveSubscriptionLicense(ctx, tx, sub, gift.License)
}
