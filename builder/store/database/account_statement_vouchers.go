package database

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func deductFeeToVouchers(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue, usageFraction float64) (float64, error) {
	remainValue := feeValue

	err := updateVouchersStatus(ctx, tx)
	if err != nil {
		return remainValue, fmt.Errorf("refresh vouchers status, error: %w", err)
	}

	targetUsage, err := resolveVoucherUsage(ctx, tx, input)
	if err != nil {
		return remainValue, fmt.Errorf("resolve voucher usage for resource id %s, error: %w", input.ResourceID, err)
	}
	if targetUsage == nil {
		// The statement carries no signature the voucher rules can match
		// against (a token scene whose gateway resource id and customer id
		// resolve no model): skip voucher deduction and let the cash steps
		// cover the fee.
		return remainValue, nil
	}

	var vouchers []AccountVoucher
	err = tx.NewSelect().Model(&vouchers).
		Where("target_uuid = ?", input.UserUUID).
		Where("status = ?", types.VoucherStatusActive).
		Where("begin_date <= ?", time.Now().In(config.GetGlobalTimeZone())).
		Where("end_date > ?", time.Now().In(config.GetGlobalTimeZone())).
		Where("(total - ABS(used)) > 0").
		Order("end_date ASC", "created_at ASC").
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return remainValue, fmt.Errorf("query active vouchers for target %s: %w", input.UserUUID, err)
	}

	if len(vouchers) < 1 {
		return remainValue, nil
	}

	priorityVouchers := applicableVouchers(vouchers, *targetUsage)

	for _, v := range priorityVouchers {
		if remainValue >= 0 {
			break
		}
		remainValue, err = deductFeeToVoucher(ctx, tx, input, remainValue, v, usageFraction)
		if err != nil {
			return remainValue, fmt.Errorf("deduct single voucher id %d error: %w", v.ID, err)
		}
	}

	return remainValue, nil
}

// voucherUsage carries the consumption signature that voucher rules are matched
// against: a token scene bills per model and matches on the model id, while a
// compute scene bills a space resource and matches on its cluster id and xpu
// model.
type voucherUsage struct {
	tokenScene bool
	clusterID  string
	xpuModel   string
	modelID    string
}

// resolveVoucherUsage derives the voucherUsage of one statement. The gateway
// resource id of a token statement carries the model segment and legacy
// statements fall back to the customer id — the same resolution
// validateTokenPlan uses. It returns a nil usage without an error when the
// statement carries no signature at all, which makes the caller skip voucher
// deduction entirely instead of failing the transaction.
func resolveVoucherUsage(ctx context.Context, tx bun.Tx, input AccountStatement) (*voucherUsage, error) {
	if utils.IsGetTokenID(input.Scene) {
		return resolveTokenUsage(input)
	}
	return resolveComputeUsage(ctx, tx, input)
}

func resolveTokenUsage(input AccountStatement) (*voucherUsage, error) {
	usage := &voucherUsage{tokenScene: true}
	usage.modelID = types.ModelIDFromResourceID(input.ResourceID, false)
	if len(usage.modelID) < 1 {
		// legacy statements without a gateway resource id keep the
		// customer id, which already carries the model id
		usage.modelID = input.CustomerID
	}
	if len(usage.modelID) < 1 {
		return nil, nil
	}
	return usage, nil
}

func resolveComputeUsage(ctx context.Context, tx bun.Tx, input AccountStatement) (*voucherUsage, error) {
	usage := &voucherUsage{tokenScene: false}
	resID, err := strconv.ParseInt(input.ResourceID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad request resource id format %s for voucher, error: %w", input.ResourceID, err)
	}

	var res SpaceResource
	res.ID = resID

	err = tx.NewSelect().Model(&res).WherePK().Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("query resource id %s for voucher, error: %w", input.ResourceID, err)
	}

	usage.clusterID = strings.TrimSpace(res.ClusterID)
	if len(usage.clusterID) < 1 {
		return nil, fmt.Errorf("cluster id is empty for resource id %s for voucher", input.ResourceID)
	}

	reqXpuModel, err := types.GetResXPUMode(res.Resources)
	if err != nil {
		return nil, fmt.Errorf("get resource xpu model for resource id %s for voucher, error: %w", input.ResourceID, err)
	}
	usage.xpuModel = strings.TrimSpace(reqXpuModel)

	return usage, nil
}

const (
	voucherRankSpecific  = 0
	voucherRankUniversal = 1
)

// voucherSortKey ranks one voucher for the deduction order: typeRank puts
// scene-specific voucher types before universal ones, so the broadly usable
// universal voucher is preserved; specificity puts the more restricted rules
// first, counting the include-mode dimensions a matched rule limits.
type voucherSortKey struct {
	typeRank    int
	specificity int
}

