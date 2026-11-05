package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountTokenPlanStoreImpl struct {
	db *DB
}

type AccountTokenPlanStore interface {
	Create(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error)
	GetByID(ctx context.Context, id int64) (*AccountTokenPlan, error)
	GetByPlanCode(ctx context.Context, planCode string) (*AccountTokenPlan, error)
	GetByPlanName(ctx context.Context, planName string) (*AccountTokenPlan, error)
	Update(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, req types.AcctTokenPlanListReq) ([]AccountTokenPlan, int, error)
	Summary(ctx context.Context) ([]types.AcctTokenPlanSummaryItem, error)
	SummaryNsBound(ctx context.Context) ([]types.AcctTokenPlanNsBoundCount, error)
	SummaryActiveSubscriptions(ctx context.Context) ([]types.AcctTokenPlanActiveSubCount, error)
	Logs(ctx context.Context, input AccountTokenPlanLog) error
	ListPublishedForNS(ctx context.Context, req types.AcctTokenPlanNSListReq) ([]AccountTokenPlan, int, error)
	GetActiveInstanceByNsUUID(ctx context.Context, req types.AcctTokenPlanNSGetReq) ([]AccountTokenPlanInstance, error)
	ListActiveTokenPlanNamespaces(ctx context.Context) ([]string, error)
	ListInstancesByNsUUID(ctx context.Context, req types.AcctTokenPlanHistoryReq) ([]AccountTokenPlanInstance, int, error)
	// ListInstances returns one page of token plan subscription instances for
	// the admin listing, newest instance first, together with the total count
	// of every instance matching the same filters. Every filter is optional.
	ListInstances(ctx context.Context, req types.AcctTokenPlanInstanceListReq) ([]AccountTokenPlanInstance, types.AcctTokenPlanInstanceTotalValue, error)
	// ListPeriodsForNS returns the paginated billing periods of a namespace,
	// newest period first, each paired with the plan version bound to it. The
	// result also carries the total count and summed amount of every period
	// matching the filter.
	ListPeriodsForNS(ctx context.Context, filter types.AcctTokenPlanPeriodsFilter) (*TokenPlanPeriodListResult, error)
	// ListPeriodsForInstance returns the paginated billing periods of one
	// instance for the admin listing, highest period number first, each paired
	// with the plan version bound to it. The result also carries the total
	// count and summed amount of every period matching the filter.
	ListPeriodsForInstance(ctx context.Context, filter types.AcctTokenPlanInstancePeriodsFilter) (*TokenPlanPeriodListResult, error)
	CreateInstanceWithPeriod(ctx context.Context, req types.AcctTokenPlanPurchaseReq) (*AccountTokenPlanInstance, error)
	UpgradeInstance(ctx context.Context, req types.AcctTokenPlanUpgradeReq) (*AccountTokenPlanInstance, error)
	RecycleNSInstance(ctx context.Context, req types.AcctTokenPlanRecycleReq) error
	CancelInstance(ctx context.Context, req types.AcctTokenPlanCancelReq) (*AccountTokenPlanInstance, error)
	UpdateInstanceOveragePolicy(ctx context.Context, req types.AcctTokenPlanUpdateNSReq) (*AccountTokenPlanInstance, error)
	// GetActiveSubscription returns the namespace's active token plan
	// subscription, or nil when it has none. It is the first hop of the
	// read-only inference quota pre-check and never reconciles plan state.
	GetActiveSubscription(ctx context.Context, nsUUID string) (*TokenPlanActiveSubscription, error)
	// GetLatestPeriodModelQuota returns the latest billing period of a
	// subscription instance together with the per-model quota row of that
	// period, in one read-only query. It returns nil when the instance has no
	// period yet; Quota is nil when the period does not cover the model.
	GetLatestPeriodModelQuota(ctx context.Context, instanceID int64, modelID string) (*TokenPlanPeriodQuota, error)
	// GetPeriodByEvent reconciles the namespace's token plan subscription and
	// returns the legitimate paid billing period whose window covers the
	// metering event time, or nil when no period qualifies.
	GetPeriodByEvent(ctx context.Context, req types.AcctTokenPlanNSEventReq) (*AccountTokenPlanInstancePeriod, error)
}

type InstanceReconcileResult struct {
	NextVal string
	Explain string
	Err     error
}

// TokenPlanActiveSubscription is the first hop of the inference quota
// pre-check: whether a namespace has an active token plan subscription and,
// when it does, which overage policy governs it and which instance the current
// period is read from. Tenants without a subscription, and tenants whose
// overage policy bills overage instead of rejecting, need no further lookup.
type TokenPlanActiveSubscription struct {
	InstanceID    int64
	OveragePolicy types.OveragePolicy
}

// TokenPlanPeriodQuota is the second hop of the inference quota pre-check: the
// latest billing period of a subscription instance and the quota row of one
// model in that period. Quota is nil when the period does not cover the model.
type TokenPlanPeriodQuota struct {
	PeriodID int64
	StartAt  int64
	EndAt    int64
	Quota    *AccountTokenPlanPeriodModelQuota
}

type ReconcileReq struct {
	NsUUID      string
	RequestTime time.Time
	EventUUID   uuid.UUID
	CurrentUser string
	Source      types.RunWay
}

func NewAccountTokenPlanStore() AccountTokenPlanStore {
	return &accountTokenPlanStoreImpl{
		db: defaultDB,
	}
}

func NewAccountTokenPlanStoreWithDB(db *DB) AccountTokenPlanStore {
	return &accountTokenPlanStoreImpl{
		db: db,
	}
}

// tokenPlanNameLockKey namespaces the advisory lock that serializes token plan
// writes sharing a plan name, so it cannot collide with other advisory locks.
func tokenPlanNameLockKey(planName string) string {
	return "account_token_plan_name:" + planName
}

// lockTokenPlanName is the first step of the duplicate-name guard: it takes a
// transaction-scoped advisory lock on the plan name so concurrent creates and
// renames of the same name run one at a time. The lock is released when the
// transaction commits or rolls back.
func lockTokenPlanName(ctx context.Context, tx bun.Tx, planName string) error {
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", tokenPlanNameLockKey(planName)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("acquire token plan name lock of %q, error: %w", planName, err)
	}
	return nil
}

// assertTokenPlanNameAvailable is the second step of the duplicate-name guard:
// with the name lock held, it re-checks the table so a writer that committed
// while this transaction waited is observed. excludeID lets an update keep its
// own name, and soft-deleted plans are ignored by the model's soft-delete scope.
func assertTokenPlanNameAvailable(ctx context.Context, tx bun.Tx, planName string, excludeID int64) error {
	q := tx.NewSelect().Model((*AccountTokenPlan)(nil)).Where("plan_name = ?", planName)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	exists, err := q.Exists(ctx)
	if err != nil {
		return fmt.Errorf("check token plan name %q, error: %w", planName, err)
	}
	if exists {
		return errorx.TokenPlanNameDuplicate(planName)
	}
	return nil
}

// tokenPlanWriteError preserves the duplicate-name error so callers and the API
// layer can match it, and converts every other database error as usual.
func tokenPlanWriteError(err error) error {
	if errors.Is(err, errorx.ErrTokenPlanNameDuplicate) {
		return err
	}
	return errorx.HandleDBError(err, nil)
}

func (a *accountTokenPlanStoreImpl) Create(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error) {
	err := a.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockTokenPlanName(ctx, tx, input.PlanName); err != nil {
			return err
		}
		if err := assertTokenPlanNameAvailable(ctx, tx, input.PlanName, 0); err != nil {
			return err
		}

		res, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, tokenPlanWriteError(err)
	}
	return &input, nil
}

func (a *accountTokenPlanStoreImpl) GetByID(ctx context.Context, id int64) (*AccountTokenPlan, error) {
	var result AccountTokenPlan
	err := a.db.Core.NewSelect().Model(&result).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &result, nil
}

func (a *accountTokenPlanStoreImpl) GetByPlanCode(ctx context.Context, planCode string) (*AccountTokenPlan, error) {
	var result AccountTokenPlan
	err := a.db.Core.NewSelect().Model(&result).Where("plan_code = ?", planCode).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &result, nil
}

func (a *accountTokenPlanStoreImpl) GetByPlanName(ctx context.Context, planName string) (*AccountTokenPlan, error) {
	var result AccountTokenPlan
	err := a.db.Core.NewSelect().Model(&result).Where("plan_name = ?", planName).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &result, nil
}

