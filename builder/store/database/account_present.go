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
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type accountPresentStoreImpl struct {
	db *DB
}

type AccountPresentStore interface {
	AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error
	SendNewUserPresent(ctx context.Context, gift NewUserPresentGift) (bool, error)
	FindPresentByUserAndActivity(ctx context.Context, userID string, activityID int64, participantUUID string) (*AccountPresent, error)
	ListExpiredPresentsByActivityID(ctx context.Context, activityID int64) ([]AccountPresent, error)
	ListAutoExpirePresents(ctx context.Context, activityID int64, grantedStart, grantedEnd time.Time, validDays int, afterID int64, limit int) ([]AutoExpirePresent, error)
	DeductExpiredPresentBalance(ctx context.Context, present AutoExpirePresent, statement AccountStatement) (float64, error)
	HasConsumptionActivity(ctx context.Context, userUUID string, startAt time.Time) (bool, error)
	MarkPresentAsUsed(ctx context.Context, eventUUID uuid.UUID) error
	CancelPresent(ctx context.Context, eventUUID uuid.UUID) error
	ListPresents(ctx context.Context, filter PresentFilter) ([]AccountPresent, int, error)
	FindPresentByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*AccountPresent, error)
}

type PresentFilter struct {
	UserUUID  string
	StartDate string
	EndDate   string
	Limit     int
	Offset    int
}

// AutoExpirePresent is one activity present selected for auto expiry: it was
// granted inside the configured window, is still owned (status init, expire_at
// not set), and its valid duration has passed. CreatedAt is the present's grant
// time (account_presents.created_at); ExpireAt is the intended expiry moment
// derived from it.
type AutoExpirePresent struct {
	ID         int64
	UserUUID   string
	ActivityID int64
	Value      float64
	CreatedAt  time.Time
	ExpireAt   time.Time
}

// ErrAutoExpirePresentAlreadyProcessed means another transaction claimed the
// present before this deduction could run. The caller must skip it: no balance
// was deducted and no statement was inserted.
var ErrAutoExpirePresentAlreadyProcessed = errors.New("auto expire present already processed")

func NewAccountPresentStore() AccountPresentStore {
	return &accountPresentStoreImpl{
		db: defaultDB,
	}
}

func NewAccountPresentStoreWithDB(db *DB) AccountPresentStore {
	return &accountPresentStoreImpl{
		db: db,
	}
}

type AccountPresent struct {
	ID              int64                      `bun:",pk,autoincrement" json:"id"`
	EventUUID       uuid.UUID                  `bun:"type:uuid,notnull" json:"event_uuid"`
	UserUUID        string                     `bun:",notnull" json:"user_uuid"`
	ActivityID      int64                      `bun:",notnull" json:"activity_id"`
	Value           float64                    `bun:",notnull" json:"value"`
	OpUID           string                     `bun:",notnull" json:"op_uid"`
	OpDesc          string                     `bun:",notnull" json:"op_desc"`
	ParticipantUUID string                     `bun:",notnull" json:"participant_uuid"`
	ExpireAt        time.Time                  `bun:",nullzero" json:"expire_at"`
	Status          types.AccountPresentStatus `bun:",default:0" json:"status"`
	times
}

// CSGLiteMonthlyResourceID is the monthly CSGLite subscription plan seeded by
// the csglite subscription prices migration.
const CSGLiteMonthlyResourceID = "csglite_sub_bymonth"

// NewUserPresentGift is everything one new-user registration present grants:
// the present record anchoring idempotency, the vouchers granted with it, the
// monthly CSGLite plan granted for one free month, and the license template
// that month issues. The caller fills the voucher validity, targets, present
// metadata and license owner; voucher face values, the present total, voucher
// numbers, the subscription, its bill and the license row are derived inside
// the transaction from the activity configuration.
type NewUserPresentGift struct {
	Present        AccountPresent
	Vouchers       []AccountVoucher
	PlanResourceID string
	License        *types.SubscriptionLicenseTemplate
}

