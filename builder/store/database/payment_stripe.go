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

type PaymentStripeStore interface {
	Get(ctx context.Context, id int64) (*PaymentStripe, error)
	Create(ctx context.Context, input PaymentStripe) (*PaymentStripe, error)
	Update(ctx context.Context, input PaymentStripe) (*PaymentStripe, error)
	GetBySessionID(ctx context.Context, sessionID string) (*PaymentStripe, error)
	List(ctx context.Context, req *types.StripeSessionListReq) (*PaymentStripeListResult, error)
}

type PaymentStripeListResult struct {
	Data        []PaymentStripe `json:"data"`
	Total       int             `json:"total"`
	TotalAmount int64           `json:"total_amount"`
}

type PaymentStripe struct {
	ID                 int64     `bun:",pk,autoincrement" json:"id"`
	ClientReferenceID  string    `bun:",notnull,unique" json:"client_reference_id"`
	UserUUID           string    `bun:",notnull" json:"user_uuid"`
	AmountTotal        int64     `bun:",notnull" json:"amount_total"`
	Currency           string    `bun:",notnull" json:"currency"`
	SessionID          string    `bun:",notnull,unique" json:"session_id"`
	SessionStatus      string    `bun:",nullzero" json:"session_status"`
	PaymentStatus      string    `bun:",nullzero" json:"payment_status"`
	SessionCreatedAt   time.Time `bun:",nullzero" json:"session_created_at"`
	SessionCompletedAt time.Time `bun:",nullzero" json:"session_completed_at"`
	SessionExpiresAt   time.Time `bun:",nullzero" json:"session_expires_at"`
	CustomerEmail      string    `bun:",nullzero" json:"customer_email"`
	CustomerName       string    `bun:",nullzero" json:"customer_name"`
	Mode               string    `bun:",nullzero" json:"mode"`
	LiveMode           bool      `bun:",nullzero" json:"live_mode"`
	PaymentIntentID    string    `bun:",nullzero" json:"payment_intent_id"`
	times
}

type paymentStripeStoreImpl struct {
	db *DB
}

func NewPaymentStripeStore() PaymentStripeStore {
	return &paymentStripeStoreImpl{
		db: defaultDB,
	}
}

func NewPaymentStripeStoreWithDB(db *DB) PaymentStripeStore {
	return &paymentStripeStoreImpl{
		db: db,
	}
}

func (s *paymentStripeStoreImpl) Get(ctx context.Context, id int64) (*PaymentStripe, error) {
	var payment PaymentStripe
	err := s.db.Core.NewSelect().Model(&payment).Where("id = ?", id).Scan(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &payment, nil
}

func (s *paymentStripeStoreImpl) Create(ctx context.Context, input PaymentStripe) (*PaymentStripe, error) {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		var acctStatement AccountStatement

		err = tx.NewSelect().Model(&acctStatement).Where("event_uuid = ?", input.ClientReferenceID).Scan(ctx, &acctStatement)

		if err == nil {
			return errorx.ErrDatabaseDuplicateKey
		}

		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check account statement by event uuid %s in create stripe session, error: %w", input.ClientReferenceID, err)
		}

		res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &input, nil
}

func (s *paymentStripeStoreImpl) Update(ctx context.Context, input PaymentStripe) (*PaymentStripe, error) {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.PaymentStatus == types.StripeStatusPaid {
			err := s.doUserCharge(ctx, tx, input)
			if err != nil {
				return err
			}
		}

		res, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}

	return &input, nil
}