func (a *accountTokenPlanStoreImpl) Update(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error) {
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var locked AccountTokenPlan
		err := tx.NewSelect().Model(&locked).Where("id = ?", input.ID).For("UPDATE").Scan(ctx)
		if err != nil {
			return err
		}

		// Only a rename can introduce a duplicate name; a no-op update must not
		// conflict with the plan's own current name.
		if input.PlanName != locked.PlanName {
			if err := lockTokenPlanName(ctx, tx, input.PlanName); err != nil {
				return err
			}
			if err := assertTokenPlanNameAvailable(ctx, tx, input.PlanName, input.ID); err != nil {
				return err
			}
		}

		res, err := tx.NewUpdate().Model(&input).WherePK().Returning("*").Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		if input.Status != types.TokenPlanStatusPublished {
			return nil
		}

		var latestPlanVer AccountTokenPlanVersion
		err = tx.NewSelect().Model(&latestPlanVer).Where("plan_id = ?", input.ID).
			Order("version DESC").Limit(1).Scan(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		newLatestPlanVer := AccountTokenPlanVersion{
			AccountTokenPlanCommon: AccountTokenPlanCommon{
				PlanCode:      input.PlanCode,
				PlanName:      input.PlanName,
				Description:   input.Description,
				Details:       input.Details,
				SubjectType:   input.SubjectType,
				PriceAmount:   input.PriceAmount,
				Currency:      input.Currency,
				BillingPeriod: input.BillingPeriod,
				PeriodUnit:    input.PeriodUnit,
				Status:        input.Status,
				CreatedBy:     input.CreatedBy,
				PublishedAt:   input.PublishedAt,
				DisabledAt:    input.DisabledAt,
				Quotas:        input.Quotas,
				NsUUIDs:       input.NsUUIDs,
			},
			PlanID:  input.ID,
			Version: latestPlanVer.Version + 1,
		}

		res, err = tx.NewInsert().Model(&newLatestPlanVer).Returning("*").Exec(ctx, &newLatestPlanVer)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		input.LatestVersionID = newLatestPlanVer.ID
		res, err = tx.NewUpdate().Model(&input).WherePK().Returning("*").Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, tokenPlanWriteError(err)
	}

	return &input, nil
}

func (a *accountTokenPlanStoreImpl) Delete(ctx context.Context, id int64) error {
	res, err := a.db.Core.NewDelete().Model((*AccountTokenPlan)(nil)).Where("id = ?", id).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

func (a *accountTokenPlanStoreImpl) List(ctx context.Context, req types.AcctTokenPlanListReq) ([]AccountTokenPlan, int, error) {
	var result []AccountTokenPlan
	q := a.db.Core.NewSelect().Model(&result)

	if req.Status != nil {
		q = q.Where("status = ?", *req.Status)
	}
	if req.SubjectType != nil {
		q = q.Where("subject_type = ?", req.SubjectType)
	}
	if req.Period != nil {
		q = q.Where("billing_period = ?", *req.Period)
	}
	if req.Search != nil {
		q = q.Where("(strpos(plan_code, ?) > 0 OR strpos(plan_name, ?) > 0)", *req.Search, *req.Search)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count token plans, error: %w", err)
	}

	err = q.Order("created_at DESC").
		Limit(req.Per).
		Offset((req.Page-1)*req.Per).
		Scan(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list token plans, error: %w", err)
	}

	if len(result) > 0 {
		err = a.fillActiveSubscriptionCounts(ctx, result)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to fill active subscription counts, error: %w", err)
		}
	}
	return result, count, nil
}

type tokenPlanActiveSubscriptionCount struct {
	PlanID int64 `bun:"plan_id"`
	Count  int64 `bun:"count"`
}

// fillActiveSubscriptionCounts counts active token plan instances grouped by
// plan id and writes the value into each plan's ActiveSubscriptionCount field.
func (a *accountTokenPlanStoreImpl) fillActiveSubscriptionCounts(ctx context.Context, plans []AccountTokenPlan) error {
	planIDs := make([]int64, 0, len(plans))
	for _, plan := range plans {
		planIDs = append(planIDs, plan.ID)
	}

	var counts []tokenPlanActiveSubscriptionCount
	err := a.db.Core.NewSelect().
		Model((*AccountTokenPlanInstance)(nil)).
		ModelTableExpr("account_token_plan_instances AS i").
		Column("i.plan_id").
		ColumnExpr("COUNT(*) AS count").
		Where("i.status = ?", types.SubscriptionStatusActive).
		Where("i.plan_id IN (?)", bun.In(planIDs)).
		Group("i.plan_id").
		Scan(ctx, &counts)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	countByPlan := make(map[int64]int64, len(counts))
	for _, count := range counts {
		countByPlan[count.PlanID] = count.Count
	}
	for i := range plans {
		plans[i].ActiveSubscriptionCount = countByPlan[plans[i].ID]
	}
	return nil
}

// Summary counts token plans grouped by status and subject type for the admin
// dashboard. Soft deleted plans are excluded by the AccountTokenPlan model.
func (a *accountTokenPlanStoreImpl) Summary(ctx context.Context) ([]types.AcctTokenPlanSummaryItem, error) {
	var result []types.AcctTokenPlanSummaryItem
	err := a.db.Core.NewSelect().
		Model((*AccountTokenPlan)(nil)).
		Column("status").
		Column("subject_type").
		ColumnExpr("COUNT(*) AS count").
		Group("status", "subject_type").
		Order("status ASC", "subject_type ASC").
		Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

// SummaryNsBound counts token plans restricted to specific namespaces
// (ns_uuids is not empty) grouped by status and subject type for the admin
// dashboard.
func (a *accountTokenPlanStoreImpl) SummaryNsBound(ctx context.Context) ([]types.AcctTokenPlanNsBoundCount, error) {
	var result []types.AcctTokenPlanNsBoundCount
	err := a.db.Core.NewSelect().
		Model((*AccountTokenPlan)(nil)).
		Column("status").
		Column("subject_type").
		ColumnExpr("COUNT(*) AS count").
		Where("jsonb_array_length(ns_uuids) > 0").
		Group("status", "subject_type").
		Order("status ASC", "subject_type ASC").
		Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

// SummaryActiveSubscriptions counts active token plan instances grouped by the
// subject type of the plan they subscribe to for the admin dashboard.
func (a *accountTokenPlanStoreImpl) SummaryActiveSubscriptions(ctx context.Context) ([]types.AcctTokenPlanActiveSubCount, error) {
	var result []types.AcctTokenPlanActiveSubCount
	err := a.db.Core.NewSelect().
		Model((*AccountTokenPlanInstance)(nil)).
		ModelTableExpr("account_token_plan_instances AS i").
		ColumnExpr("p.subject_type AS subject_type").
		ColumnExpr("COUNT(*) AS count").
		Join("JOIN account_token_plans AS p ON p.id = i.plan_id").
		Where("i.status = ?", types.SubscriptionStatusActive).
		Group("p.subject_type").
		Order("p.subject_type ASC").
		Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (a *accountTokenPlanStoreImpl) Logs(ctx context.Context, log AccountTokenPlanLog) error {
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return recordLog(ctx, tx, log)
	})

	if err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

func (a *accountTokenPlanStoreImpl) ListPublishedForNS(ctx context.Context, req types.AcctTokenPlanNSListReq) ([]AccountTokenPlan, int, error) {
	var result []AccountTokenPlan
	q := a.db.Core.NewSelect().Model(&result).
		Where("status = ?", types.TokenPlanStatusPublished)

	cond, err := json.Marshal([]string{req.NsUUID})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal namespace uuid filter, error: %w", err)
	}
	q = q.Where("(subject_type = '' OR (subject_type = ? AND (ns_uuids @> ?::jsonb OR ns_uuids = ?::jsonb)))",
		req.NsType, string(cond), "[]")

	err = q.Order("created_at DESC").Scan(ctx, &result)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	return result, len(result), nil
}

func (a *accountTokenPlanStoreImpl) GetActiveInstanceByNsUUID(ctx context.Context, req types.AcctTokenPlanNSGetReq) ([]AccountTokenPlanInstance, error) {
	var result []AccountTokenPlanInstance

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayAutoScan,
			},
		)
		if err != nil {
			return err
		}

		err = tx.NewSelect().Model(&result).
			Where("ns_uuid = ?", req.NsUUID).
			Where("status = ?", types.SubscriptionStatusActive).
			Order("id DESC").
			Scan(ctx)

		if err != nil {
			return err
		}

		for i := range result {
			instance := &result[i]
			err = loadLatestPeriodAndVersion(ctx, tx, instance)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return result, nil
}