func (ap *accountPresentStoreImpl) AddPresent(ctx context.Context, input AccountPresent, statement AccountStatement) error {
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account present, error:%w", err)
		}

		if err := assertAffectedOneRow(tx.NewInsert().Model(&statement).Exec(ctx)); err != nil {
			return fmt.Errorf("insert account statement, error:%w", err)
		}

		var accountUser AccountUser
		if err := tx.NewSelect().Model(&accountUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE NOWAIT").Scan(ctx, &accountUser); err != nil {
			return fmt.Errorf("update user account to add present, error:%w", err)
		}

		runSql := "update account_users set balance=balance + ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
			return fmt.Errorf("update account balance, error:%w", err)
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to add present", slog.Int64("activity_id", input.ActivityID), slog.String("user_uuid", input.UserUUID), slog.Any("error", err))
	}

	return err
}

// SendNewUserPresent grants the new-user registration present atomically: it
// inserts the present record, one month of CSGLite subscription billed at zero
// that the renewal worker cancels when the gifted period ends, the gift's
// vouchers, and the CSGLite license covering the gifted period. Each voucher
// face value and the present total come from the activity's voucher_value
// configuration, not from the caller, so a caller cannot choose the granted
// amount. The subscription part is skipped when the user already has any
// CSGLite subscription, so a paid entitlement is never overwritten; the present
// and vouchers are still granted. The present record is expected to be marked
// canceled with an expire_at so the auto-expire scan never deducts the voucher
// total from the account balance. The transaction takes the subscription
// advisory lock of (csglite, user) before locking the account row, following the
// same order as the paid purchase and renewal paths. That serializes the gift
// with a concurrent purchase, so the "no subscription yet" check and the
// subscription insert cannot interleave into two active rows, and concurrent
// gifts for one user serialize as well. A user who already has a present for
// the activity is skipped without touching any table, which makes repeated
// calls idempotent. Before the present is created the activity must be active
// and now must fall inside its configured [begin_at, end_at] window (a zero
// bound is open), and it must have a positive voucher value; otherwise
// ErrActivityNotGrantable is returned and nothing is written. The account
// balance is never modified. The boolean result reports whether this call
// created the gift.
func (ap *accountPresentStoreImpl) SendNewUserPresent(ctx context.Context, gift NewUserPresentGift) (bool, error) {
	awarded := false
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		awarded = false

		// Serialize with a concurrent paid purchase or renewal of the same
		// subscription, and take this lock before the account row lock so the
		// order matches the purchase/renewal paths (subscription -> account).
		// Without it both sides could read "no subscription" and insert two
		// active rows, because (user_uuid, sku_type) is not unique.
		if err := lockSubscription(ctx, tx, types.SKUCSGLite, gift.Present.UserUUID); err != nil {
			return fmt.Errorf("lock subscription for new user present, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
		}

		var accountUser AccountUser
		err := tx.NewSelect().Model(&accountUser).
			Where("user_uuid = ?", gift.Present.UserUUID).
			For("UPDATE").Scan(ctx, &accountUser)
		if err != nil {
			return fmt.Errorf("lock account user for new user present, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
		}

		var existing AccountPresent
		q := tx.NewSelect().Model(&existing).
			Where("user_uuid = ? and activity_id = ?", gift.Present.UserUUID, gift.Present.ActivityID)

		err = q.Limit(1).Scan(ctx, &existing)
		switch {
		case err == nil:
			// already granted before: leave every table untouched
			return nil
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("find existing new user present, user_uuid:%s, activity_id:%d, error:%w",
				gift.Present.UserUUID, gift.Present.ActivityID, err)
		}

		// the activity must be open right now, otherwise the gift is refused
		// before any row is written

		act, err := findGrantableActivity(ctx, tx, gift.Present.ActivityID, time.Now().In(config.GetGlobalTimeZone()))
		if err != nil {
			return err
		}
		if act.VoucherValue <= 0 {
			return fmt.Errorf("%w: activity %d has no positive voucher value",
				ErrActivityNotGrantable, gift.Present.ActivityID)
		}

		voucherValue := float64(act.VoucherValue)
		gift.Present.Value = 0
		for i := range gift.Vouchers {
			gift.Vouchers[i].Total = voucherValue
			gift.Present.Value += gift.Vouchers[i].Total
		}

		if err := assertAffectedOneRow(tx.NewInsert().Model(&gift.Present).Exec(ctx)); err != nil {
			return fmt.Errorf("insert new user present, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
		}

		if err := insertNewUserPresentSubscription(ctx, tx, gift); err != nil {
			return err
		}

		for i := range gift.Vouchers {
			voucherNo, err := genVoucherNo(ctx, tx)
			if err != nil {
				return fmt.Errorf("generate new user present voucher number, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
			}
			gift.Vouchers[i].VoucherNo = voucherNo
			if err := assertAffectedOneRow(tx.NewInsert().Model(&gift.Vouchers[i]).Exec(ctx)); err != nil {
				return fmt.Errorf("insert new user present voucher, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
			}
		}

		awarded = true
		return nil
	})

	if err != nil {
		return false, err
	}
	return awarded, nil
}

// insertNewUserPresentSubscription creates the gifted one-month CSGLite
// subscription, its zero-amount paid bill and the customer license covering the
// gifted period. The bill is inserted before the subscription because
// account_subscriptions.last_bill_id is unique and a placeholder zero would
// serialize every concurrent gift on one value. The subscription deliberately
// carries no next price, so the renewal worker cancels it automatically when
// the gifted month ends. The license row is written after the bill is linked so
// downloads during the gifted month resolve exactly like a paid month. A user
// who already has any CSGLite subscription row keeps it: overwriting the row
// would lose the paid entitlement, and inserting a newer one would shadow it in
// latest-row reads. SendNewUserPresent takes the subscription advisory lock
// before this check, so a concurrent purchase cannot slip a row in between the
// check and the insert. The gift therefore skips the subscription and license
// part and still grants its present and vouchers.
func insertNewUserPresentSubscription(ctx context.Context, tx bun.Tx, gift NewUserPresentGift) error {
	var existing AccountSubscription
	err := tx.NewSelect().Model(&existing).
		Where("user_uuid = ?", gift.Present.UserUUID).
		Where("sku_type = ?", types.SKUCSGLite).
		Limit(1).Scan(ctx, &existing)
	switch {
	case err == nil:
		slog.Warn("skip gifted csglite subscription because the user already has one",
			slog.String("user_uuid", gift.Present.UserUUID),
			slog.Int64("subscription_id", existing.ID),
			slog.String("subscription_status", string(existing.Status)))
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("find existing csglite subscription for new user present, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
	}

	var price AccountPrice
	priceQuery := tx.NewSelect().Model(&price).
		Where("sku_type = ?", types.SKUCSGLite).
		Where("sku_kind = ?", types.SKUTimeSpan).
		Where("resource_id = ?", gift.PlanResourceID).
		Where("sku_unit_type = ?", types.UnitMonth)
	priceQuery = filterSubscriptionPurchasePrice(priceQuery, types.SKUCSGLite)
	if err := priceQuery.Order("created_at desc").Limit(1).Scan(ctx, &price); err != nil {
		return fmt.Errorf("find csglite monthly price for new user present, resource_id:%s, error:%w", gift.PlanResourceID, err)
	}

	now := time.Now()
	periodEnd, err := calculatePeriodEndTime(now, &price, now, 0)
	if err != nil {
		return fmt.Errorf("calculate gifted subscription period end, resource_id:%s, error:%w", gift.PlanResourceID, err)
	}

	bill := AccountSubscriptionBill{
		EventUUID:   gift.Present.EventUUID.String(),
		UserUUID:    gift.Present.UserUUID,
		AmountPaid:  0,
		Status:      types.BillingStatusPaid,
		Reason:      types.BillingReasonSubscriptionCreate,
		PeriodStart: now,
		PeriodEnd:   periodEnd,
		PriceID:     price.ID,
		ResourceID:  price.ResourceID,
		Explain:     gift.Present.OpDesc,
		SkuType:     price.SkuType,
		Discount:    price.Discount,
	}
	if err := assertAffectedOneRow(tx.NewInsert().Model(&bill).Exec(ctx, &bill)); err != nil {
		return fmt.Errorf("insert gifted subscription bill, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
	}

	sub := AccountSubscription{
		UserUUID:        gift.Present.UserUUID,
		SkuType:         price.SkuType,
		PriceID:         price.ID,
		ResourceID:      price.ResourceID,
		Status:          types.SubscriptionStatusActive,
		ActionUser:      gift.Present.OpUID,
		StartAt:         now,
		LastBillID:      bill.ID,
		LastPeriodStart: now,
		LastPeriodEnd:   periodEnd,
		AmountPaidCount: 1,
		// NextPriceID and NextResourceID stay empty on purpose: the renewal
		// worker finds no next price when the gifted period ends and cancels
		// the subscription without charging the user.
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&sub).Exec(ctx, &sub)); err != nil {
		return fmt.Errorf("insert gifted subscription, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
	}

	bill.SubID = sub.ID
	if err := assertAffectedOneRow(tx.NewUpdate().Model(&bill).WherePK().Exec(ctx)); err != nil {
		return fmt.Errorf("link gifted subscription bill, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
	}

	// the license needs the linked bill, so it is written last
	if err := saveSubscriptionLicense(ctx, tx, &sub, gift.License); err != nil {
		return fmt.Errorf("issue gifted subscription license, user_uuid:%s, error:%w", gift.Present.UserUUID, err)
	}
	return nil
}

func (ap *accountPresentStoreImpl) FindPresentByUserAndActivity(ctx context.Context, userID string, activityID int64, participantUUID string) (*AccountPresent, error) {
	present := &AccountPresent{}
	q := ap.db.Core.NewSelect().Model(present).Where("user_uuid = ? and activity_id = ?", userID, activityID)
	if len(participantUUID) > 0 {
		q = q.Where("participant_uuid = ?", participantUUID)
	}

	if err := q.Limit(1).Scan(ctx, present); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		slog.Error("failed to find account present by user id and activity id", slog.String("user_id", userID), slog.Int64("activity_id", activityID), slog.Any("error", err))
		return nil, fmt.Errorf("find account present by user id and activity id, error:%w", err)
	}
	return present, nil
}

func (ap *accountPresentStoreImpl) ListExpiredPresentsByActivityID(ctx context.Context, activityID int64) ([]AccountPresent, error) {
	var presents []AccountPresent
	err := ap.db.Core.NewSelect().Model(&presents).
		Where("activity_id = ?", activityID).
		Where("status = ?", types.AccountPresentStatusInit).
		Where("expire_at < ?", time.Now()).
		Where("expire_at is not null").
		Scan(ctx, &presents)
	if err != nil {
		slog.Error("failed to list expired presents by activity id", slog.Int64("activity_id", activityID), slog.Any("error", err))
		return nil, fmt.Errorf("list expired presents by activity id, activity_id:%d, error:%w", activityID, err)
	}
	return presents, nil
}

// ListAutoExpirePresents returns activity presents granted between grantedStart
// (inclusive) and grantedEnd (exclusive) at least validDays ago, judged by
// account_presents.created_at. The expire_at backfill marker keeps the scan
// idempotent: processed presents drop out on the next run.
// ListAutoExpirePresents returns one keyset page of activity presents granted
// between grantedStart (inclusive) and grantedEnd (exclusive) at least
// validDays ago, judged by account_presents.created_at. afterID is the last ID
// from the previous page (zero for the first page) and limit bounds the page.
func (ap *accountPresentStoreImpl) ListAutoExpirePresents(ctx context.Context, activityID int64, grantedStart, grantedEnd time.Time, validDays int, afterID int64, limit int) ([]AutoExpirePresent, error) {
	var presents []AutoExpirePresent
	createdBefore := time.Now().AddDate(0, 0, -validDays)
	query := ap.db.Core.NewSelect().
		ColumnExpr("id, user_uuid, activity_id, value, created_at").
		TableExpr("account_presents").
		Where("activity_id = ?", activityID).
		Where("status = ?", types.AccountPresentStatusInit).
		Where("expire_at IS NULL").
		Where("created_at >= ?", grantedStart).
		Where("created_at < ?", grantedEnd).
		Where("created_at <= ?", createdBefore)
	if afterID > 0 {
		query = query.Where("id > ?", afterID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("id").Scan(ctx, &presents)
	if err != nil {
		slog.Error("failed to list auto expire presents", slog.Int64("activity_id", activityID), slog.Any("error", err))
		return nil, fmt.Errorf("list auto expire presents, activity_id:%d, error:%w", activityID, err)
	}
	for i := range presents {
		presents[i].ExpireAt = presents[i].CreatedAt.AddDate(0, 0, validDays)
	}
	return presents, nil
}

// DeductExpiredPresentBalance enforces expiry of one user's activity presents
// inside a single transaction. It first claims the present row (status init and
// expire_at empty) so concurrent workers cannot deduct the same present twice;
// a claim of zero rows returns ErrAutoExpirePresentAlreadyProcessed without
// inserting a statement or changing the balance. After claiming, it locks the
// account_users row to serialize with concurrent balance changes, deducts
// min(balance, present value) when the balance is positive, and records a
// negative account statement. Users with a non-positive balance are claimed but
// not deducted. It returns the actually deducted value.
func (ap *accountPresentStoreImpl) DeductExpiredPresentBalance(ctx context.Context, present AutoExpirePresent, statement AccountStatement) (float64, error) {
	deducted := 0.0
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		deducted = 0

		claimResult, err := tx.NewUpdate().Model(&AccountPresent{}).
			Set("status = ?", types.AccountPresentStatusCanceled).
			Set("expire_at = ?", present.ExpireAt).
			Where("id = ? AND status = ? AND expire_at IS NULL",
				present.ID, types.AccountPresentStatusInit).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("claim expired present for deduction, user_uuid:%s, error:%w", present.UserUUID, err)
		}
		claimed, err := claimResult.RowsAffected()
		if err != nil {
			return fmt.Errorf("check expired present claim, user_uuid:%s, error:%w", present.UserUUID, err)
		}
		if claimed == 0 {
			return ErrAutoExpirePresentAlreadyProcessed
		}

		var accountUser AccountUser
		err = tx.NewSelect().Model(&accountUser).Where("user_uuid = ?", present.UserUUID).For("UPDATE").Scan(ctx, &accountUser)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// account row missing: nothing to deduct, present stays claimed
		case err != nil:
			return fmt.Errorf("lock account user for expired present deduction, user_uuid:%s, error:%w", present.UserUUID, err)
		case accountUser.Balance > 0:
			deducted = present.Value
			if accountUser.Balance < present.Value {
				deducted = accountUser.Balance
			}
			statement.Value = -deducted
			statement.RegularValue = -deducted
			statement.BalanceValuePrev = accountUser.Balance
			statement.BalanceValue = accountUser.Balance - deducted
			statement.BalanceType = types.ChargeBalance
			if err := assertAffectedOneRow(tx.NewInsert().Model(&statement).Exec(ctx)); err != nil {
				return fmt.Errorf("insert expired present deduction statement, user_uuid:%s, error:%w", present.UserUUID, err)
			}
			runSql := "update account_users set balance=balance - ? where user_uuid=?"
			if err := assertAffectedOneRow(tx.Exec(runSql, deducted, present.UserUUID)); err != nil {
				return fmt.Errorf("deduct account balance for expired present, user_uuid:%s, error:%w", present.UserUUID, err)
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAutoExpirePresentAlreadyProcessed) {
			return 0, err
		}
		slog.Error("failed to deduct expired present balance",
			slog.String("user_uuid", present.UserUUID),
			slog.Int64("activity_id", present.ActivityID),
			slog.Any("error", err))
		return 0, err
	}
	return deducted, nil
}

func (ap *accountPresentStoreImpl) HasConsumptionActivity(ctx context.Context, userUUID string, startAt time.Time) (bool, error) {
	consumptionScenes := []types.SceneType{
		types.ScenePayOrder,
		types.ScenePaySubscription,
		types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless,
		types.SceneStarship,
		types.SceneGuiAgent,
	}
	count, err := ap.db.Core.NewSelect().Model(&AccountStatement{}).
		Where("user_uuid = ?", userUUID).
		Where("created_at >= ?", startAt).
		Where("scene in (?)", bun.In(consumptionScenes)).
		Count(ctx)
	if err != nil {
		slog.Error("failed to check present consumed", slog.String("user_uuid", userUUID), slog.Time("start_at", startAt), slog.Any("error", err))
		return false, fmt.Errorf("check present consumed, user_uuid:%s, start_at:%s, error:%w", userUUID, startAt, err)
	}
	return count > 0, nil
}

func (ap *accountPresentStoreImpl) MarkPresentAsUsed(ctx context.Context, eventUUID uuid.UUID) error {
	_, err := ap.db.Operator.Core.NewUpdate().Model(&AccountPresent{}).Set("status = ?", types.AccountPresentStatusUsed).Where("event_uuid = ?", eventUUID).Exec(ctx)
	if err != nil {
		slog.Error("failed to mark present as used", slog.String("event_uuid", eventUUID.String()), slog.Any("error", err))
		return fmt.Errorf("mark present as used, event_uuid:%s, error:%w", eventUUID.String(), err)
	}
	return nil
}

func (ap *accountPresentStoreImpl) CancelPresent(ctx context.Context, eventUUID uuid.UUID) error {
	err := ap.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var present AccountPresent
		if err := tx.NewSelect().Model(&present).Where("event_uuid = ?", eventUUID).Scan(ctx, &present); err != nil {
			return fmt.Errorf("get account present, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		if err := assertAffectedOneRow(tx.NewUpdate().Model(&AccountPresent{}).Set("status = ?", types.AccountPresentStatusCanceled).Where("event_uuid = ?", eventUUID).Exec(ctx)); err != nil {
			return fmt.Errorf("update account present status, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		if err := assertAffectedOneRow(tx.NewUpdate().Model(&AccountStatement{}).Set("is_cancel = ?", true).Where("event_uuid = ?", eventUUID).Exec(ctx)); err != nil {
			return fmt.Errorf("update account statement to cancel, event_uuid:%s, error:%w", eventUUID.String(), err)
		}

		var accountUser AccountUser
		if err := tx.NewSelect().Model(&accountUser).Where("user_uuid = ?", present.UserUUID).For("UPDATE NOWAIT").Scan(ctx, &accountUser); err != nil {
			return fmt.Errorf("update user account to cancel, user_uuid:%s, error:%w", present.UserUUID, err)
		}

		runSql := "update account_users set balance=balance - ? where user_uuid=?"
		if err := assertAffectedOneRow(tx.Exec(runSql, present.Value, present.UserUUID)); err != nil {
			return fmt.Errorf("update account balance, user_uuid:%s, error:%w", present.UserUUID, err)
		}

		return nil
	})

	if err != nil {
		slog.Error("failed to cancel present", slog.String("event_uuid", eventUUID.String()), slog.Any("error", err))
	}
	return err
}

func (ap *accountPresentStoreImpl) ListPresents(ctx context.Context, filter PresentFilter) ([]AccountPresent, int, error) {
	var presents []AccountPresent
	tx := ap.db.Operator.Core.NewSelect().Model(&presents)

	// Apply filters
	if filter.UserUUID != "" {
		tx = tx.Where("user_uuid = ?", filter.UserUUID)
	}

	if filter.StartDate != "" {
		tx = tx.Where("created_at >= ?", filter.StartDate)
	}

	if filter.EndDate != "" {
		tx = tx.Where("created_at <= ?", filter.EndDate+" 23:59:59")
	}

	// Get total count
	total, err := tx.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count presents error, %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		tx = tx.Limit(filter.Limit)
	}

	if filter.Offset > 0 {
		tx = tx.Offset(filter.Offset)
	}

	// Order by created_at descending
	tx = tx.Order("created_at DESC")

	// Execute query
	if err := tx.Scan(ctx); err != nil {
		return nil, 0, fmt.Errorf("list presents error, %w", err)
	}

	return presents, total, nil
}

func (ap *accountPresentStoreImpl) FindPresentByEventUUID(ctx context.Context, eventUUID uuid.UUID) (*AccountPresent, error) {
	var present AccountPresent
	if err := ap.db.Operator.Core.NewSelect().Model(&present).Where("event_uuid = ?", eventUUID).Scan(ctx, &present); err != nil {
		return nil, fmt.Errorf("get account present, event_uuid:%s, error:%w", eventUUID.String(), err)
	}
	return &present, nil
}
