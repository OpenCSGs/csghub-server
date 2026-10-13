package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	CreateInstanceWithPeriod(ctx context.Context, req types.AcctTokenPlanPurchaseReq) (*AccountTokenPlanInstance, error)
	UpgradeInstance(ctx context.Context, req types.AcctTokenPlanUpgradeReq) (*AccountTokenPlanInstance, error)
	RecycleNSInstance(ctx context.Context, req types.AcctTokenPlanRecycleReq) error
	CancelInstance(ctx context.Context, req types.AcctTokenPlanCancelReq) (*AccountTokenPlanInstance, error)
	UpdateInstanceOveragePolicy(ctx context.Context, req types.AcctTokenPlanUpdateNSReq) (*AccountTokenPlanInstance, error)
}

type InstanceReconcileResult struct {
	NextVal string
	Explain string
	Err     error
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

func (a *accountTokenPlanStoreImpl) Create(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error) {
	res, err := a.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, nil)
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

func (a *accountTokenPlanStoreImpl) Update(ctx context.Context, input AccountTokenPlan) (*AccountTokenPlan, error) {
	err := a.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var locked AccountTokenPlan
		err := tx.NewSelect().Model(&locked).Where("id = ?", input.ID).For("UPDATE").Scan(ctx)
		if err != nil {
			return err
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
		return nil, errorx.HandleDBError(err, nil)
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
	return result, count, nil
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