// ListActiveTokenPlanNamespaces returns the distinct namespace UUIDs that own
// at least one active token plan instance.
func (a *accountTokenPlanStoreImpl) ListActiveTokenPlanNamespaces(ctx context.Context) ([]string, error) {
	var nsUUIDs []string
	err := a.db.Core.NewSelect().
		Model((*AccountTokenPlanInstance)(nil)).
		Column("ns_uuid").
		Where("status = ?", types.SubscriptionStatusActive).
		Group("ns_uuid").
		Order("ns_uuid ASC").
		Scan(ctx, &nsUUIDs)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return nsUUIDs, nil
}

// ListInstancesByNsUUID returns the paginated token plan subscription history
// of a namespace, newest instance first. Each instance includes its latest
// period and plan version for display purposes.
func (a *accountTokenPlanStoreImpl) ListInstancesByNsUUID(ctx context.Context, req types.AcctTokenPlanHistoryReq) ([]AccountTokenPlanInstance, int, error) {
	var result []AccountTokenPlanInstance
	total := 0

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		count, err := tx.NewSelect().Model((*AccountTokenPlanInstance)(nil)).
			Where("ns_uuid = ?", req.NsUUID).
			Count(ctx)
		if err != nil {
			return err
		}
		total = count

		err = tx.NewSelect().Model(&result).
			Where("ns_uuid = ?", req.NsUUID).
			Order("id DESC").
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per).
			Scan(ctx)
		if err != nil {
			return err
		}

		for i := range result {
			instance := &result[i]
			err = loadLatestPeriodAndVersion(ctx, tx, instance)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	return result, total, nil
}

// ListInstances returns one page of token plan subscription instances for the
// admin listing, newest instance first. Every filter is an optional pointer
// that combines with AND: NsUUIDs matches any of the given namespaces,
// CreatedAtStart and CreatedAtEnd bound the instance creation time in Unix
// seconds with an inclusive start and an exclusive end, Status and PlanID
// filter on the subscription state and the plan it binds to, and PlanCode
// matches plans whose code contains it, ignoring case, including soft-deleted
// plans so historical subscriptions remain reportable. Each instance is
// enriched with the namespace path, the plan version of its latest billing
// period and the summed amount and cost of all of its periods. Total counts
// every instance matching the same filters, not only the returned page. All of
// these reads share one transaction, so a concurrent renewal or statement
// update cannot make the count, the rows and their sums disagree.
func (a *accountTokenPlanStoreImpl) ListInstances(ctx context.Context, req types.AcctTokenPlanInstanceListReq) ([]AccountTokenPlanInstance, types.AcctTokenPlanInstanceTotalValue, error) {
	var result []AccountTokenPlanInstance
	var totalValue types.AcctTokenPlanInstanceTotalValue

	applyFilters := func(q *bun.SelectQuery, planCodeIDs *bun.SelectQuery) *bun.SelectQuery {
		if req.NsUUIDs != nil {
			if len(*req.NsUUIDs) > 0 {
				q = q.Where("ns_uuid IN (?)", bun.In(*req.NsUUIDs))
			} else {
				// An explicitly empty namespace list filters on nothing.
				q = q.Where("1 = 0")
			}
		}
		if req.Status != nil {
			q = q.Where("status = ?", *req.Status)
		}
		if req.PlanID != nil {
			q = q.Where("plan_id = ?", *req.PlanID)
		}
		if planCodeIDs != nil {
			// The plan code only lives on the plan table, so instances are
			// matched through the plans carrying that code.
			q = q.Where("plan_id IN (?)", planCodeIDs)
		}
		if req.CreatedAtStart != nil {
			q = q.Where("created_at >= ?", time.Unix(*req.CreatedAtStart, 0).In(config.GetGlobalTimeZone()))
		}
		if req.CreatedAtEnd != nil {
			q = q.Where("created_at < ?", time.Unix(*req.CreatedAtEnd, 0).In(config.GetGlobalTimeZone()))
		}
		return q
	}

	// Every read of the page, its display fields and its totals runs in one
	// transaction, so a concurrent renewal or statement update cannot make the
	// count, the rows and their sums disagree.
	err := a.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		// A plan code filter has to read the plan table, so its subquery is
		// built on the same transaction to keep every read of the listing on
		// one connection.
		var planCodeIDs *bun.SelectQuery
		if req.PlanCode != nil {
			// The plan code filter is a case-insensitive substring match, so a
			// user can search a plan by part of its code. The wildcard
			// characters are escaped to keep them literal instead of letting a
			// query of "%" match every plan.
			pattern := "%" + escapeLikePattern(strings.ToLower(*req.PlanCode)) + "%"
			planCodeIDs = tx.NewSelect().
				Model((*AccountTokenPlan)(nil)).
				WhereAllWithDeleted().
				Column("id").
				Where("LOWER(plan_code) LIKE ? ESCAPE '\\'", pattern)
		}

		count, err := applyFilters(tx.NewSelect().Model((*AccountTokenPlanInstance)(nil)), planCodeIDs).Count(ctx)
		if err != nil {
			return err
		}

		err = applyFilters(tx.NewSelect().Model(&result), planCodeIDs).
			Order("id DESC").
			Limit(req.Per).
			Offset((req.Page - 1) * req.Per).
			Scan(ctx)
		if err != nil {
			return err
		}
		totalValue.Total = int(count)

		if len(result) > 0 {
			if err := a.fillInstanceDisplayFields(ctx, tx, result); err != nil {
				return err
			}
			if err := a.fillInstancePeriodTotals(ctx, tx, result); err != nil {
				return err
			}
		}

		// The summed amount and cost cover every matching instance, not only the
		// returned page, so they are computed from the ids matching the same
		// filter without the pagination of the page being listed.
		matchingIDs := applyFilters(tx.NewSelect(), planCodeIDs).
			Model((*AccountTokenPlanInstance)(nil)).
			Column("id")
		filterTotals, err := a.sumInstancePeriodTotals(ctx, tx, matchingIDs)
		if err != nil {
			return err
		}
		totalValue.TotalAmount = filterTotals.TotalAmount
		totalValue.TotalCost = filterTotals.TotalCost

		return nil
	})
	if err != nil {
		return nil, types.AcctTokenPlanInstanceTotalValue{}, errorx.HandleDBError(err, nil)
	}

	return result, totalValue, nil
}

// fillInstanceDisplayFields enriches each instance with the path of the
// namespace that owns it and the plan version bound to its latest billing
// period. Both are resolved with one batched query per field so a page of
// instances does not fan out into per-row lookups. An instance whose namespace
// or latest period is missing keeps a nil field rather than failing the page.
func (a *accountTokenPlanStoreImpl) fillInstanceDisplayFields(ctx context.Context, tx bun.Tx, instances []AccountTokenPlanInstance) error {
	nsUUIDs := make([]string, 0, len(instances))
	seenNsUUID := make(map[string]struct{}, len(instances))
	for i := range instances {
		nsUUID := instances[i].NsUUID
		if nsUUID == "" {
			continue
		}
		if _, ok := seenNsUUID[nsUUID]; ok {
			continue
		}
		seenNsUUID[nsUUID] = struct{}{}
		nsUUIDs = append(nsUUIDs, nsUUID)
	}

	pathByNsUUID := map[string]string{}
	if len(nsUUIDs) > 0 {
		var namespaces []Namespace
		err := tx.NewSelect().
			Model(&namespaces).
			Column("uuid").
			Column("path").
			Where("uuid IN (?)", bun.In(nsUUIDs)).
			Scan(ctx)
		if err != nil {
			return err
		}
		for _, namespace := range namespaces {
			pathByNsUUID[namespace.UUID] = namespace.Path
		}
	}

	instanceIDs := make([]int64, 0, len(instances))
	for i := range instances {
		instanceIDs = append(instanceIDs, instances[i].ID)
	}

	// The latest period of an instance is the one with the greatest id, so the
	// highest period per instance is picked with a single DISTINCT ON pass.
	var latestPeriods []AccountTokenPlanInstancePeriod
	err := tx.NewSelect().
		Model(&latestPeriods).
		ColumnExpr("DISTINCT ON (instance_id) instance_id, id, plan_version_id").
		Where("instance_id IN (?)", bun.In(instanceIDs)).
		OrderExpr("instance_id ASC, id DESC").
		Scan(ctx)
	if err != nil {
		return err
	}

	versionByInstanceID := make(map[int64]int64, len(latestPeriods))
	for _, period := range latestPeriods {
		versionByInstanceID[period.InstanceID] = period.PlanVersionID
	}

	versionIDs := make([]int64, 0, len(versionByInstanceID))
	seenVersionID := make(map[int64]struct{}, len(versionByInstanceID))
	for _, versionID := range versionByInstanceID {
		if versionID <= 0 {
			continue
		}
		if _, ok := seenVersionID[versionID]; ok {
			continue
		}
		seenVersionID[versionID] = struct{}{}
		versionIDs = append(versionIDs, versionID)
	}

	versionByID := map[int64]*AccountTokenPlanVersion{}
	if len(versionIDs) > 0 {
		var versions []AccountTokenPlanVersion
		err := tx.NewSelect().
			Model(&versions).
			Where("id IN (?)", bun.In(versionIDs)).
			Scan(ctx)
		if err != nil {
			return err
		}
		for i := range versions {
			versionByID[versions[i].ID] = &versions[i]
		}
	}

	for i := range instances {
		instance := &instances[i]
		instance.NamespacePath = pathByNsUUID[instance.NsUUID]
		if versionID, ok := versionByInstanceID[instance.ID]; ok {
			instance.LatestPlanVersion = versionByID[versionID]
		}
	}
	return nil
}

