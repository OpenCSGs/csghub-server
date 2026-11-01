package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestAccountStatementStore_DeductWithTokenVoucher covers one serverless
// statement partially paid by a token voucher: the voucher bill row and the
// cash bill row must each carry their fee share of the usage detail, and both
// rows must add back to the statement's full token counts, duration and
// consumption.
func TestAccountStatementStore_DeductWithTokenVoucher(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		au := database.AccountUser{UserUUID: "voucher-user"}
		_, err := tx.NewInsert().Model(&au).Exec(ctx)
		require.Nil(t, err)

		voucher := database.AccountVoucher{
			VoucherNo:   "VC-TEST-TOKEN-1",
			TargetType:  database.UserNamespace,
			TargetUUID:  "voucher-user",
			TargetName:  "voucher-user",
			Total:       40,
			BeginDate:   time.Now().Add(-24 * time.Hour),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      types.VoucherStatusActive,
			IssueUUID:   "admin-uuid",
			IssueName:   "admin",
			VoucherType: types.VoucherTypeToken,
			Rules: types.VoucherRules{
				TokenLimits: types.VoucherTokenRule{
					Models: types.RuleItem{Mode: types.RuleItemTypeAny},
				},
			},
		}
		_, err = tx.NewInsert().Model(&voucher).Exec(ctx)
		require.Nil(t, err)

		seedAPIKeyQuotas(t, ctx, tx, 7, "7")
		return nil
	})
	require.Nil(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	err = store.Create(ctx, database.AccountStatement{
		EventUUID:       uuid.New(),
		UserUUID:        "voucher-user",
		Value:           -100,
		Scene:           types.SceneModelServerless,
		ResourceID:      "csghub://serverless/ns/model",
		CustomerID:      "ns/model",
		EventDate:       time.Now(),
		Consumption:     1000,
		PromptToken:     100,
		CompletionToken: 300,
		Duration:        10,
		APIKey:          "7",
		TokenID:         7,
	}, types.AcctStatementExtra{})
	require.Nil(t, err)

	// the voucher pays 40 of the 100 fee; the overage runs the cash balances
	// negative through the all-in fallback
	var voucherRow database.AccountVoucher
	err = db.Core.NewSelect().Model(&voucherRow).
		Where("voucher_no = ?", "VC-TEST-TOKEN-1").Scan(ctx, &voucherRow)
	require.Nil(t, err)
	require.Equal(t, -40.0, voucherRow.Used)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).
		Where("user_uuid = ?", "voucher-user").Scan(ctx, &acctUser)
	require.Nil(t, err)
	require.Equal(t, -60.0, acctUser.CashBalance)

	var bills []database.AccountBill
	err = db.Core.NewSelect().Model(&bills).
		Where("user_uuid = ?", "voucher-user").Scan(ctx, &bills)
	require.Nil(t, err)
	require.Len(t, bills, 2)

	voucherBill := database.AccountBill{}
	cashBill := database.AccountBill{}
	for _, bill := range bills {
		if bill.VoucherNo == "VC-TEST-TOKEN-1" {
			voucherBill = bill
		} else {
			cashBill = bill
		}
	}
	require.NotZero(t, voucherBill.ID)
	require.NotZero(t, cashBill.ID)

	require.Equal(t, -40.0, voucherBill.Value)
	require.Equal(t, -40.0, voucherBill.VoucherValue)
	require.Equal(t, 0.0, voucherBill.CashValue)
	require.Equal(t, 400.0, voucherBill.Consumption)
	require.Equal(t, 40.0, voucherBill.PromptToken)
	require.Equal(t, 120.0, voucherBill.CompletionToken)
	require.Equal(t, 4.0, voucherBill.Duration)

	require.Equal(t, -60.0, cashBill.Value)
	require.Equal(t, 0.0, cashBill.VoucherValue)
	require.Equal(t, -60.0, cashBill.CashValue)
	require.Equal(t, 600.0, cashBill.Consumption)
	require.Equal(t, 60.0, cashBill.PromptToken)
	require.Equal(t, 180.0, cashBill.CompletionToken)
	require.Equal(t, 6.0, cashBill.Duration)

	// both rows add back to the statement's full usage detail
	require.Equal(t, 100.0, voucherBill.PromptToken+cashBill.PromptToken)
	require.Equal(t, 300.0, voucherBill.CompletionToken+cashBill.CompletionToken)
	require.Equal(t, 10.0, voucherBill.Duration+cashBill.Duration)
	require.Equal(t, 1000.0, voucherBill.Consumption+cashBill.Consumption)

	// the split into a voucher row and a cash row must not count the statement
	// usage twice on the API key quotas
	requireAPIKeyQuotaUsage(t, ctx, db, 7, 1000, -100)
}

