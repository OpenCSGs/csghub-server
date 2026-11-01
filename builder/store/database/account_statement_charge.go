package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// chargeAccountFee handles the core logic for charging (adding) funds to an account.
// It performs: upsert account_user → lock account → check duplicate → update balance → insert statement.
// It does NOT call updateFeeBill — the caller is responsible for that (same pattern as DeductAccountFee).
// The input.AccountStatement is modified in-place: BalanceType and BalanceValue are set.
func chargeAccountFee(ctx context.Context, tx bun.Tx, input *AccountStatement) error {
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

	err = CheckDuplicatedEvent(ctx, tx, *input)
	if err != nil {
		if errors.Is(err, types.ErrDuplicatedEvent) {
			slog.Warn("skip duplicated charge fee by event uuid", slog.Any("input", input))
			return nil
		}
		return fmt.Errorf("check duplicated charge event failed, error: %w", err)
	}

	runSql := ""
	if input.Scene == types.SceneCashCharge {
		input.BalanceValuePrev = acctUser.CashBalance
		input.BalanceType = types.ChargeCashBalance
		runSql = "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	} else {
		input.BalanceValuePrev = acctUser.Balance
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

	err = assertAffectedOneRow(tx.NewInsert().Model(input).Exec(ctx))
	if err != nil {
		return fmt.Errorf("insert statement, error:%w", err)
	}

	return nil
}

func (as *accountStatementStoreImpl) chargeFeeStatement(ctx context.Context, input AccountStatement) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := chargeAccountFee(ctx, tx, &input); err != nil {
			return err
		}

		cashValue := 0.0
		if input.Scene == types.SceneCashCharge {
			cashValue = input.Value
		}

		err := updateFeeBill(ctx, tx, input, BillValues{
			TotalValue:   input.Value,
			VoucherValue: 0,
			CashValue:    cashValue,
			Consumption:  input.Consumption,
		})
		if err != nil {
			return fmt.Errorf("update account bill for charge, error: %w", err)
		}
		return nil
	})

	return err
}