// fillInstancePeriodTotals sums the amount paid and the metered cost of every
// billing period of each instance with one grouped query, so a page of
// instances does not fan out into per-row period lookups. Both totals stay in
// cents, as stored; converting them to yuan is the caller's job. An instance
// without any period keeps both totals at zero.
func (a *accountTokenPlanStoreImpl) fillInstancePeriodTotals(ctx context.Context, tx bun.Tx, instances []AccountTokenPlanInstance) error {
	if len(instances) == 0 {
		return nil
	}

	instanceIDs := make([]int64, 0, len(instances))
	for i := range instances {
		instanceIDs = append(instanceIDs, instances[i].ID)
	}

	var sums []struct {
		InstanceID int64   `bun:"instance_id"`
		Amount     float64 `bun:"amount"`
		CostAmount float64 `bun:"cost_amount"`
	}
	err := tx.NewSelect().
		Model((*AccountTokenPlanInstancePeriod)(nil)).
		Column("instance_id").
		ColumnExpr("COALESCE(SUM(amount), 0) AS amount").
		ColumnExpr("COALESCE(SUM(cost_amount), 0) AS cost_amount").
		Where("instance_id IN (?)", bun.In(instanceIDs)).
		Group("instance_id").
		Scan(ctx, &sums)
	if err != nil {
		return err
	}

	amountByInstanceID := make(map[int64]float64, len(sums))
	costByInstanceID := make(map[int64]float64, len(sums))
	for _, sum := range sums {
		amountByInstanceID[sum.InstanceID] = sum.Amount
		costByInstanceID[sum.InstanceID] = sum.CostAmount
	}

	for i := range instances {
		instance := &instances[i]
		instance.TotalAmount = amountByInstanceID[instance.ID]
		instance.TotalCost = costByInstanceID[instance.ID]
	}
	return nil
}

// sumInstancePeriodTotals sums the amount paid and the metered cost of the
// billing periods of every instance whose id the given subquery returns, in
// cents. Filtering by a subquery of instance ids keeps the sums independent of
// the pagination of the page being listed.
func (a *accountTokenPlanStoreImpl) sumInstancePeriodTotals(ctx context.Context, tx bun.Tx, matchingIDs *bun.SelectQuery) (types.AcctTokenPlanInstanceTotalValue, error) {
	var totals types.AcctTokenPlanInstanceTotalValue

	var sums []struct {
		Amount     float64 `bun:"amount"`
		CostAmount float64 `bun:"cost_amount"`
	}
	err := tx.NewSelect().
		TableExpr("account_token_plan_instance_periods AS p").
		Where("p.instance_id IN (?)", matchingIDs).
		ColumnExpr("COALESCE(SUM(p.amount), 0) AS amount").
		ColumnExpr("COALESCE(SUM(p.cost_amount), 0) AS cost_amount").
		Scan(ctx, &sums)
	if err != nil {
		return totals, err
	}
	if len(sums) > 0 {
		totals.TotalAmount = sums[0].Amount
		totals.TotalCost = sums[0].CostAmount
	}
	return totals, nil
}

// ListPeriodsForNS returns the paginated billing periods of a namespace,
// newest period first. The date range filters on the period start time:
// StartAt is inclusive and EndAt is exclusive, and a zero value leaves that
// side open. A non-empty Status filters on the period billing status. Each
// period carries the plan version bound to it so callers can show the plan
// terms that applied when the period was created. Total and TotalAmount cover
// every period matching the filter, not only the returned page. TotalCost is
// left zero because namespace-facing responses do not expose metered cost.
func (a *accountTokenPlanStoreImpl) ListPeriodsForNS(ctx context.Context, filter types.AcctTokenPlanPeriodsFilter) (*TokenPlanPeriodListResult, error) {
	var periods []AccountTokenPlanInstancePeriod
	result := &TokenPlanPeriodListResult{}

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		newQuery := func() *bun.SelectQuery {
			q := tx.NewSelect().
				TableExpr("account_token_plan_instance_periods AS p").
				Join("JOIN account_token_plan_instances AS i ON i.id = p.instance_id").
				Where("i.ns_uuid = ?", filter.NsUUID)
			if filter.StartAt > 0 {
				q = q.Where("p.start_at >= ?", filter.StartAt)
			}
			if filter.EndAt > 0 {
				q = q.Where("p.start_at < ?", filter.EndAt)
			}
			if filter.Status != "" {
				q = q.Where("p.status = ?", filter.Status)
			}
			return q
		}

		count, err := newQuery().Count(ctx)
		if err != nil {
			return err
		}
		result.Total = count

		var sum struct {
			TotalAmount int64 `bun:"total_amount"`
		}
		err = newQuery().
			ColumnExpr("COALESCE(SUM(p.amount), 0) AS total_amount").
			Scan(ctx, &sum)
		if err != nil {
			return err
		}
		result.TotalAmount = sum.TotalAmount

		err = newQuery().
			ColumnExpr("p.*").
			OrderExpr("p.start_at DESC, p.id DESC").
			Limit(filter.Per).
			Offset((filter.Page-1)*filter.Per).
			Scan(ctx, &periods)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	planVersions, err := a.loadPlanVersions(ctx, periods)
	if err != nil {
		return nil, err
	}

	result.Periods = make([]TokenPlanPeriodWithPlanVersion, 0, len(periods))
	for i := range periods {
		result.Periods = append(result.Periods, TokenPlanPeriodWithPlanVersion{
			AccountTokenPlanInstancePeriod: periods[i],
			PlanVersion:                    planVersions[periods[i].PlanVersionID],
		})
	}
	return result, nil
}

// ListPeriodsForInstance returns the paginated billing periods of one token
// plan instance for the admin listing, highest period number first. The date
// range filters on the period start time: StartAt is inclusive and EndAt is
// exclusive, and a zero value leaves that side open. A non-empty Status
// filters on the period billing status. Each period carries the plan version
// bound to it so the admin console can show the plan terms that applied when
// the period was created. Total, TotalAmount and TotalCost cover every period
// matching the filter, not only the returned page. An instance without
// periods, or an unknown instance id, yields an empty result rather than an
// error.
func (a *accountTokenPlanStoreImpl) ListPeriodsForInstance(ctx context.Context, filter types.AcctTokenPlanInstancePeriodsFilter) (*TokenPlanPeriodListResult, error) {
	var periods []AccountTokenPlanInstancePeriod
	result := &TokenPlanPeriodListResult{}

	err := a.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		newQuery := func() *bun.SelectQuery {
			q := tx.NewSelect().
				TableExpr("account_token_plan_instance_periods AS p").
				Where("p.instance_id = ?", filter.InstanceID)
			if filter.StartAt > 0 {
				q = q.Where("p.start_at >= ?", filter.StartAt)
			}
			if filter.EndAt > 0 {
				q = q.Where("p.start_at < ?", filter.EndAt)
			}
			if filter.Status != "" {
				q = q.Where("p.status = ?", filter.Status)
			}
			return q
		}

		count, err := newQuery().Count(ctx)
		if err != nil {
			return err
		}
		result.Total = count

		var sum struct {
			TotalAmount int64   `bun:"total_amount"`
			TotalCost   float64 `bun:"total_cost"`
		}
		err = newQuery().
			ColumnExpr("COALESCE(SUM(p.amount), 0) AS total_amount").
			ColumnExpr("COALESCE(SUM(p.cost_amount), 0) AS total_cost").
			Scan(ctx, &sum)
		if err != nil {
			return err
		}
		result.TotalAmount = sum.TotalAmount
		result.TotalCost = sum.TotalCost

		// The admin listing is ordered by the billing period number so the
		// newest cycle of the subscription comes first; the id breaks ties for
		// periods that share a number after a failed renewal.
		err = newQuery().
			ColumnExpr("p.*").
			OrderExpr("p.period_no DESC, p.id DESC").
			Limit(filter.Per).
			Offset((filter.Page-1)*filter.Per).
			Scan(ctx, &periods)
		return err
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	planVersions, err := a.loadPlanVersions(ctx, periods)
	if err != nil {
		return nil, err
	}

	result.Periods = make([]TokenPlanPeriodWithPlanVersion, 0, len(periods))
	for i := range periods {
		result.Periods = append(result.Periods, TokenPlanPeriodWithPlanVersion{
			AccountTokenPlanInstancePeriod: periods[i],
			PlanVersion:                    planVersions[periods[i].PlanVersionID],
		})
	}
	return result, nil
}

