package database

import (
	"time"

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
	PeriodUnit    types.SkuUnitType                  `bun:",notnull,default:'month'" json:"period_unit"` // period unit
	Status        types.TokenPlanStatus              `bun:",notnull,default:0" json:"status"`
	CreatedBy     string                             `bun:",notnull" json:"created_by"`
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

	// ActiveSubscriptionCount is not persisted. It is calculated for admin
	// token plan listing and counts active AccountTokenPlanInstance rows that
	// reference this plan.
	ActiveSubscriptionCount int64 `bun:"-" json:"active_subscription_count"`
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
	StartAt            int64                    `bun:",notnull" json:"start_at"`               // start at unit(second)
	EndAt              int64                    `bun:",notnull" json:"end_at"`                 // end at unit(second)
	PeriodCount        int64                    `bun:",notnull,default:0" json:"period_count"` // total period count
	Status             types.SubscriptionStatus `bun:",notnull,default:''" json:"status"`
	ExpiredAt          int64                    `bun:",nullzero" json:"expired_at"`
	CreatedBy          string                   `bun:",notnull" json:"created_by"` // user
	times
	LatestPeriod      *AccountTokenPlanInstancePeriod `bun:"-" json:"latest_period,omitempty"`
	LatestPlanVersion *AccountTokenPlanVersion        `bun:"-" json:"latest_plan_version,omitempty"`
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
	ModelsUsage  []AccountTokenPlanPeriodModelQuota `bun:"-" json:"models_usage,omitempty"`
	APIKeysUsage []AccountTokenPlanPeriodKeyQuota   `bun:"-" json:"api_keys_usage,omitempty"`
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