// TestAccountStatementStore_DeductWithTokenPlanAndVoucher covers one serverless
// statement that a token plan partially covers and a token voucher partially
// pays: the plan, voucher and cash bill rows must each carry their fee share of
// the usage detail, and all three rows must add back to the statement's full
// token counts, duration and consumption. The voucher share is computed against
// the post-plan fee, so it has to be mapped back onto the full detail by the
// plan's non-plan usage fraction.
func TestAccountStatementStore_DeductWithTokenPlanAndVoucher(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		au := database.AccountUser{UserUUID: "plan-voucher-user", CashBalance: 100}
		_, err := tx.NewInsert().Model(&au).Exec(ctx)
		require.Nil(t, err)

		voucher := database.AccountVoucher{
			VoucherNo:   "VC-TEST-PLAN-TOKEN-1",
			TargetType:  database.UserNamespace,
			TargetUUID:  "plan-voucher-user",
			TargetName:  "plan-voucher-user",
			Total:       40,
			BeginDate:   time.Now().Add(-24 * time.Hour),
			EndDate:     time.Now().Add(24 * time.Hour),
			Status:      types.VoucherStatusActive,
			IssueUUID:   "admin-uuid",
			IssueName:   "admin",
			VoucherType: types.VoucherTypeToken,
			Rules: types.VoucherRules{
				TokenLimits: types.VoucherTokenRule{
					Models: types.RuleItem{Mode: types.RuleItemTypeAny},
				},
			},
		}
		_, err = tx.NewInsert().Model(&voucher).Exec(ctx)
		require.Nil(t, err)

		// the period's model quota has half of its 1000 tokens left, so the plan
		// absorbs 500 of the statement's 1000 tokens: the fee drops from -100 to
		// -50 and the non-plan usage fraction is 0.5
		_, err = tx.NewInsert().Model(&database.AccountTokenPlanPeriodModelQuota{
			PeriodID: 601, ModelID: "ns/model", TaskType: string(types.TextGeneration),
			QuotaTokens: 1000, UsedTokens: 500,
		}).Exec(ctx)
		require.Nil(t, err)

		seedAPIKeyQuotas(t, ctx, tx, 7, "7")
		return nil
	})
	require.Nil(t, err)

	store := database.NewAccountStatementStoreWithDB(db)
	err = store.Create(ctx, database.AccountStatement{
		EventUUID:         uuid.New(),
		UserUUID:          "plan-voucher-user",
		Value:             -100,
		Scene:             types.SceneModelServerless,
		ResourceID:        "csghub://serverless/ns/model",
		CustomerID:        "ns/model",
		EventDate:         time.Now(),
		Consumption:       1000,
		PromptToken:       100,
		CompletionToken:   300,
		Duration:          10,
		ValueType:         types.TokenNumberType,
		TokenPlanPeriodID: 601,
		TokenID:           7,
		APIKey:            "7",
		RecordedAt:        time.Now(),
	}, types.AcctStatementExtra{})
	require.Nil(t, err)

	var voucherRow database.AccountVoucher
	err = db.Core.NewSelect().Model(&voucherRow).
		Where("voucher_no = ?", "VC-TEST-PLAN-TOKEN-1").Scan(ctx, &voucherRow)
	require.Nil(t, err)
	require.Equal(t, -40.0, voucherRow.Used)

	var acctUser database.AccountUser
	err = db.Core.NewSelect().Model(&acctUser).
		Where("user_uuid = ?", "plan-voucher-user").Scan(ctx, &acctUser)
	require.Nil(t, err)
	require.Equal(t, 90.0, acctUser.CashBalance)

	var bills []database.AccountBill
	err = db.Core.NewSelect().Model(&bills).
		Where("user_uuid = ?", "plan-voucher-user").Scan(ctx, &bills)
	require.Nil(t, err)
	require.Len(t, bills, 3)

	var voucherBill, cashBill, planBill database.AccountBill
	for _, bill := range bills {
		switch {
		case bill.VoucherNo == "VC-TEST-PLAN-TOKEN-1":
			voucherBill = bill
		case bill.PeriodID == 601:
			planBill = bill
		default:
			cashBill = bill
		}
	}
	require.NotZero(t, voucherBill.ID)
	require.NotZero(t, cashBill.ID)
	require.NotZero(t, planBill.ID)

	// the voucher pays 40 of the post-plan 50 fee, i.e. 40% of the full fee
	require.Equal(t, -40.0, voucherBill.Value)
	require.Equal(t, -40.0, voucherBill.VoucherValue)
	require.Equal(t, 0.0, voucherBill.CashValue)
	require.Equal(t, 400.0, voucherBill.Consumption)
	require.Equal(t, 40.0, voucherBill.PromptToken)
	require.Equal(t, 120.0, voucherBill.CompletionToken)
	require.Equal(t, 4.0, voucherBill.Duration)

	// the cash row carries the remaining 10% of the full fee
	require.Equal(t, -10.0, cashBill.Value)
	require.Equal(t, 0.0, cashBill.VoucherValue)
	require.Equal(t, -10.0, cashBill.CashValue)
	require.Equal(t, 100.0, cashBill.Consumption)
	require.Equal(t, 10.0, cashBill.PromptToken)
	require.Equal(t, 30.0, cashBill.CompletionToken)
	require.Equal(t, 1.0, cashBill.Duration)

	// the plan row carries the covered 50%
	require.Equal(t, -50.0, planBill.Value)
	require.Equal(t, 500.0, planBill.Consumption)
	require.Equal(t, 50.0, planBill.PromptToken)
	require.Equal(t, 150.0, planBill.CompletionToken)
	require.Equal(t, 5.0, planBill.Duration)

	// the plan, voucher and cash rows add back to the statement's full detail
	require.Equal(t, 100.0, planBill.PromptToken+voucherBill.PromptToken+cashBill.PromptToken)
	require.Equal(t, 300.0, planBill.CompletionToken+voucherBill.CompletionToken+cashBill.CompletionToken)
	require.Equal(t, 10.0, planBill.Duration+voucherBill.Duration+cashBill.Duration)
	require.Equal(t, 1000.0, planBill.Consumption+voucherBill.Consumption+cashBill.Consumption)

	// the plan, voucher and cash rows must not each count the statement usage
	// on the API key quotas: the token quota counts the full call once, while
	// the fee quota only accumulates the charged (post-plan) fee
	requireAPIKeyQuotaUsage(t, ctx, db, 7, 1000, -50)
}