// loadPlanVersions loads the plan versions referenced by periods and indexes
// them by version ID.
func (a *accountTokenPlanStoreImpl) loadPlanVersions(ctx context.Context, periods []AccountTokenPlanInstancePeriod) (map[int64]*AccountTokenPlanVersion, error) {
	result := make(map[int64]*AccountTokenPlanVersion, len(periods))
	if len(periods) == 0 {
		return result, nil
	}

	ids := make([]int64, 0, len(periods))
	seen := make(map[int64]struct{}, len(periods))
	for _, period := range periods {
		if period.PlanVersionID <= 0 {
			continue
		}
		if _, ok := seen[period.PlanVersionID]; ok {
			continue
		}
		seen[period.PlanVersionID] = struct{}{}
		ids = append(ids, period.PlanVersionID)
	}
	if len(ids) == 0 {
		return result, nil
	}

	var versions []AccountTokenPlanVersion
	err := a.db.Core.NewSelect().Model(&versions).
		Where("id IN (?)", bun.In(ids)).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	for i := range versions {
		result[versions[i].ID] = &versions[i]
	}
	return result, nil
}

// loadLatestPeriodAndVersion fills the latest billing period and its bound
// plan version on an instance. Instances without any period are left empty.
func loadLatestPeriodAndVersion(ctx context.Context, tx bun.Tx, instance *AccountTokenPlanInstance) error {
	var latestPeriod AccountTokenPlanInstancePeriod
	err := tx.NewSelect().Model(&latestPeriod).
		Where("instance_id = ?", instance.ID).
		Order("id DESC").Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	err = loadPeriodUsages(ctx, tx, &latestPeriod)
	if err != nil {
		return err
	}
	instance.LatestPeriod = &latestPeriod

	var planVersion AccountTokenPlanVersion
	err = tx.NewSelect().Model(&planVersion).
		Where("id = ?", latestPeriod.PlanVersionID).
		Scan(ctx)
	if err != nil {
		return err
	}
	instance.LatestPlanVersion = &planVersion

	return nil
}

// loadPeriodUsages fills the runtime quota usage of a period that is not
// persisted on the period row itself.
func loadPeriodUsages(ctx context.Context, tx bun.Tx, period *AccountTokenPlanInstancePeriod) error {
	err := tx.NewSelect().Model(&period.ModelsUsage).
		Where("period_id = ?", period.ID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	err = tx.NewSelect().Model(&period.APIKeysUsage).
		Where("period_id = ?", period.ID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	return nil
}

// GetActiveSubscription returns the namespace's active token plan
// subscription, or nil when it has none. It is the first hop of the inference
// quota pre-check: a tenant without an active subscription is allowed through
// without reading any period or quota.
//
// It is deliberately read-only: the AI gateway calls it on the inference
// request path, so it must not reconcile, bill or otherwise write. Period
// rollover is owned by the periodic reconciler
// (ScanAndConfirmPeriodicalTokenPlans).
func (a *accountTokenPlanStoreImpl) GetActiveSubscription(ctx context.Context, nsUUID string) (*TokenPlanActiveSubscription, error) {
	var instance AccountTokenPlanInstance
	err := a.db.Core.NewSelect().Model(&instance).
		Where("ns_uuid = ?", nsUUID).
		Where("status = ?", types.SubscriptionStatusActive).
		Order("id DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &TokenPlanActiveSubscription{
		InstanceID:    instance.ID,
		OveragePolicy: instance.OveragePolicy,
	}, nil
}

// latestPeriodModelQuotaRow is the flat projection of the period/quota join.
// The quota columns are nullable because the latest period may not cover the
// requested model; the LEFT JOIN still returns the period row in that case.
// The fields map to the projected column names by bun's default snake_case
// conversion.
type latestPeriodModelQuotaRow struct {
	PeriodID int64
	StartAt  int64
	EndAt    int64

	QuotaID           sql.NullInt64
	QuotaModelID      sql.NullString
	QuotaTaskType     sql.NullString
	QuotaTokens       sql.NullInt64
	UsedTokens        sql.NullInt64
	QuotaRequestCount sql.NullInt64
	UsedRequestCount  sql.NullInt64
	QuotaCreatedAt    sql.NullTime
	QuotaUpdatedAt    sql.NullTime
}

// GetLatestPeriodModelQuota returns the latest billing period of the instance
// together with the quota row of one model in that period, in a single query.
// It returns (nil, nil) when the instance has no period yet, and the returned
// Quota is nil when the latest period does not cover the model.
//
// The lookup never falls back to an older period: the allowance enforced is the
// current one. It is read-only for the same reason as GetActiveSubscription.
func (a *accountTokenPlanStoreImpl) GetLatestPeriodModelQuota(ctx context.Context, instanceID int64, modelID string) (*TokenPlanPeriodQuota, error) {
	var row latestPeriodModelQuotaRow
	err := a.db.Core.NewSelect().
		TableExpr("account_token_plan_instance_periods AS p").
		ColumnExpr("p.id AS period_id, p.start_at, p.end_at").
		ColumnExpr("q.id AS quota_id, q.model_id AS quota_model_id, q.task_type AS quota_task_type").
		ColumnExpr("q.quota_tokens, q.used_tokens, q.quota_request_count, q.used_request_count").
		ColumnExpr("q.created_at AS quota_created_at, q.updated_at AS quota_updated_at").
		// "quota" is one of bun's non-pluralized table names (see
		// AccountTokenPlanPeriodModelQuota); the join must use that exact name.
		Join("LEFT JOIN account_token_plan_period_model_quota AS q ON q.period_id = p.id AND q.model_id = ?", modelID).
		Where("p.instance_id = ?", instanceID).
		OrderExpr("p.id DESC").
		Limit(1).
		Scan(ctx, &row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, nil)
	}

	periodQuota := &TokenPlanPeriodQuota{
		PeriodID: row.PeriodID,
		StartAt:  row.StartAt,
		EndAt:    row.EndAt,
	}
	if row.QuotaID.Valid {
		quota := &AccountTokenPlanPeriodModelQuota{
			ID:                row.QuotaID.Int64,
			PeriodID:          row.PeriodID,
			ModelID:           row.QuotaModelID.String,
			TaskType:          row.QuotaTaskType.String,
			QuotaTokens:       row.QuotaTokens.Int64,
			UsedTokens:        row.UsedTokens.Int64,
			QuotaRequestCount: row.QuotaRequestCount.Int64,
			UsedRequestCount:  row.UsedRequestCount.Int64,
		}
		quota.CreatedAt = row.QuotaCreatedAt.Time
		quota.UpdatedAt = row.QuotaUpdatedAt.Time
		periodQuota.Quota = quota
	}
	return periodQuota, nil
}

// GetPeriodByEvent returns the legitimate token plan billing period that
// covers one metering event, or nil when the event has no token plan coverage.
//
// The namespace's instances are reconciled first in the same transaction, as
// of RequestTime (the processing time) — an event that arrives just after the
// current period window closed must roll the subscription into its renewal
// period — and the covering period is then selected by EventTime (the usage
// time, which may be in the past) instead of "latest", so a late or replayed
// event is attributed to the period its usage actually happened in.
//
// A period qualifies purely by its window covering the event time
// (start_at <= event time < end_at): neither the period status nor the
// instance status is filtered, so an unpaid renewal period still covers the
// events of its window, and cancellation keeps the current period's quota
// until the window ends. When several periods match — an upgrade leaves the
// previous period's window overlapping the new one — the newest period wins.
//
// The reconcile runs with RunWayAutoScan semantics: a renewal whose fee
// deduction fails (e.g. insufficient balance) is logged and leaves the new
// period unpaid instead of failing the lookup; the unpaid period still covers
// the event by window.
func (a *accountTokenPlanStoreImpl) GetPeriodByEvent(ctx context.Context, req types.AcctTokenPlanNSEventReq) (*AccountTokenPlanInstancePeriod, error) {
	var result *AccountTokenPlanInstancePeriod

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayAutoScan,
			},
		)
		if err != nil {
			return err
		}

		var period AccountTokenPlanInstancePeriod
		eventTimeUnix := req.EventTime.Unix()
		err = tx.NewSelect().
			TableExpr("account_token_plan_instance_periods AS p").
			ColumnExpr("p.*").
			Join("JOIN account_token_plan_instances AS i ON i.id = p.instance_id").
			Where("i.ns_uuid = ?", req.NsUUID).
			Where("p.start_at <= ?", eventTimeUnix).
			Where("p.end_at > ?", eventTimeUnix).
			OrderExpr("p.id DESC").
			Limit(1).
			Scan(ctx, &period)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		result = &period
		return nil
	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (a *accountTokenPlanStoreImpl) CreateInstanceWithPeriod(ctx context.Context, req types.AcctTokenPlanPurchaseReq) (*AccountTokenPlanInstance, error) {

	newInstance := AccountTokenPlanInstance{}

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayUserAPI,
			},
		)
		if err != nil {
			return err
		}

		var existings []AccountTokenPlanInstance
		err = tx.NewSelect().Model(&existings).
			Where("ns_uuid = ?", req.NsUUID).
			Where("status = ?", types.SubscriptionStatusActive).
			Order("id DESC").
			Scan(ctx)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if len(existings) > 0 {
			return errorx.DuplicateKey(fmt.Errorf("namespace %s already has an active token plan", req.NsUUID), nil)
		}

		newInstance.NsUUID = req.NsUUID
		newInstance.PlanID = req.PlanID
		newInstance.OveragePolicy = req.OveragePolicy
		newInstance.StartAt = req.RequestTime.Unix()
		newInstance.EndAt = 0 // never expires, until manually canceled by user
		newInstance.PeriodCount = 0
		newInstance.Status = types.SubscriptionStatusActive
		newInstance.CreatedBy = req.CurrentUser

		res, err := tx.NewInsert().Model(&newInstance).Returning("*").Exec(ctx, &newInstance)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		err = reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   uuid.New(),
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayUserAPI,
			},
		)
		if err != nil {
			return err
		}

		res, err = tx.NewSelect().Model(&newInstance).WherePK().Exec(ctx, &newInstance)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}
		return nil

	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &newInstance, nil
}

