package database

import (
	"context"
	"fmt"
	"math"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// CreateTradeStatements creates both a buyer deduction statement and a seller income statement
// in a single database transaction, ensuring atomicity for dataset trade operations.
func (as *accountStatementStoreImpl) CreateTradeStatements(ctx context.Context, buyerStmt, sellerStmt AccountStatement) error {
	return as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// 1. Deduct from buyer
		initEventValue := buyerStmt.Value
		changed, err := DeductAccountFee(ctx, tx, buyerStmt, true)
		if err != nil {
			return fmt.Errorf("deduct buyer fee, error: %w", err)
		}

		calcValue := initEventValue - changed.Voucher
		consumption := 0.0
		if initEventValue != 0 {
			consumption = math.Abs(calcValue/initEventValue) * buyerStmt.Consumption
		}

		err = updateFeeBill(ctx, tx, buyerStmt, BillValues{
			TotalValue:   calcValue,
			VoucherValue: 0,
			CashValue:    changed.Cash,
			Consumption:  consumption,
		})
		if err != nil {
			return fmt.Errorf("update buyer bill, error: %w", err)
		}

		// 2. Credit to seller
		if err := chargeAccountFee(ctx, tx, &sellerStmt); err != nil {
			return fmt.Errorf("credit seller fee, error: %w", err)
		}

		cashValue := 0.0
		if sellerStmt.Scene == types.SceneCashCharge {
			cashValue = sellerStmt.Value
		}

		err = updateFeeBill(ctx, tx, sellerStmt, BillValues{
			TotalValue:   sellerStmt.Value,
			VoucherValue: 0,
			CashValue:    cashValue,
			Consumption:  sellerStmt.Consumption,
		})
		if err != nil {
			return fmt.Errorf("update seller bill, error: %w", err)
		}

		return nil
	})
}

func (as *accountStatementStoreImpl) HasUserPurchasedDataset(ctx context.Context, userUUID string, datasetID int64) (bool, error) {
	if as == nil || as.db == nil {
		return false, nil
	}
	var count int
	err := as.db.Operator.Core.NewSelect().
		Model((*AccountStatement)(nil)).
		ColumnExpr("COUNT(*)").
		Where("user_uuid = ?", userUUID).
		Where("scene = ?", types.SceneDatasetPurchase).
		Where("resource_id = ?", fmt.Sprintf("%d", datasetID)).
		Where("resource_name = ?", "dataset").
		Scan(ctx, &count)

	if err != nil {
		return false, fmt.Errorf("check if user purchased dataset error: %w", err)
	}

	return count > 0, nil
}
