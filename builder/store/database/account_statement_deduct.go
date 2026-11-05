package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func (as *accountStatementStoreImpl) deductFeeStatement(ctx context.Context, input AccountStatement, extra types.AcctStatementExtra) error {
	err := as.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if input.Value > 0 {
			return fmt.Errorf("deduct fee statement value must be negative or zero")
		}

		coverage, err := validateTokenPlan(ctx, tx, &input)
		if err != nil {
			if errors.Is(err, types.ErrDuplicatedEvent) {
				// the token plan dedup already saw the committed statement:
				// skip the whole deduct the same way DeductAccountFee does
				slog.Warn("skip duplicated token plan deduct fee by event uuid", slog.Any("input", input))
				return nil
			}
			return fmt.Errorf("validate ns uuid %s, token plan period id %d, error: %w",
				input.UserUUID, input.TokenPlanPeriodID, err)
		}

		initEventValue := input.Value

		// The voucher bill rows are written inside deductAccountFee, so they
		// must be scaled against the statement's full (pre-plan) usage detail
		// the same way the cash row below is: a statement the plan partially
		// absorbed carries only its non-plan share into the cash and voucher
		// rows.
		changed, err := deductAccountFee(ctx, tx, input, extra.CheckBalance, nonPlanUsageFraction(coverage))
		if err != nil {
			if errors.Is(err, types.ErrDuplicatedEvent) {
				slog.Warn("skip duplicated deduct fee by event uuid", slog.Any("input", input))
				return nil
			}
			if errors.Is(err, types.ErrDuplicatedAcademyOrder) {
				slog.Warn("skip duplicated academy order", slog.Any("input", input))
				return nil
			}
			return fmt.Errorf("deduct account fee, error: %w", err)
		}

		// The statement insert is confirmed non-duplicate under the
		// account_users row lock held by DeductAccountFee, so mirroring the
		// covered usage from here records it exactly once — including periods
		// without a model quota row, where validateTokenPlan has no row to
		// lock and performs no write of its own. That ordering is the caller's
		// contract and cannot be checked inside the mirror; whether the mirror
		// applies to this statement at all is decided there.
		if err := recordTokenPlanKeyQuotaUsage(ctx, tx, input, coverage); err != nil {
			return fmt.Errorf("record token plan key quota usage, error: %w", err)
		}

		calcValue, consumption := calcValueAndConsumption(initEventValue, input, changed, coverage)
		cashInput, planInput := splitUsageDetailByCoverage(input, &coverage)

		// The voucher bill rows written inside deductAccountFee already carry
		// their share of the non-plan usage detail; scale the cash row down to
		// its own fee share so the bill rows of one statement add back to the
		// statement's full usage detail instead of duplicating it.
		cashShare := 1.0
		if initEventValue != 0 {
			cashShare = math.Abs(calcValue / initEventValue)
		}
		scaleUsageDetail(&cashInput, cashShare)

		err = updateFeeBill(ctx, tx, cashInput, BillValues{
			TotalValue:   calcValue,
			VoucherValue: 0,
			CashValue:    changed.Cash,
			Consumption:  consumption,
		})
		if err != nil {
			return fmt.Errorf("update account bill for consume, error: %w", err)
		}

		// update token plan coverage for fee bill
		err = updateFeeBillForTokenPlanCoverage(ctx, tx, planInput, coverage)
		if err != nil {
			return fmt.Errorf("update token plan coverage, error: %w", err)
		}

		// The statement's fee may have been split into several money rows above
		// (one per voucher plus the cash row); the API key quotas must count the
		// statement's usage once, on the un-split detail.
		recordAPIKeyUsage(ctx, tx, input)

		// update hardware computing resource subscription usage
		err = updateSubscription(ctx, tx, input)
		if err != nil {
			return fmt.Errorf("update subscription, error: %w", err)
		}

		return nil
	})

	return err
}