func (s *paymentStripeStoreImpl) doUserCharge(ctx context.Context, tx bun.Tx, input PaymentStripe) error {
	var err error
	var acctUser AccountUser
	var acctStatement AccountStatement

	userUUID := input.UserUUID
	eventUUID, err := uuid.Parse(input.ClientReferenceID)
	if err != nil {
		return fmt.Errorf("parse client-reference-id %s in stripe pay, error: %w", input.ClientReferenceID, err)
	}

	err = tx.NewSelect().Model(&acctStatement).Where("event_uuid = ?", eventUUID.String()).Scan(ctx, &acctStatement)
	if err == nil {
		// already processed, skip
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		// unexpected error, return it
		return fmt.Errorf("verfiy account statement by event_uuid %s in stripe pay, error: %w", eventUUID.String(), err)
	}

	// check account user balance
	_, err = CheckUserAccount(ctx, tx, userUUID)
	if err != nil {
		return err
	}

	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", userUUID).Scan(ctx, &acctUser)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			newAcctUser := AccountUser{
				UserUUID:    userUUID,
				Balance:     0,
				CashBalance: 0,
			}
			res, err := tx.NewInsert().Model(&newAcctUser).Exec(ctx, &newAcctUser)
			if err := assertAffectedOneRow(res, err); err != nil {
				return fmt.Errorf("create account user %s in stripe pay, error: %w", userUUID, err)
			}
		} else {
			return fmt.Errorf("get account user %s in stripe pay, error: %w", userUUID, err)
		}
	}

	// for no account statement record
	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", userUUID).For("UPDATE").Scan(ctx, &acctUser)
	if err != nil {
		return fmt.Errorf("get account user %s for lock in stripe pay, error: %w", userUUID, err)
	}

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	err = assertAffectedOneRow(tx.Exec(runSql, input.AmountTotal, userUUID))
	if err != nil {
		return fmt.Errorf("update account user %s cash balance in stripe pay, error:%w", userUUID, err)
	}

	acctUser = AccountUser{}
	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", userUUID).Scan(ctx, &acctUser)
	if err != nil {
		return fmt.Errorf("get account user %s cash balance in stripe pay, error: %w", userUUID, err)
	}

	newAcctStatement := AccountStatement{
		EventUUID:        eventUUID,
		UserUUID:         input.UserUUID,
		Value:            float64(input.AmountTotal),
		Scene:            types.SceneCashCharge,
		OpUID:            input.UserUUID,
		CreatedAt:        time.Now(),
		EventDate:        time.Now(),
		RecordedAt:       time.Now(),
		BalanceType:      types.ChargeCashBalance,
		BalanceValue:     acctUser.CashBalance,
		IsCancel:         false,
		EventValue:       float64(input.AmountTotal),
		SkuPriceCurrency: input.Currency,
	}

	err = assertAffectedOneRow(tx.NewInsert().Model(&newAcctStatement).Exec(ctx))
	if err != nil {
		return fmt.Errorf("create statement for client-reference-id %s in stripe pay, error: %w", eventUUID.String(), err)
	}

	return nil
}

func (s *paymentStripeStoreImpl) GetBySessionID(ctx context.Context, sessionID string) (*PaymentStripe, error) {
	var payment PaymentStripe
	err := s.db.Core.NewSelect().Model(&payment).Where("session_id = ?", sessionID).Scan(ctx)
	if err != nil {
		err = errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &payment, nil
}

func (s *paymentStripeStoreImpl) List(ctx context.Context, req *types.StripeSessionListReq) (*PaymentStripeListResult, error) {
	var paySessions []PaymentStripe

	q := s.db.Operator.Core.NewSelect().Model(&paySessions).
		Where("session_created_at >= ?", req.StartDate).
		Where("session_created_at <= ?", req.EndDate)

	if len(req.QueryUserUUID) > 0 {
		q = q.Where("user_uuid = ?", req.QueryUserUUID)
	}

	if len(req.SessionStatus) > 0 {
		q = q.Where("session_status = ?", req.SessionStatus)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	type SumResult struct {
		TotalAmount int64 `bun:"total_amount"`
	}
	var sumResult SumResult

	sumQuery := s.db.Operator.Core.NewSelect().Model((*PaymentStripe)(nil)).
		ColumnExpr("COALESCE(SUM(amount_total), 0) as total_amount").
		Where("session_created_at >= ?", req.StartDate).
		Where("session_created_at <= ?", req.EndDate)

	if len(req.QueryUserUUID) > 0 {
		sumQuery = sumQuery.Where("user_uuid = ?", req.QueryUserUUID)
	}

	if len(req.SessionStatus) > 0 {
		sumQuery = sumQuery.Where("session_status = ?", req.SessionStatus)
	}

	err = sumQuery.Scan(ctx, &sumResult)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &paySessions)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	result := &PaymentStripeListResult{
		Data:        paySessions,
		Total:       count,
		TotalAmount: sumResult.TotalAmount,
	}

	return result, nil
}