// UpgradeInstance immediately switches an active token plan instance to a
// different plan. It ends the current instance and creates a new instance and
// first period in one transaction, then reuses reconcileNSTokenPlanInstances to
// charge the prorated first period fee.
func (a *accountTokenPlanStoreImpl) UpgradeInstance(ctx context.Context, req types.AcctTokenPlanUpgradeReq) (*AccountTokenPlanInstance, error) {
	var result AccountTokenPlanInstance

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayUserAPI,
			},
		)
		if err != nil {
			return err
		}

		prevInstance, prevPeriod, targetVersion, err := preUpgradePlan(ctx, tx, req)
		if err != nil {
			return err
		}

		req.EventUUID = uuid.New()
		err = doUpgradePlan(ctx, tx, req, prevInstance, prevPeriod, targetVersion)
		if err != nil {
			return err
		}

		err = tx.NewSelect().Model(&result).
			Where("ns_uuid = ?", req.NsUUID).
			Where("plan_id = ?", req.PlanID).
			Where("status = ?", types.SubscriptionStatusActive).
			Order("id DESC").
			Scan(ctx)

		if err != nil {
			return err
		}

		err = loadLatestPeriodAndVersion(ctx, tx, &result)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &result, nil
}

// CancelInstance stops the automatic renewal of an active instance at the end
// of its current billing period by setting the instance end time to the period
// end time. The instance stays active with its current period quota until then,
// and the fee paid for that period is not refunded.
func (a *accountTokenPlanStoreImpl) CancelInstance(ctx context.Context, req types.AcctTokenPlanCancelReq) (*AccountTokenPlanInstance, error) {
	var canceled AccountTokenPlanInstance

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayUserAPI,
			},
		)
		if err != nil {
			return err
		}

		err = tx.NewSelect().Model(&canceled).
			Where("id = ?", req.InstanceID).
			Where("ns_uuid = ?", req.NsUUID).
			Where("plan_id = ?", req.PlanID).
			For("UPDATE").
			Scan(ctx)
		if err != nil {
			return err
		}

		if canceled.Status != types.SubscriptionStatusActive {
			return nil
		}

		if canceled.EndAt <= 0 {
			var period AccountTokenPlanInstancePeriod
			err = tx.NewSelect().Model(&period).
				Where("instance_id = ?", canceled.ID).
				Order("id DESC").Limit(1).
				Scan(ctx)
			if err != nil {
				return err
			}

			if period.EndAt <= 0 {
				return fmt.Errorf("token plan instance %d has no valid current period end time", canceled.ID)
			}

			canceled.EndAt = period.EndAt
		}

		if canceled.EndAt <= req.RequestTime.Unix() {
			canceled.Status = types.SubscriptionStatusCanceled
		}

		res, err := tx.NewUpdate().Model(&canceled).WherePK().Returning("*").Exec(ctx, &canceled)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &canceled, nil
}

// UpdateInstanceOveragePolicy switches the overage policy of the active token
// plan instance bound to a namespace. Reconciliation runs first so that an
// already elapsed instance is not treated as active. Only the latest active
// instance is changed because purchasing enforces a single active instance per
// namespace.
func (a *accountTokenPlanStoreImpl) UpdateInstanceOveragePolicy(ctx context.Context, req types.AcctTokenPlanUpdateNSReq) (*AccountTokenPlanInstance, error) {
	var updated AccountTokenPlanInstance

	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := reconcileNSTokenPlanInstances(ctx, tx,
			ReconcileReq{
				NsUUID:      req.NsUUID,
				RequestTime: req.RequestTime,
				EventUUID:   req.EventUUID,
				CurrentUser: req.CurrentUser,
				Source:      types.RunWayUserAPI,
			},
		)
		if err != nil {
			return err
		}

		err = tx.NewSelect().Model(&updated).
			Where("ns_uuid = ?", req.NsUUID).
			Where("status = ?", types.SubscriptionStatusActive).
			Order("id DESC").Limit(1).
			For("UPDATE").
			Scan(ctx)
		if err != nil {
			return err
		}

		if updated.OveragePolicy == req.OveragePolicy {
			return errorx.ErrReqParamInvalid
		}

		updated.OveragePolicy = req.OveragePolicy
		res, err := tx.NewUpdate().Model(&updated).WherePK().Returning("*").Exec(ctx, &updated)
		if err := assertAffectedOneRow(res, err); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &updated, nil
}

// For scheduled task scan and reconcile token plan instances for namespace
func (a *accountTokenPlanStoreImpl) RecycleNSInstance(ctx context.Context, req types.AcctTokenPlanRecycleReq) error {
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		req := ReconcileReq{
			NsUUID:      req.NsUUID,
			RequestTime: req.RequestTime,
			EventUUID:   req.EventUUID,
			CurrentUser: req.CurrentUser,
			Source:      types.RunWayAutoScan,
		}
		err := reconcileNSTokenPlanInstances(ctx, tx, req)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	return nil
}

func reconcileNSTokenPlanInstances(ctx context.Context, tx bun.Tx, req ReconcileReq) error {
	err := lockNSTokenPlanInstances(ctx, tx, req.NsUUID)
	if err != nil {
		return err
	}

	var existings []AccountTokenPlanInstance
	err = tx.NewSelect().Model(&existings).
		Where("ns_uuid = ?", req.NsUUID).
		Where("status = ?", types.SubscriptionStatusActive).
		Order("id DESC").
		Scan(ctx)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if len(existings) < 1 {
		// no active token plan
		return nil
	}

	for _, instance := range existings {
		instRes := reconcileTokenPlanInstance(ctx, tx, instance, req.RequestTime, req.EventUUID)
		if instRes.Err == nil {
			continue
		}
		// instance reconcile logs
		log := AccountTokenPlanLog{
			CurrentUser: req.CurrentUser,
			Action:      types.TokenPlanActionBuildPeriod,
			Target:      fmt.Sprintf("instance: %d", instance.ID),
			DataNext:    instRes.NextVal,
			Explain:     instRes.Explain,
		}
		logErr := recordLog(ctx, tx, log)
		if logErr != nil {
			return fmt.Errorf("source: %s, instance_id: %d, explain: %s, log error: %w, reconcile error: %w",
				req.Source, instance.ID, instRes.Explain, logErr, instRes.Err)
		}
		if req.Source == types.RunWayAutoScan {
			continue
		}
		if errors.Is(instRes.Err, errorx.ErrInsufficientBalance) {
			return instRes.Err
		}

		return fmt.Errorf("source: %s, instance_id: %d, explain: %s, log error: %w, reconcile error: %w",
			req.Source, instance.ID, instRes.Explain, logErr, instRes.Err)

	}

	return nil
}