func calcValueAndConsumption(initEventValue float64, input AccountStatement,
	changed ChangedValue, coverage tokenPlanCoverage) (float64, float64) {
	// subtract the voucher value from the initial value to get the cash value
	calcValue := initEventValue - changed.Voucher

	// The plan-absorbed shares come from the coverage: non-zero only when
	// the plan actually deducted this statement. A partial coverage leaves
	// a non-zero cash fee (initEventValue) alongside a non-zero covered
	// fee; a full coverage zeroes the fee and, per the zero-value fee
	// semantics, produces no bill row at all.
	uncoveredConsumption := input.Consumption - coverage.coveredConsumption

	actualConsumption := 0.0
	if initEventValue != 0 {
		actualConsumption = math.Abs(calcValue/initEventValue) * uncoveredConsumption
	} else {
		actualConsumption = uncoveredConsumption
	}

	return calcValue, actualConsumption
}

// scaleUsageDetail multiplies the per-call usage detail of one statement copy
// by the fee share the bill row built from it carries: the bill rows of one
// statement must add back to the statement's full usage detail instead of
// duplicating it (see splitUsageDetailByCoverage for the plan/cash split).
func scaleUsageDetail(input *AccountStatement, fraction float64) {
	input.PromptToken *= fraction
	input.PromptCachedToken *= fraction
	input.CompletionToken *= fraction
	input.ReasoningToken *= fraction
	input.Duration *= fraction
}

// nonPlanUsageFraction returns the share of a statement's full usage detail the
// cash and voucher bill rows carry together: the whole detail when the plan
// never resolved coverage for the statement, and the plan's cash charge ratio
// otherwise. The plan row carries the remainder (see
// splitUsageDetailByCoverage), so scaling every non-plan row by this fraction
// keeps the plan, cash and voucher rows of one statement adding back to the
// statement's full detail.
func nonPlanUsageFraction(coverage tokenPlanCoverage) float64 {
	if !coverage.applies {
		return 1.0
	}
	return coverage.chargeRatio
}

// splitUsageDetailByCoverage returns the cash-billed and plan-billed copies of
// one statement, each carrying its share of the per-call usage detail: the four
// token columns and the duration. Both shares follow the coverage's charge
// ratio, so the two bill rows add back to the statement's full detail instead of
// duplicating or dropping any of it.
//
// The duration is split by the same ratio rather than kept whole on the cash row.
// It is a per-call scalar like the token counts, and a partially covered
// statement writes two bill rows: leaving it on both would bill one call's
// duration twice, while clearing it on the plan copy would lose it entirely for
// a fully covered statement, which has no cash row at all because its fee is
// zero.
func splitUsageDetailByCoverage(input AccountStatement, coverage *tokenPlanCoverage) (AccountStatement, AccountStatement) {
	cashCovered := input
	tokenPlanCovered := input

	// A statement the plan never resolved coverage for — a non-serverless
	// scene, no resolved period or api key, no model id in the gateway
	// resource id, or a model outside the period's quotas — has no plan share
	// to bill: its whole detail stays on the cash row. chargeRatio alone cannot
	// express that, because its zero value is what a fully covered statement
	// carries (see tokenPlanCoverage.applies).
	cashFraction := nonPlanUsageFraction(*coverage)
	planFraction := 1 - cashFraction

	scaleUsageDetail(&cashCovered, cashFraction)
	scaleUsageDetail(&tokenPlanCovered, planFraction)

	coverage.coveredCostAmount = input.CostAmount * planFraction

	return cashCovered, tokenPlanCovered
}

// DeductAccountFee deducts one consumption statement from vouchers, the cash
// balance and the bonus balance. The caller owns the statement's fee as-is: a
// caller that already applied token plan coverage to input.Value must call
// deductAccountFee with the plan's non-plan usage fraction so the voucher bill
// rows carry their share of the statement's full usage detail.
func DeductAccountFee(ctx context.Context, tx bun.Tx, input AccountStatement, checkBalance bool) (ChangedValue, error) {
	return deductAccountFee(ctx, tx, input, checkBalance, 1.0)
}