// applicableVouchers filters the vouchers that may pay this statement and
// orders them for deduction: scene-specific vouchers before universal ones,
// then the more restricted rules first, then the earliest expiring one, then
// the oldest voucher, with the id as the final deterministic tiebreak.
func applicableVouchers(vouchers []AccountVoucher, usage voucherUsage) []AccountVoucher {
	type rankedVoucher struct {
		voucher AccountVoucher
		key     voucherSortKey
	}

	ranked := make([]rankedVoucher, 0, len(vouchers))
	for _, v := range vouchers {
		key, ok := voucherSortKeyFor(v, usage)
		if !ok {
			continue
		}
		ranked = append(ranked, rankedVoucher{voucher: v, key: key})
	}

	slices.SortStableFunc(ranked, func(a, b rankedVoucher) int {
		return cmp.Or(
			cmp.Compare(a.key.typeRank, b.key.typeRank),
			cmp.Compare(b.key.specificity, a.key.specificity),
			a.voucher.EndDate.Compare(b.voucher.EndDate),
			a.voucher.CreatedAt.Compare(b.voucher.CreatedAt),
			cmp.Compare(a.voucher.ID, b.voucher.ID),
		)
	})

	sorted := make([]AccountVoucher, 0, len(ranked))
	for _, r := range ranked {
		sorted = append(sorted, r.voucher)
	}
	return sorted
}

// voucherSortKeyFor reports whether the voucher may pay this statement and
// returns its deduction rank. Universal vouchers are matched against the rule
// side of their own scene, keeping compute_limits and token_limits orthogonal
// (see universalVoucherSortKey). Compute vouchers apply to compute scenes
// whose rules match the resource's cluster id and xpu model, and are skipped
// when the statement carries no xpu model at all. Token vouchers apply to
// token scenes whose rules match the model. Vouchers of an unknown type apply
// nowhere.
func voucherSortKeyFor(voucher AccountVoucher, usage voucherUsage) (voucherSortKey, bool) {
	if voucher.VoucherType == types.VoucherTypeUniversal {
		return universalVoucherSortKey(voucher, usage)
	}

	// token scene
	if usage.tokenScene {
		return tokenVoucherSortKey(voucher, usage)
	}

	// compute scene
	return computeVoucherSortKey(voucher, usage)
}

// universalVoucherSortKey evaluates only the rule side a universal voucher
// carries for the current scene: a token scene reads token_limits and a
// compute scene reads compute_limits, so the two rule groups stay orthogonal
// and independently effective. A side without a declared restriction is the
// unconditional "true universal" case; a restricted side must match the
// statement, and its specificity ranks restricted universal vouchers ahead of
// unrestricted ones within the universal tier.
func universalVoucherSortKey(voucher AccountVoucher, usage voucherUsage) (voucherSortKey, bool) {
	if usage.tokenScene {
		specificity, ok := tokenRuleSpecificity(voucher.Rules.TokenLimits.Models, usage.modelID)
		if !ok {
			return voucherSortKey{}, false
		}
		return voucherSortKey{typeRank: voucherRankUniversal, specificity: specificity}, true
	}

	specificity, ok := computeRulesSpecificity(voucher.Rules.ComputeLimits, usage)
	if !ok {
		return voucherSortKey{}, false
	}
	return voucherSortKey{typeRank: voucherRankUniversal, specificity: specificity}, true
}

func tokenVoucherSortKey(voucher AccountVoucher, usage voucherUsage) (voucherSortKey, bool) {
	if voucher.VoucherType != types.VoucherTypeToken {
		return voucherSortKey{}, false
	}

	specificity, ok := tokenRuleSpecificity(voucher.Rules.TokenLimits.Models, usage.modelID)
	return voucherSortKey{specificity: specificity}, ok
}

func computeVoucherSortKey(voucher AccountVoucher, usage voucherUsage) (voucherSortKey, bool) {
	if voucher.VoucherType != types.VoucherTypeCompute {
		return voucherSortKey{}, false
	}

	if len(usage.xpuModel) < 1 {
		// without an xpu model the statement cannot be attributed to a
		// compute resource: dedicated compute vouchers are skipped and only
		// the universal side evaluation still applies
		return voucherSortKey{}, false
	}

	specificity, ok := computeRulesSpecificity(voucher.Rules.ComputeLimits, usage)
	return voucherSortKey{specificity: specificity}, ok
}

// tokenRuleSpecificity evaluates one model restriction: an include-mode rule
// matches only the listed models and is more specific than an any-mode rule;
// every other mode (an unset one included) matches nothing.
func tokenRuleSpecificity(rule types.RuleItem, modelID string) (int, bool) {
	if !ruleItemMatches(rule, modelID) {
		return 0, false
	}
	if rule.Mode == types.RuleItemTypeInclude {
		return 1, true
	}
	return 0, true
}

// computeRulesSpecificity evaluates the compute restrictions of one voucher:
// the rules are alternatives and the most specific matched rule decides the
// rank. An empty rule list declares no restriction at all and matches as
// unrestricted. A declared include-mode restriction only matches a statement
// carrying the dimension it pins, so a statement without an xpu model still
// matches the unrestricted and cluster-only rules but never an xpu-model
// restriction.
func computeRulesSpecificity(rules []types.VoucherComputeRule, usage voucherUsage) (int, bool) {
	if len(rules) == 0 {
		return 0, true
	}

	matched := false
	specificity := 0
	for _, rule := range rules {
		if !ruleItemMatches(rule.ClusterIDs, usage.clusterID) ||
			!ruleItemMatches(rule.XPUModels, usage.xpuModel) {
			continue
		}
		matched = true
		specificity = max(specificity, computeRuleSpecificity(rule))
	}
	if !matched {
		return 0, false
	}
	return specificity, true
}

