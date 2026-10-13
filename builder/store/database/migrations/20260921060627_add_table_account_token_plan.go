package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountTokenPlanCommon struct {
	ID            int64                              `bun:",pk,autoincrement" json:"id"`
	PlanCode      string                             `bun:",notnull" json:"plan_code"`
	PlanName      string                             `bun:",notnull" json:"plan_name"`
	Description   string                             `bun:",nullzero" json:"description"`
	Details       string                             `bun:",nullzero" json:"details"`
	SubjectType   NamespaceType                      `bun:",notnull,default:''" json:"subject_type"` // user or organization
	PriceAmount   int64                              `bun:",notnull,default:0" json:"price_amount"`  // cent
	Currency      string                             `bun:",notnull,default:'CNY'" json:"currency"`
	BillingPeriod int                                `bun:",notnull,default:1" json:"billing_period"`    // period count
	PeriodUnit    string                             `bun:",notnull,default:'month'" json:"period_unit"` // period unit
	Status        types.TokenPlanStatus              `bun:",notnull,default:0" json:"status"`
	CreatedBy     string                             `bun:",notnull" json:"created_by"` // user uuid
	PublishedAt   time.Time                          `bun:",nullzero" json:"published_at"`
	DisabledAt    time.Time                          `bun:",nullzero" json:"disabled_at"`
	Quotas        []types.AccountTokenPlanModelQuota `bun:",type:jsonb,notnull,default:'[]'" json:"quotas"`
	NsUUIDs       []string                           `bun:"ns_uuids,type:jsonb,notnull,default:'[]'" json:"ns_uuids"`
	times
}

// AccountTokenPlan defines a token plan package template for personal or
// organization API Token plans. Draft plans cannot be purchased until
// published; disabled plans cannot be used for new token plans.
type AccountTokenPlan struct {
	AccountTokenPlanCommon
	LatestVersionID int64     `bun:",notnull,default:0" json:"latest_version_id"`
	DeletedAt       time.Time `bun:",soft_delete,nullzero" json:"deleted_at"`
}

// AccountTokenPlanVersion is an immutable snapshot of a plan configuration
// captured at publish time. Active token plan periods bind to a specific
// version so later plan edits never retroactively affect running cycles.
type AccountTokenPlanVersion struct {
	AccountTokenPlanCommon
	PlanID  int64 `bun:",notnull" json:"plan_id"`
	Version int64 `bun:",notnull,default:0" json:"version"`
}

// AccountTokenPlanInstance represents a single active token plan purchase for
// a personal user or organization. It binds to a plan version snapshot and
// tracks the user-selected overage policy which can be switched dynamically.
type AccountTokenPlanInstance struct {
	ID                 int64                    `bun:",pk,autoincrement" json:"id"`
	NsUUID             string                   `bun:"ns_uuid,notnull" json:"ns_uuid"`
	PlanID             int64                    `bun:",notnull" json:"plan_id"`
	FirstPlanVersionID int64                    `bun:",notnull" json:"first_plan_version_id"` // plan version id of first period
	OveragePolicy      types.OveragePolicy      `bun:",notnull,default:''" json:"overage_policy"`
	StartAt            int64                    `bun:",notnull" json:"start_at"`               // plan start at unit(second)
	EndAt              int64                    `bun:",notnull" json:"end_at"`                 // plan end at unit(second)
	PeriodCount        int64                    `bun:",notnull,default:0" json:"period_count"` // total period count
	Status             types.SubscriptionStatus `bun:",notnull,default:''" json:"status"`
	ExpiredAt          int64                    `bun:",nullzero" json:"expired_at"`
	CreatedBy          string                   `bun:",notnull" json:"created_by"` // user uuid
	times
}

// AccountTokenPlanInstancePeriod tracks each billing cycle of a subscription. Renewal
// creates a new period and re-initializes quotas from the plan version bound
// at renewal time. Historical periods are never overwritten.
type AccountTokenPlanInstancePeriod struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	InstanceID    int64                `bun:",notnull" json:"instance_id"`
	PlanVersionID int64                `bun:",notnull" json:"plan_version_id"`
	PeriodNo      int64                `bun:",notnull" json:"period_no"`
	StartAt       int64                `bun:",notnull" json:"start_at"` // period start at unit(second)
	EndAt         int64                `bun:",notnull" json:"end_at"`   // period end at unit(second)
	EventID       string               `bun:",nullzero" json:"event_id"`
	Amount        int64                `bun:",notnull,default:0" json:"amount"` // cent
	Status        types.BillingStatus  `bun:",notnull,default:''" json:"status"`
	Reason        types.BillingReasion `bun:",notnull,default:''" json:"reason"`
	RetryCount    int64                `bun:",notnull,default:0" json:"retry_count"`
	RetryAt       int64                `bun:",notnull,default:0" json:"retry_at"`
	times
}