// deductAccountFee is DeductAccountFee with the usage-detail share the voucher
// bill rows must carry: usageFraction is the non-plan share of the statement's
// full detail (see nonPlanUsageFraction), which maps the voucher's share of the
// post-plan fee back onto the full detail the cash and plan rows are split from.
func deductAccountFee(ctx context.Context, tx bun.Tx, input AccountStatement, checkBalance bool, usageFraction float64) (ChangedValue, error) {
	var err error
	var acctUser AccountUser
	initEventValue := input.Value
	changedValue := ChangedValue{
		Cash:    0.0,
		Voucher: 0.0,
	}

	// add lock
	err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).For("UPDATE").Scan(ctx, &acctUser)
	if err != nil {
		return changedValue, fmt.Errorf("failed to get account user: %w", err)
	}

	if input.Scene == types.SceneAcademyPurchase {
		// check if user uuid and customer id is duplicated
		var stmt AccountStatement
		err = tx.NewSelect().Model(&stmt).
			Where("user_uuid = ?", input.UserUUID).
			Where("scene = ?", input.Scene).
			Where("customer_id = ?", input.CustomerID).
			Scan(ctx, &stmt)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return changedValue, fmt.Errorf("check duplicated %s event failed, error: %w", input.CustomerID, err)
		}
		if err == nil {
			return changedValue, types.ErrDuplicatedAcademyOrder
		}
	}

	err = CheckDuplicatedEvent(ctx, tx, input)
	if err != nil {
		if errors.Is(err, types.ErrDuplicatedEvent) {
			// return duplicated error for check
			return changedValue, err
		}
		return changedValue, fmt.Errorf("check duplicated event failed, error: %w", err)
	}

	// Check balance if needed
	if checkBalance && input.Value < 0 {
		if acctUser.Balance+acctUser.CashBalance+input.Value < 0 {
			return changedValue, errorx.ErrInsufficientBalance
		}
	}

	remainValue := input.Value
	cashChangePart1 := 0.0
	cashChangePart2 := 0.0

	if remainValue == 0 {
		// only save statement
		err = assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
		if err != nil {
			return changedValue, fmt.Errorf("insert statement, error:%w", err)
		}
		return changedValue, nil
	}

	// 1. check and reduce voucher
	if remainValue < 0 && utils.IsUseVoucher(input.Scene) {
		remainValue, err = deductFeeToVouchers(ctx, tx, input, remainValue, usageFraction)
		if err != nil {
			return changedValue, fmt.Errorf("deduct voucher, error: %w", err)
		}
		changedValue.Voucher = initEventValue - remainValue
	}

	// 2. check and reduce cash
	if remainValue < 0 && acctUser.CashBalance > 0 {
		remainValue, cashChangePart1, err = deductFeeToCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct cash balance, error:%w", err)
		}
	}

	// 3. check and reduce bonus
	if remainValue < 0 && acctUser.Balance > 0 {
		remainValue, err = deductFeeToBalance(ctx, tx, input, remainValue, acctUser.Balance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct balance, error:%w", err)
		}
	}

	// 4. check and reduce all in cash
	if remainValue < 0 {
		err = tx.NewSelect().Model(&acctUser).Where("user_uuid = ?", input.UserUUID).Scan(ctx, &acctUser)
		if err != nil {
			return changedValue, fmt.Errorf("failed to get account user: %w", err)
		}

		_, cashChangePart2, err = deductFeeAllInCashBalance(ctx, tx, input, remainValue, acctUser.CashBalance)
		if err != nil {
			return changedValue, fmt.Errorf("deduct all in balance, error:%w", err)
		}
	}
	changedValue.Cash = cashChangePart1 + cashChangePart2
	return changedValue, nil
}

func deductFeeToBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, balanceValue float64) (float64, error) {
	if balanceValue <= 0 {
		return 0, fmt.Errorf("balance is not enough")
	}
	statementValue := 0.0
	remainValue := balanceValue + feeValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - balanceValue
	}

	input.BalanceValuePrev = balanceValue
	input.Value = statementValue
	input.BalanceValue = balanceValue + statementValue
	input.BalanceType = types.ChargeBalance

	runSql := "update account_users set balance=balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, fmt.Errorf("update balance, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, fmt.Errorf("insert statement for deduct balance, error:%w", err)
	}
	return remainValue, nil
}

func deductFeeToCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, float64, error) {
	if cashBalanceValue <= 0 {
		return 0, 0, fmt.Errorf("cash balance is not enough")
	}
	statementValue := 0.0
	remainValue := cashBalanceValue + feeValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - cashBalanceValue
	}

	input.BalanceValuePrev = cashBalanceValue
	input.Value = statementValue
	input.BalanceValue = cashBalanceValue + statementValue
	input.BalanceType = types.ChargeCashBalance

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, 0, fmt.Errorf("update cash balance, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, 0, fmt.Errorf("insert statement for deduct cash balance, error:%w", err)
	}
	return remainValue, statementValue, nil
}

func deductFeeAllInCashBalance(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, cashBalanceValue float64) (float64, float64, error) {
	remainValue := cashBalanceValue + feeValue

	input.BalanceValuePrev = cashBalanceValue
	input.Value = feeValue
	input.BalanceValue = remainValue
	input.BalanceType = types.ChargeCashBalance

	runSql := "update account_users set cash_balance=cash_balance + ? where user_uuid=?"
	if err := assertAffectedOneRow(tx.Exec(runSql, input.Value, input.UserUUID)); err != nil {
		return 0, 0, fmt.Errorf("update balance for allin, error:%w", err)
	}

	if err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx)); err != nil {
		return 0, 0, fmt.Errorf("insert statement for deduct allin balance, error:%w", err)
	}

	return remainValue, feeValue, nil
}

// tokenPlanCoverage carries the outcome of validateTokenPlan: the covered
// (free) usage of one consumption statement, the plan-absorbed fee and
// consumption shares for the bill split, and the model quota row it was
// resolved against. The caller mirrors the covered usage onto the per-api-key
// quota row once DeductAccountFee has confirmed the statement insert is not a
// duplicate event, so a concurrent redelivery never records the key usage
// twice.
type tokenPlanCoverage struct {
	quota AccountTokenPlanPeriodModelQuota
	// modelID is the quota key resolved from the statement's gateway resource
	// id (falling back to CustomerID for legacy statements without one).
	modelID string
	// applies reports whether the token plan resolved coverage for this
	// statement at all, and gates both effects derived from the coverage: the
	// per-api-key usage mirror (read inside recordTokenPlanKeyQuotaUsage) and
	// the bill split (read inside splitUsageDetailByCoverage). A zero
	// chargeRatio means "the plan absorbed the whole statement", which is
	// indistinguishable from the zero value a statement the plan never applied
	// to carries, so the split reads this flag to tell the two apart: an
	// unresolved statement mirrors no key usage and keeps its entire usage
	// detail on the cash row instead of losing it to a plan row that is never
	// written.
	applies             bool
	chargeRatio         float64
	coveredToken        int64
	coveredRequestCount int64
	// plan-absorbed fee and consumption share of the statement: the bill
	// split books them on a period-attributed plan row while the cash row
	// carries the remainder; coveredConsumption is the coveredToken amount
	// itself, so the plan bill row matches the recorded quota usage
	coveredFee         float64
	coveredConsumption float64
	coveredCostAmount  float64
}