func reconcileTokenPlanInstance(ctx context.Context, tx bun.Tx, instance AccountTokenPlanInstance,
	requestTime time.Time, eventUUID uuid.UUID) InstanceReconcileResult {
	if instance.EndAt > 0 && instance.EndAt <= requestTime.Unix() {
		// cancel by user
		return markInstanceCanceled(ctx, tx, instance, requestTime)
	}

	if instance.EndAt == 0 {
		return reconcileInstancePeriod(ctx, tx, instance, requestTime, eventUUID)
	}

	return InstanceReconcileResult{NextVal: "", Explain: "", Err: nil}
}

func markInstanceCanceled(ctx context.Context, tx bun.Tx, instance AccountTokenPlanInstance, requestTime time.Time) InstanceReconcileResult {
	instance.Status = types.SubscriptionStatusCanceled
	instance.ExpiredAt = requestTime.Unix()
	res, err := tx.NewUpdate().Model(&instance).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return InstanceReconcileResult{
			Explain: fmt.Sprintf("MarkInstanceCanceled %d error: %s", instance.ID, err.Error()),
			Err:     err,
		}
	}
	return InstanceReconcileResult{Err: nil}
}

func markInstanceExpired(ctx context.Context, tx bun.Tx, instance AccountTokenPlanInstance, requestTime time.Time) InstanceReconcileResult {
	instance.Status = types.SubscriptionStatusExpired
	instance.ExpiredAt = requestTime.Unix()
	res, err := tx.NewUpdate().Model(&instance).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return InstanceReconcileResult{
			Explain: fmt.Sprintf("MarkInstanceExpired %d error: %s", instance.ID, err.Error()),
			Err:     err,
		}
	}
	return InstanceReconcileResult{Err: nil}
}

func reconcileInstancePeriod(ctx context.Context,
	tx bun.Tx, instance AccountTokenPlanInstance,
	requestTime time.Time, eventUUID uuid.UUID) InstanceReconcileResult {

	var period AccountTokenPlanInstancePeriod
	err := tx.NewSelect().Model(&period).
		Where("instance_id = ?", instance.ID).
		Order("period_no DESC").Limit(1).Scan(ctx, &period)

	if err == nil {
		if requestTime.Unix() < period.EndAt {
			// found latest period with valid period end time and check if it is paid
			feeResult := checkAndCreatePeriodFeeRecord(ctx, tx, &instance, requestTime, eventUUID, &period)
			return feeResult
		}
		if period.Status != types.BillingStatusPaid {
			// mark instance as expired when unpaid status is keeping for the whole period
			return markInstanceExpired(ctx, tx, instance, requestTime)
		}
		// found latest period and re-cycle new period
		reason := types.BillingReasonSubscriptionCycle
		return buildInstancePeriod(ctx, tx, instance, requestTime, eventUUID, reason, &period)
	}

	if errors.Is(err, sql.ErrNoRows) {
		//  create first period for no any period
		reason := types.BillingReasonSubscriptionCreate
		return buildInstancePeriod(ctx, tx, instance, requestTime, eventUUID, reason, nil)
	}

	return InstanceReconcileResult{
		Explain: fmt.Sprintf("ReconcileInstancePeriod.QueryLatestPeriod %d error: %s", instance.ID, err.Error()),
		Err:     err,
	}
}

func buildInstancePeriod(ctx context.Context, tx bun.Tx,
	instance AccountTokenPlanInstance,
	requestTime time.Time,
	eventUUID uuid.UUID,
	reason types.BillingReasion,
	prevPeriod *AccountTokenPlanInstancePeriod,
) InstanceReconcileResult {
	currentPeriod, err := createInstancePeriod(ctx, tx, &instance, requestTime, eventUUID, reason, prevPeriod)
	if err != nil {
		return InstanceReconcileResult{
			Explain: fmt.Sprintf("BuildInstancePeriod %d error: %s", instance.ID, err.Error()),
			Err:     err,
		}
	}

	feeResult := checkAndCreatePeriodFeeRecord(ctx, tx, &instance, requestTime, eventUUID, currentPeriod)
	return feeResult
}

func createInstancePeriod(ctx context.Context, tx bun.Tx,
	instance *AccountTokenPlanInstance,
	requestTime time.Time,
	eventUUID uuid.UUID,
	reason types.BillingReasion,
	prevPeriod *AccountTokenPlanInstancePeriod,
) (*AccountTokenPlanInstancePeriod, error) {
	// query latest plan version
	var latestPlanVersion AccountTokenPlanVersion
	err := tx.NewSelect().Model(&latestPlanVersion).
		Where("plan_id = ?", instance.PlanID).Order("version DESC").Limit(1).
		Scan(ctx, &latestPlanVersion)
	if err != nil {
		return nil, err
	}

	// new period
	newPeriod := AccountTokenPlanInstancePeriod{
		InstanceID:    instance.ID,
		PlanVersionID: latestPlanVersion.ID,
		EventID:       eventUUID.String(),
		Amount:        latestPlanVersion.PriceAmount,
		Status:        types.BillingStatusUnpaid,
		Reason:        reason,
	}

	if prevPeriod != nil {
		newPeriod.PeriodNo = prevPeriod.PeriodNo + 1
		newPeriod.StartAt = prevPeriod.EndAt
		newPeriod.EndAt = calculateTokenPlanPeriodEnd(time.Unix(newPeriod.StartAt, 0), latestPlanVersion.BillingPeriod).Unix()
	} else {
		newPeriod.PeriodNo = 1
		newPeriod.StartAt = requestTime.Unix()
		newPeriod.EndAt = calculateTokenPlanPeriodEnd(requestTime, latestPlanVersion.BillingPeriod).Unix()
	}

	res, err := tx.NewInsert().Model(&newPeriod).Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, err
	}

	// update instance period count
	instance.PeriodCount = instance.PeriodCount + 1
	if prevPeriod == nil {
		instance.FirstPlanVersionID = latestPlanVersion.ID
	}

	res, err = tx.NewUpdate().Model(instance).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, err
	}

	// add period quotas
	planQuotas := latestPlanVersion.Quotas
	if len(planQuotas) > 0 {
		var newPeriodQuotas []AccountTokenPlanPeriodModelQuota
		for _, quota := range planQuotas {
			newPeriodQuotas = append(newPeriodQuotas, AccountTokenPlanPeriodModelQuota{
				PeriodID:          newPeriod.ID,
				ModelID:           quota.ModelID,
				TaskType:          quota.TaskType,
				QuotaTokens:       quota.QuotaTokens,
				QuotaRequestCount: quota.QuotaRequestCount,
			})
		}
		res, err = tx.NewInsert().Model(&newPeriodQuotas).Returning("*").Exec(ctx)
		if err := assertAffectedXRows(int64(len(planQuotas)), res, err); err != nil {
			return nil, err
		}
	}

	return &newPeriod, nil
}

func calculateTokenPlanPeriodEnd(periodStart time.Time, billingPeriod int) time.Time {
	return calculateEndTimeByMonth(periodStart.In(config.GetGlobalTimeZone()), billingPeriod)
}