// AccountTokenPlanPeriodModelQuota holds the runtime per-model quota balance for
// a token plan period. Quota is initialized from the plan version at period
// start and consumed atomically during API calls.
type AccountTokenPlanPeriodModelQuota struct {
	ID                int64  `bun:",pk,autoincrement" json:"id"`
	PeriodID          int64  `bun:",notnull" json:"period_id"`
	ModelID           string `bun:",notnull" json:"model_id"`
	TaskType          string `bun:",notnull" json:"task_type"`
	QuotaTokens       int64  `bun:",notnull,default:0" json:"quota_tokens"`
	UsedTokens        int64  `bun:",notnull,default:0" json:"used_tokens"`
	QuotaRequestCount int64  `bun:",notnull,default:0" json:"quota_request_count"`
	UsedRequestCount  int64  `bun:",notnull,default:0" json:"used_request_count"`
	LatestUsedAt      int64  `bun:",notnull,default:0" json:"latest_used_at"`
	times
}

// AccountTokenPlanKeyQuota allocates an optional per-API-Key quota limit within
// a token plan period.
type AccountTokenPlanPeriodKeyQuota struct {
	ID                int64  `bun:",pk,autoincrement" json:"id"`
	PeriodID          int64  `bun:",notnull" json:"period_id"`
	TokenID           int64  `bun:",notnull" json:"token_id"`
	ModelID           string `bun:",notnull" json:"model_id"`
	TaskType          string `bun:",notnull" json:"task_type"`
	QuotaTokens       int64  `bun:",notnull,default:0" json:"quota_tokens"`
	UsedTokens        int64  `bun:",notnull,default:0" json:"used_tokens"`
	QuotaRequestCount int64  `bun:",notnull,default:0" json:"quota_request_count"`
	UsedRequestCount  int64  `bun:",notnull,default:0" json:"used_request_count"`
	Remark            string `bun:",nullzero" json:"remark"`
	LatestUsedAt      int64  `bun:",notnull,default:0" json:"latest_used_at"`
	times
}

type AccountTokenPlanLog struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	CurrentUser string `bun:",notnull" json:"current_user"`
	Action      string `bun:",notnull" json:"action"`
	Target      string `bun:",notnull" json:"target"`
	DataPrev    string `bun:",nullzero" json:"data_prev"`
	DataNext    string `bun:",nullzero" json:"data_next"`
	Explain     string `bun:",nullzero" json:"explain"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db,
			AccountTokenPlan{},
			AccountTokenPlanVersion{},
			AccountTokenPlanInstance{},
			AccountTokenPlanInstancePeriod{},
			AccountTokenPlanPeriodModelQuota{},
			AccountTokenPlanPeriodKeyQuota{},
			AccountTokenPlanLog{},
		)
		if err != nil {
			return fmt.Errorf("failed to create account token plan tables: %w", err)
		}

		indexes := []struct {
			model   any
			name    string
			columns []string
		}{
			{&AccountTokenPlan{},
				"idx_acct_token_plans_status_subtype_period",
				[]string{"status", "subject_type", "billing_period"},
			},
			{&AccountTokenPlanInstance{},
				"idx_acct_token_plans_uuid_planid_ver_status",
				[]string{"ns_uuid", "plan_id", "first_plan_version_id", "status"},
			},
			{&AccountTokenPlanInstance{},
				"idx_acct_token_plans_uuid_start_end_status",
				[]string{"ns_uuid", "start_at", "end_at", "status"},
			},
		}

		for _, idx := range indexes {
			_, err = db.NewCreateIndex().
				Model(idx.model).
				Index(idx.name).
				Column(idx.columns...).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index %s: %w", idx.name, err)
			}
		}

		uniqueIndexes := []struct {
			model   any
			name    string
			columns []string
		}{
			{&AccountTokenPlan{},
				"idx_acct_token_plans_plan_code", []string{"plan_code"},
			},
			{&AccountTokenPlanVersion{},
				"idx_acct_token_plans_ver_planid_version", []string{"plan_id", "version"},
			},
			{&AccountTokenPlanPeriodModelQuota{},
				"idx_acct_token_plans_period_periodid_modelid", []string{"period_id", "model_id"},
			},
			{&AccountTokenPlanPeriodKeyQuota{},
				"idx_acct_token_plans_period_periodid_tokenid_modelid", []string{"period_id", "token_id", "model_id"},
			},
			{&AccountTokenPlanInstancePeriod{},
				"idx_acct_token_plans_period_instid_planverid_perno",
				[]string{"instance_id", "plan_version_id", "period_no"},
			},
		}

		for _, idx := range uniqueIndexes {
			_, err = db.NewCreateIndex().
				Model(idx.model).
				Index(idx.name).
				Column(idx.columns...).
				Unique().
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index %s: %w", idx.name, err)
			}
		}

		// GIN index on NSUUIDs (JSONB string array) for containment queries (@>, ?, ?|, ?&).
		_, err = db.NewCreateIndex().
			Model(&AccountTokenPlan{}).
			Index("idx_acct_token_plans_ns_uuids").
			Using("GIN").
			Column("ns_uuids").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_acct_token_plans_ns_uuids: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		err := dropTables(ctx, db,
			AccountTokenPlan{},
			AccountTokenPlanVersion{},
			AccountTokenPlanInstance{},
			AccountTokenPlanInstancePeriod{},
			AccountTokenPlanPeriodModelQuota{},
			AccountTokenPlanPeriodKeyQuota{},
			AccountTokenPlanLog{},
		)
		if err != nil {
			return fmt.Errorf("failed to drop account token plan tables: %w", err)
		}
		return nil
	})
}