// validateTokenPlan applies the token plan in-plan quota to one consumption
// statement: it records the model's quota usage for the statement's resolved
// token plan period and scales the charged fee down to the part the plan does
// not cover.
//
// Only the serverless model scenes whose statement carries a resolved token
// plan period are touched. The statement ResourceID carries the gateway
// resource id; its model segment is matched against the period's per-model
// quota row. Statements without a gateway resource id fall back to CustomerID
// for backward compatibility. A period whose instance defines model quotas but
// not for this model leaves the statement fully pay-as-you-go; a period without
// any model quota is unrestricted: it covers every call inside it with a zero
// charge and still records the per-api-key usage.
//
// The consumed quota dimension follows the quota row's task type (see
// IsTokenMetred). A non-positive allowance means the dimension is
// unlimited within the plan: the statement is fully covered. Otherwise:
//   - the remaining allowance covers the whole consumption: charge zero;
//   - it covers only part of a token-metred consumption: charge the uncovered
//     share of the fee (the request-count dimension is binary: any remaining
//     free request covers the whole fee);
//   - it is exhausted: charge the full pay-as-you-go fee.
//
// Quota usage records the covered (free) portion only, and an exhausted
// quota adds no usage at all, not even the request count. A covered
// token-metred statement adds its covered tokens plus one request count, and
// the pay-as-you-go overage never consumes plan quota, so UsedTokens stays
// within a positive allowance. A covered request-count-metred statement adds
// one request count plus its consumption as token usage: the count dimension
// decides coverage by the request count alone, but still records the token
// consumption.
//
// The covered usage of a statement carrying a resolved api key id is
// mirrored onto the per-api-key quota row of the period and model by the
// caller, upserted through its unique index; a missing row is created
// inheriting the task type and the allowances of the model quota row (see
// recordTokenPlanKeyQuotaUsage). The mirror runs only after DeductAccountFee
// has confirmed the statement insert is not a duplicate event. The key-level
// quota is an access-side limit and never affects the charge split.
//
// When a model quota row exists it is locked FOR UPDATE, serializing
// concurrent statements of the same period and model. The duplicated-event
// check runs after that lock is acquired, not before: concurrent
// redeliveries of the same event contend on the same quota row, so by the
// time the later transaction holds the lock the earlier one has committed and
// its statement is visible — the quota is consumed exactly once. A pre-lock
// check would leave a race window where a redelivery passes the check while
// the first transaction is still in flight, then consumes the quota a second
// time: DeductAccountFee skips the duplicate statement insert afterwards, but
// this transaction would still commit the quota increment. A period without
// any model quota row has no row to lock and nothing to consume: this function
// performs no write at all on that path, so an unserialized redelivery that
// passes the duplicated-event check stays harmless — the exactly-once
// guarantee of the per-api-key mirror comes from the caller running it after
// DeductAccountFee's post-lock dedup.
func validateTokenPlan(ctx context.Context, tx bun.Tx, input *AccountStatement) (tokenPlanCoverage, error) {
	var coverage tokenPlanCoverage
	if !utils.IsGetTokenID(input.Scene) || !tokenPlanRowApplies(*input) {
		// not a token-plan scene, or no token plan period resolved
		return coverage, nil
	}

	modelID := types.ModelIDFromResourceID(input.ResourceID, false)
	if len(modelID) < 1 {
		// Legacy statements without a gateway resource id keep the previous
		// key, which already carries the model id for external models.
		modelID = input.CustomerID
	}
	if len(modelID) < 1 {
		// without a model id there is nothing to match the quota against
		return coverage, nil
	}
	coverage.modelID = modelID

	var (
		err             error
		quota           AccountTokenPlanPeriodModelQuota
		anyQuotaDefined bool
	)

	quotaErr := tx.NewSelect().Model(&quota).
		Where("period_id = ?", input.TokenPlanPeriodID).
		Where("model_id = ?", modelID).
		OrderExpr("id").Limit(1).
		For("UPDATE").
		Scan(ctx)
	if quotaErr != nil && !errors.Is(quotaErr, sql.ErrNoRows) {
		return coverage, quotaErr
	}

	anyQuotaDefined = quotaErr == nil

	if errors.Is(quotaErr, sql.ErrNoRows) {
		// A period whose instance defines no model quota at all is
		// unrestricted: every call inside the period is covered by the plan
		// and charges zero, with the per-key usage still recorded. A period
		// that defines quotas but not for this model leaves the statement
		// fully pay-as-you-go.
		anyQuotaDefined, err = tx.NewSelect().Model(&AccountTokenPlanPeriodModelQuota{}).
			Where("period_id = ?", input.TokenPlanPeriodID).
			Exists(ctx)
		if err != nil {
			return coverage, err
		}
	}

	// Dedup before any mutation. When a model quota row was found, its
	// FOR UPDATE lock serializes concurrent redeliveries of the same event,
	// so the earlier transaction's statement is committed and visible by now.
	if err := CheckDuplicatedEvent(ctx, tx, *input); err != nil {
		if errors.Is(err, types.ErrDuplicatedEvent) {
			// DeductAccountFee skips the statement insert the same way
			return coverage, err
		}
		return coverage, fmt.Errorf("check duplicated token plan event failed, error: %w", err)
	}

	if anyQuotaDefined {
		if quotaErr == nil {
			// quota defined: check the quota usage
			err = checkAndRecordTokenPlanQuotaUsage(ctx, tx, input, &quota, &coverage)
			if err != nil {
				return coverage, err
			}
		} else {
			// the model id matches no defined quota: fully pay-as-you-go,
			// nothing recorded
			return coverage, nil
		}
	} else {
		// valid token plan without quota defined: unlimited allowance, fully
		// covered; the whole fee and consumption count as the covered shares
		coverage.coveredFee = input.Value
		coverage.chargeRatio = 0.0
		input.Value = 0.0
		coverage.coveredToken = max(int64(input.Consumption), 0)
		coverage.coveredConsumption = float64(coverage.coveredToken)
		coverage.coveredRequestCount = 1
	}

	// the plan resolved this statement, whether it absorbed all of it, part of
	// it or nothing at all: the charge split is driven by chargeRatio, and the
	// caller mirrors the covered usage onto the per-api-key quota of the same
	// period and model once the statement insert is confirmed
	coverage.applies = true
	coverage.quota = quota

	if coverage.chargeRatio < 0 || coverage.chargeRatio > 1 {
		return coverage, fmt.Errorf("chargeRatio must be between 0 and 1")
	}
	return coverage, nil
}