// computeRuleSpecificity counts the include-mode dimensions of one compute
// rule: a rule naming both the cluster and the xpu model is more specific
// than one limiting a single dimension, which still beats an unrestricted
// rule. A rule reaching the caller has matched every include-mode dimension
// it declares, so the count is the number of dimensions it actually pins.
func computeRuleSpecificity(rule types.VoucherComputeRule) int {
	specificity := 0
	if rule.ClusterIDs.Mode == types.RuleItemTypeInclude {
		specificity++
	}
	if rule.XPUModels.Mode == types.RuleItemTypeInclude {
		specificity++
	}
	return specificity
}

// VoucherAppliesToComputeFilter reports whether the voucher can pay a compute
// resource in the requested cluster and xpu model. It evaluates the voucher's
// compute rules with the same helper the deduct path uses, so a dashboard or
// the deployment balance check reading it cannot drift from the deduction
// semantics: an empty rule list and any-mode items match unconditionally, and
// only compute and universal vouchers qualify at all.
//
// The filter may omit a dimension. An empty cluster id or xpu model is still
// admitted by an any-mode or empty restriction, while an include-mode
// restriction on the omitted dimension cannot be confirmed and does not match.
func VoucherAppliesToComputeFilter(voucher AccountVoucher, clusterID, xpuModel string) bool {
	switch voucher.VoucherType {
	case types.VoucherTypeCompute, types.VoucherTypeUniversal:
	default:
		return false
	}

	_, ok := computeRulesSpecificity(voucher.Rules.ComputeLimits, voucherUsage{
		clusterID: clusterID,
		xpuModel:  xpuModel,
	})
	return ok
}

// ruleItemMatches reports whether the request value satisfies one rule item:
// include mode requires the value to be listed, any mode does not restrict
// the dimension.
func ruleItemMatches(item types.RuleItem, value string) bool {
	if item.Mode == types.RuleItemTypeAny {
		return true
	}

	if item.Mode == types.RuleItemTypeInclude {
		return slices.Contains(item.Values, value)
	}

	return false
}

func deductFeeToVoucher(ctx context.Context, tx bun.Tx, input AccountStatement, feeValue float64, voucher AccountVoucher, usageFraction float64) (float64, error) {
	eventValue := input.Value
	remainValue := feeValue
	if remainValue >= 0 {
		return remainValue, nil
	}

	available := voucher.Total - math.Abs(voucher.Used)
	if available <= 0 {
		return remainValue, nil
	}

	statementValue := 0.0
	remainValue = available + remainValue
	if remainValue >= 0 {
		remainValue = 0.0
		statementValue = feeValue
	} else {
		statementValue = 0 - available
	}

	input.BalanceValuePrev = available
	input.Value = statementValue
	input.BalanceValue = available + statementValue
	input.BalanceType = types.ChargeVoucher
	input.VoucherNo = voucher.VoucherNo

	err := assertAffectedOneRow(tx.NewInsert().Model(&input).Exec(ctx))
	if err != nil {
		return 0, fmt.Errorf("insert statement for deduct voucher id %d error:%w", voucher.ID, err)
	}

	_, err = tx.NewUpdate().Model(&AccountVoucher{}).
		Set("used = used + ?", statementValue).Set("updated_at = ?", time.Now()).
		Where("id = ?", voucher.ID).Exec(ctx)
	if err != nil {
		return remainValue, fmt.Errorf("update voucher id %d used amount error: %w", voucher.ID, err)
	}

	consumption := 0.0
	voucherShare := 0.0
	if eventValue != 0 {
		// eventValue is the fee left after token plan coverage, so the
		// statement's non-plan usage fraction maps the voucher's share of that
		// fee back onto the full usage detail the cash and plan rows are split
		// from: without it a partially plan-covered statement would bill its
		// usage detail twice.
		voucherShare = math.Abs(statementValue/eventValue) * usageFraction
		consumption = voucherShare * input.Consumption
	}

	// The voucher bill row carries its fee share of the per-call usage detail:
	// the voucher rows plus the cash row must add back to the statement's full
	// token counts and duration instead of duplicating them. The scaling runs
	// after the statement insert above, so the ledger rows keep the statement's
	// original detail.
	scaleUsageDetail(&input, voucherShare)

	err = updateFeeBill(ctx, tx, input, BillValues{
		TotalValue:   statementValue,
		VoucherValue: statementValue,
		CashValue:    0.0,
		Consumption:  consumption,
	})
	if err != nil {
		return remainValue, fmt.Errorf("update voucher bill for voucher %s error: %w", input.VoucherNo, err)
	}

	return remainValue, nil
}