// seedAPIKeyQuotas creates the default unlimited token and fee quota rows of an
// access token so that deducting a statement with APIKey/TokenID accumulates the
// usage on both dimensions.
func seedAPIKeyQuotas(t *testing.T, ctx context.Context, tx bun.Tx, tokenID int64, apiKey string) {
	t.Helper()
	for _, valueType := range []types.AccountingQuotaValueType{
		types.AccountingQuotaValueTypeToken,
		types.AccountingQuotaValueTypeFee,
	} {
		quota := database.AccountAccessTokenQuota{
			APIKey:    apiKey,
			TokenID:   tokenID,
			QuotaType: types.AccountingQuotaTypeUnlimited,
			ValueType: valueType,
		}
		_, err := tx.NewInsert().Model(&quota).Exec(ctx)
		require.NoError(t, err)
	}
}

// requireAPIKeyQuotaUsage asserts the accumulated usage of an API key's token
// and fee quota rows.
func requireAPIKeyQuotaUsage(t *testing.T, ctx context.Context, db *database.DB, tokenID int64, wantToken, wantFee float64) {
	t.Helper()
	var quotas []database.AccountAccessTokenQuota
	err := db.Core.NewSelect().Model(&quotas).Where("token_id = ?", tokenID).Scan(ctx)
	require.NoError(t, err)
	require.Len(t, quotas, 2)

	for _, quota := range quotas {
		switch quota.ValueType {
		case types.AccountingQuotaValueTypeToken:
			require.Equal(t, wantToken, quota.Usage, "token quota usage")
		case types.AccountingQuotaValueTypeFee:
			require.Equal(t, wantFee, quota.Usage, "fee quota usage")
		}
	}
}