// checkAndRecordTokenPlanQuotaUsage splits the statement fee between the
// period's free allowance and pay-as-you-go by mutating input.Value, then
// records the covered (free) consumption on the model quota row: the
// pay-as-you-go overage never consumes plan quota, so UsedTokens stays within
// a positive allowance, and an exhausted quota adds no usage at all, not even
// the request count. A non-positive allowance means the dimension is unlimited
// within the plan — the statement is fully covered and recorded in full.
//
// The consumed dimension follows the quota row's task type (see
// utils.IsTokenMetred). A token-metred statement records its covered tokens
// plus one request count; a request-count-metred statement decides coverage
// by the request count alone (binary: any remaining free request covers the
// whole fee) and records one request count plus its consumption as token
// usage.
//
// The caller must hold the row's FOR UPDATE lock and have passed the
// duplicated-event check. On success it fills the coverage with the recorded
// covered token amount and request count — for the caller to mirror onto the
// per-api-key quota — plus the plan-absorbed fee and consumption shares for
// the bill split.
func checkAndRecordTokenPlanQuotaUsage(ctx context.Context, tx bun.Tx,
	input *AccountStatement, quota *AccountTokenPlanPeriodModelQuota,
	coverage *tokenPlanCoverage) error {

	coveredToken := int64(0)
	coveredRequestCount := int64(1)
	consumed := max(int64(input.Consumption), 0)
	chargeRatio := 0.0 // default: unlimited allowance, fully covered

	if types.IsTokenMetred(quota.TaskType) {
		// token quota
		if quota.QuotaTokens > 0 {
			freeRemain := quota.QuotaTokens - quota.UsedTokens
			switch {
			case freeRemain <= 0:
				// quota exhausted: full pay-as-you-go, nothing covered
				chargeRatio = 1.0
				coveredRequestCount = int64(0)
			case freeRemain < consumed:
				// partial coverage: charge the uncovered share and record the
				// covered one
				chargeRatio = float64(consumed-freeRemain) / float64(consumed)
				coveredToken = freeRemain
			default:
				// quota covers the whole consumption: charge zero
				coveredToken = consumed
			}
		} else {
			// non-positive allowance: unlimited usage, fully covered
			coveredToken = consumed
		}
	} else {
		// request count quota: binary — any remaining free request covers the
		// whole fee; a non-positive allowance is unlimited. Coverage is
		// decided by the request count alone, while the statement's
		// consumption is still recorded as token usage.
		if quota.QuotaRequestCount > 0 && quota.QuotaRequestCount-quota.UsedRequestCount <= 0 {
			// quota exhausted: full pay-as-you-go, nothing covered
			chargeRatio = 1.0
			coveredRequestCount = int64(0)
		} else {
			coveredToken = consumed
		}
	}

	preValue := input.Value
	if chargeRatio >= 0 && chargeRatio < 1 {
		input.Value = input.Value * chargeRatio
	}

	quota.UsedTokens = quota.UsedTokens + coveredToken
	quota.UsedRequestCount = quota.UsedRequestCount + coveredRequestCount
	quota.LatestUsedAt = time.Now().Unix()

	res, err := tx.NewUpdate().Model(quota).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("record token plan model quota usage, quota id %d, error: %w", quota.ID, err)
	}

	coverage.coveredToken = coveredToken
	coverage.coveredRequestCount = coveredRequestCount
	// plan-absorbed fee for the bill split, derived from the actual fee
	// mutation so the cash and plan bill rows sum back to the full fee
	// exactly; the plan-covered consumption is the covered token amount
	// itself, matching the quota usage recorded above, while the cash row
	// carries the consumption remainder
	coverage.coveredFee = preValue - input.Value
	coverage.coveredConsumption = float64(coveredToken)
	coverage.chargeRatio = chargeRatio

	return nil
}