func checkAndCreatePeriodFeeRecord(ctx context.Context, tx bun.Tx,
	instance *AccountTokenPlanInstance,
	requestTime time.Time,
	eventUUID uuid.UUID,
	currentPeriod *AccountTokenPlanInstancePeriod,
) InstanceReconcileResult {
	if currentPeriod.Status == types.BillingStatusPaid {
		// period is paid, no need to deduct fee
		return InstanceReconcileResult{
			Err: nil,
		}
	}

	// deduct account fee
	amountValue := 0 - float64(currentPeriod.Amount)
	statement := AccountStatement{
		EventUUID:         eventUUID,
		UserUUID:          instance.NsUUID,
		Value:             amountValue,
		Scene:             types.SceneTokenPlan,
		OpUID:             instance.NsUUID,
		CreatedAt:         time.Now(),
		EventDate:         requestTime,
		Price:             float64(currentPeriod.Amount),
		RecordedAt:        requestTime,
		IsCancel:          false,
		EventValue:        amountValue,
		TokenPlanPeriodID: currentPeriod.ID,
		RecordedAtUnix:    requestTime.Unix(),
	}

	feeVal, err := json.Marshal(statement)
	if err != nil {
		return InstanceReconcileResult{
			Explain: fmt.Sprintf("DeductAccountFee.Marshal %s", err.Error()),
			Err:     err,
		}
	}

	_, feeErr := DeductAccountFee(ctx, tx, statement, true)
	explain := ""
	if feeErr != nil {
		currentPeriod.Status = types.BillingStatusUnpaid
		currentPeriod.RetryCount = currentPeriod.RetryCount + 1
		currentPeriod.RetryAt = requestTime.Unix()

		if errors.Is(feeErr, errorx.ErrInsufficientBalance) {
			instance.Status = types.SubscriptionStatusExpired
			instance.ExpiredAt = requestTime.Unix()
			currentPeriod.Reason = types.BillingReasonBalanceInsufficient
			currentPeriod.EndAt = requestTime.Unix() // close period
		} else {
			currentPeriod.Reason = types.BillingReasonDeductFailed
		}

		explain = feeErr.Error()
	} else {
		currentPeriod.Status = types.BillingStatusPaid
		explain = string(currentPeriod.Status)
	}

	res, err := tx.NewUpdate().Model(currentPeriod).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return InstanceReconcileResult{
			NextVal: string(feeVal),
			Explain: fmt.Sprintf("DeductAccountFee.UpdatePeriod %d error: : %s", currentPeriod.ID, err.Error()),
			Err:     err,
		}
	}

	res, err = tx.NewUpdate().Model(instance).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return InstanceReconcileResult{
			NextVal: string(feeVal),
			Explain: fmt.Sprintf("DeductAccountFee.UpdateInstance %d error: %s", instance.ID, err.Error()),
			Err:     err,
		}
	}

	return InstanceReconcileResult{
		NextVal: string(feeVal),
		Explain: fmt.Sprintf("DeductAccountFee: %s for period_no %d, period %d of instance %d",
			explain, currentPeriod.PeriodNo, currentPeriod.ID, instance.ID),
		Err: feeErr,
	}
}

func preUpgradePlan(ctx context.Context, tx bun.Tx, req types.AcctTokenPlanUpgradeReq) (*AccountTokenPlanInstance, *AccountTokenPlanInstancePeriod, *AccountTokenPlanVersion, error) {
	var prevInstance AccountTokenPlanInstance
	err := tx.NewSelect().Model(&prevInstance).
		Where("id = ?", req.InstanceID).
		Where("ns_uuid = ?", req.NsUUID).
		Where("status = ?", types.SubscriptionStatusActive).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var prevLatestPeriod AccountTokenPlanInstancePeriod
	err = tx.NewSelect().Model(&prevLatestPeriod).
		Where("instance_id = ?", prevInstance.ID).
		Order("period_no DESC").Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	var targetVersion AccountTokenPlanVersion
	err = tx.NewSelect().Model(&targetVersion).
		Where("plan_id = ?", req.PlanID).
		Order("version DESC").Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	if prevInstance.PlanID == req.PlanID {
		return nil, nil, nil, errorx.ErrTokenPlanInUsing
	}

	if prevLatestPeriod.Status != types.BillingStatusPaid {
		return nil, nil, nil, errorx.ErrTokenPlanLatestPeriodNotPaid
	}

	if prevLatestPeriod.Amount > targetVersion.PriceAmount {
		return nil, nil, nil, errorx.ErrTokenPlanPriceLower
	}

	return &prevInstance, &prevLatestPeriod, &targetVersion, nil
}

func doUpgradePlan(ctx context.Context, tx bun.Tx,
	req types.AcctTokenPlanUpgradeReq,
	prevInstance *AccountTokenPlanInstance,
	prevPeriod *AccountTokenPlanInstancePeriod,
	targetVersion *AccountTokenPlanVersion) error {

	var prevQuotas []AccountTokenPlanPeriodModelQuota
	err := tx.NewSelect().Model(&prevQuotas).
		Where("period_id = ?", prevPeriod.ID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	unusedAmount, consumedRate := calculateTokenPlanUnusedAmount(prevPeriod.Amount, prevQuotas)
	payableAmount := math.Max(0, float64(targetVersion.PriceAmount-unusedAmount))

	prevInstance.Status = types.SubscriptionStatusCanceled
	prevInstance.ExpiredAt = req.RequestTime.Unix()
	res, err := tx.NewUpdate().Model(prevInstance).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return err
	}

	newInstance := AccountTokenPlanInstance{
		NsUUID:             req.NsUUID,
		PlanID:             targetVersion.PlanID,
		FirstPlanVersionID: targetVersion.ID,
		OveragePolicy:      req.OveragePolicy,
		StartAt:            req.RequestTime.Unix(),
		EndAt:              0,
		PeriodCount:        0,
		Status:             types.SubscriptionStatusActive,
		CreatedBy:          req.CurrentUser,
	}

	res, err = tx.NewInsert().Model(&newInstance).Returning("*").Exec(ctx, &newInstance)
	if err := assertAffectedOneRow(res, err); err != nil {
		return err
	}

	reason := types.BillingReasionSubscriptionUpgrade
	currentPeriod, err := createInstancePeriod(ctx, tx, &newInstance, req.RequestTime, req.EventUUID, reason, nil)
	if err != nil {
		return err
	}

	currentPeriod.Amount = int64(payableAmount)
	res, err = tx.NewUpdate().Model(currentPeriod).WherePK().Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return err
	}

	feeResult := checkAndCreatePeriodFeeRecord(ctx, tx, &newInstance, req.RequestTime, req.EventUUID, currentPeriod)
	if feeResult.Err != nil {
		return feeResult.Err
	}

	// instance upgrade logs
	prevLog := "pre period %d with amount %d consumedRate %f unused fee %d"
	prevLog += ", target version %d with amount %d"
	prevLog += ", new period %d with amount %d"
	log := AccountTokenPlanLog{
		CurrentUser: req.CurrentUser,
		Action:      types.TokenPlanActionUpgrade,
		Target:      fmt.Sprintf("instance: %d", req.InstanceID),
		DataPrev: fmt.Sprintf(prevLog,
			prevPeriod.ID, prevPeriod.Amount, consumedRate, unusedAmount,
			targetVersion.ID, targetVersion.PriceAmount,
			currentPeriod.ID, currentPeriod.Amount),
		DataNext: feeResult.NextVal,
		Explain:  feeResult.Explain,
	}
	logErr := recordLog(ctx, tx, log)
	if logErr != nil {
		return fmt.Errorf("upgrade instance_id: %d to plan %d, explain: %s, log error: %w, error: %w",
			req.InstanceID, req.PlanID, feeResult.Explain, logErr, feeResult.Err)
	}

	return nil
}

// calculateTokenPlanUnusedAmount converts the unconsumed percentage of the old
// token plan period into a monetary amount. For each model the average of its
// token and request consumption percentages is calculated, then the average
// across models is used to derive the unused portion of the paid period amount.
func calculateTokenPlanUnusedAmount(paidAmount int64, quotas []AccountTokenPlanPeriodModelQuota) (int64, float64) {
	type modelUsage struct {
		rateSum   float64
		rateCount int
	}

	models := make(map[string]*modelUsage)
	for _, quota := range quotas {
		var rateSum float64
		var rateCount int

		if quota.QuotaTokens > 0 {
			rate := float64(quota.UsedTokens) / float64(quota.QuotaTokens)
			rateSum += math.Max(0, math.Min(1, rate))
			rateCount++
		}
		if quota.QuotaRequestCount > 0 {
			rate := float64(quota.UsedRequestCount) / float64(quota.QuotaRequestCount)
			rateSum += math.Max(0, math.Min(1, rate))
			rateCount++
		}
		if rateCount == 0 {
			continue
		}

		usage, ok := models[quota.ModelID]
		if !ok {
			usage = &modelUsage{}
			models[quota.ModelID] = usage
		}
		usage.rateSum += rateSum / float64(rateCount)
		usage.rateCount++
	}

	if len(models) == 0 {
		return 0, 0
	}

	var consumedRate float64
	for _, usage := range models {
		consumedRate += usage.rateSum / float64(usage.rateCount)
	}
	consumedRate /= float64(len(models))
	consumedRate = math.Max(0, math.Min(1, consumedRate))

	unusedAmount := float64(paidAmount) * (1 - consumedRate)
	return int64(math.Round(unusedAmount)), consumedRate
}

func recordLog(ctx context.Context, tx bun.Tx, log AccountTokenPlanLog) error {
	res, err := tx.NewInsert().Model(&log).Returning("*").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return err
	}
	return nil
}

// lockNSTokenPlanInstances serializes the changes of all token plan instances
// of one namespace until the current transaction ends. The lock must be taken
// before reading instances so that purchase, recycle and cancel cannot
// interleave.
func lockNSTokenPlanInstances(ctx context.Context, tx bun.Tx, nsUUID string) error {
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", nsUUID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("acquire token plan instance lock of namespace %s, error: %w", nsUUID, err)
	}
	return nil
}