// recordTokenPlanKeyQuotaUsage mirrors one statement's covered (free) usage
// onto the per-API-key quota row of the same token plan period and model: the
// pay-as-you-go overage is not recorded, matching the model quota row. The key
// quota is an access-side limit allocated per api key; the statement deduct
// only records its usage — the charge split stays driven by the period-level
// model quota.
//
// The row is upserted through the unique index on (period_id, token_id,
// model_id), using the quota model id resolved from the statement's resource
// id (see validateTokenPlan): a missing row is created inheriting the task type
// and the allowances of the period's model quota row, and an existing row
// accumulates the covered tokens and the covered request count. A period that
// defines no model quota at all passes the zero quota row, so the created key
// row gets an empty task type and zero allowances. The single upsert statement
// is atomic, so concurrent statements of the same key cannot race into duplicate
// rows or lost updates.
//
// The caller runs this only after DeductAccountFee has confirmed, under the
// account_users row lock, that the statement insert is not a duplicate event,
// so a redelivered event is never recorded here twice — including periods
// without a model quota row, where no quota row lock serializes deliveries. That
// ordering stays a caller contract; the applicability check below is what this
// function owns.
//
// It is a no-op when the token plan resolved no coverage for the statement, and
// when the statement carries no api key id: the upsert key (period_id, token_id,
// model_id) needs a resolved key to attribute the covered usage to, and a
// token_id of 0 would collapse every non-key request of a period and model into
// one row. Guarding here rather than at the call site keeps that key precondition
// next to the write that depends on it.
func recordTokenPlanKeyQuotaUsage(ctx context.Context, tx bun.Tx,
	input AccountStatement, coverage tokenPlanCoverage) error {

	if !coverage.applies {
		return nil
	}

	coveredTokens := coverage.coveredToken
	coveredRequestCount := coverage.coveredRequestCount

	keyQuota := &AccountTokenPlanPeriodKeyQuota{
		PeriodID:          input.TokenPlanPeriodID,
		TokenID:           input.TokenID,
		ModelID:           coverage.modelID,
		TaskType:          coverage.quota.TaskType,
		QuotaTokens:       coverage.quota.QuotaTokens,
		QuotaRequestCount: coverage.quota.QuotaRequestCount,
		UsedTokens:        coveredTokens,
		UsedRequestCount:  coveredRequestCount,
		LatestUsedAt:      time.Now().Unix(),
	}

	res, err := tx.NewInsert().Model(keyQuota).
		On("CONFLICT (period_id, token_id, model_id) DO UPDATE").
		Set("used_tokens = account_token_plan_period_key_quota.used_tokens + ?", coveredTokens).
		Set("used_request_count = account_token_plan_period_key_quota.used_request_count + ?", coveredRequestCount).
		Set("latest_used_at = ?", keyQuota.LatestUsedAt).
		Set("updated_at = current_timestamp").
		Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("upsert token plan key quota usage, period id %d, token id %d, model id %s, error: %w",
			input.TokenPlanPeriodID, input.TokenID, coverage.modelID, err)
	}
	return nil
}

func tokenPlanRowApplies(input AccountStatement) bool {
	return input.TokenID > 0 && input.TokenPlanPeriodID > 0
}

// updateFeeBillForTokenPlanCoverage bills the share of the fee, consumption and
// usage detail that the token plan absorbed on a partially covered statement
// (case 6) as a second row attributed to the resolved period. The cash row
// written by updateFeeBill carries the uncovered share; together the two rows
// add back to the statement's full pre-split fee, consumption, token detail and
// duration (see splitUsageDetailByCoverage).
//
// The caller passes the plan-covered copy of the statement, so its token columns
// and its duration already hold the covered share and nothing is adjusted here.
// The row is written through updateFeeBillRow — not updateFeeBill — because the
// API key usage side effect already ran on the cash-row write and must not
// repeat.
//
// tokenPlanRowApplies and coverage.applies decide whether such a row exists at
// all: an unresolved statement or one without a resolved period or API key
// (no per-key attribution) writes no plan row, and the zero-value fee semantics
// in updateFeeBillRow write nothing for a fully covered one.
func updateFeeBillForTokenPlanCoverage(ctx context.Context, tx bun.Tx, input AccountStatement, coverage tokenPlanCoverage) error {
	if !tokenPlanRowApplies(input) {
		return nil
	}

	err := updateFeeBillRow(ctx, tx, input, BillValues{
		TotalValue:   coverage.coveredFee,
		VoucherValue: 0,
		CashValue:    0,
		Consumption:  coverage.coveredConsumption,
	})
	if err != nil {
		return fmt.Errorf("update account bill for token plan covered share, error: %w", err)
	}

	// update token plan period cost amount
	if err := accumulatePlanPeriodCost(ctx, tx, input, coverage); err != nil {
		return fmt.Errorf("accumulate token plan period cost, error: %w", err)
	}
	return nil
}

func accumulatePlanPeriodCost(ctx context.Context, tx bun.Tx, input AccountStatement, coverage tokenPlanCoverage) error {
	// Match the zero-value fee semantics of updateFeeBillRow: a zero covered
	// cost has nothing to accumulate, so skip the period row update entirely.
	// An exhausted quota reaches here with a zero covered cost; updating the
	// row anyway would take its write lock and emit WAL for every statement,
	// serializing unrelated model usage on the same period.
	if !coverage.applies || coverage.coveredCostAmount == 0 {
		return nil
	}

	res, err := tx.NewUpdate().Table("account_token_plan_instance_periods").
		Set("cost_amount = cost_amount + ?", coverage.coveredCostAmount).
		Set("updated_at = current_timestamp").
		Where("id = ?", input.TokenPlanPeriodID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update account_token_plan_instance_periods id %d, error: %w", input.TokenPlanPeriodID, err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("period cost not accumulated, token_plan_period_id %d, error: %w", input.TokenPlanPeriodID, err)
	}
	return nil
}
