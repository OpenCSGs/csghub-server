package types

import (
	"time"

	"github.com/google/uuid"
)

const DEFAULT_CURRENCY = "CNY"
const DEFAULT_PERIOD_UNIT = "month"

type RunWay string

const (
	RunWayUserAPI  RunWay = "User"
	RunWayAutoScan RunWay = "Scan"
)

type TokenPlanStatus int

const (
	TokenPlanStatusDraft     TokenPlanStatus = 0
	TokenPlanStatusDisabled  TokenPlanStatus = 4
	TokenPlanStatusPublished TokenPlanStatus = 9
)

// AcctTokenPlanSummaryItem is one group of token plan counts grouped by status
// and subject type for the admin summary API.
type AcctTokenPlanSummaryItem struct {
	Status      TokenPlanStatus `json:"status"`
	SubjectType string          `json:"subject_type"`
	Count       int64           `json:"count"`
}

// AcctTokenPlanNsBoundCount is one group of token plan counts restricted to
// specific namespaces (ns_uuids is not empty) grouped by status and subject
// type.
type AcctTokenPlanNsBoundCount struct {
	Status      TokenPlanStatus `json:"status"`
	SubjectType string          `json:"subject_type"`
	Count       int64           `json:"count"`
}

// AcctTokenPlanActiveSubCount is the count of active token plan subscriptions
// grouped by subject type.
type AcctTokenPlanActiveSubCount struct {
	SubjectType string `json:"subject_type"`
	Count       int64  `json:"count"`
}

// AcctTokenPlanSummary is the admin dashboard summary of token plans.
type AcctTokenPlanSummary struct {
	// ByStatusAndSubject groups all token plans by status and subject type.
	ByStatusAndSubject []AcctTokenPlanSummaryItem `json:"by_status_and_subject"`
	// NsBoundByStatusAndSubject groups token plans restricted to specific
	// namespaces (ns_uuids is not empty) by status and subject type.
	NsBoundByStatusAndSubject []AcctTokenPlanNsBoundCount `json:"ns_bound_by_status_and_subject"`
	// ActiveSubscriptionBySubject counts active token plan instances grouped by
	// subject type.
	ActiveSubscriptionBySubject []AcctTokenPlanActiveSubCount `json:"active_subscription_by_subject"`
}

type OveragePolicy string

const (
	OveragePolicyPayAsYouGo OveragePolicy = "pay_as_you_go"
	OveragePolicyReject     OveragePolicy = "reject"
)

const (
	TokenPlanActionUpdate              = "token_plan_update"
	TokenPlanActionDelete              = "token_plan_delete"
	TokenPlanActionBuildPeriod         = "token_plan_build_period"
	TokenPlanActionCancel              = "token_plan_cancel"
	TokenPlanActionUpdateOveragePolicy = "token_plan_update_overage_policy"
	TokenPlanActionUpgrade             = "token_plan_upgrade"
)

// AcctTokenPlanCreateReq is the request body for creating a token plan.
type AcctTokenPlanCreateReq struct {
	PlanCode      string                       `json:"plan_code" binding:"required"`
	PlanName      string                       `json:"plan_name" binding:"required"`
	Description   string                       `json:"description"`
	Details       string                       `json:"details"`
	SubjectType   string                       `json:"subject_type"`
	PriceAmount   int64                        `json:"price_amount" binding:"min=1"`
	Currency      string                       `json:"currency"`
	BillingPeriod int                          `json:"billing_period" binding:"min=1"`
	PeriodUnit    SkuUnitType                  `json:"-"`
	Quotas        []AccountTokenPlanModelQuota `json:"quotas"`
	NsUUIDs       []string                     `json:"ns_uuids"`
	CreatedBy     string                       `json:"-"`
}

// AcctTokenPlanUpdateReq is the request body for updating a token plan. All
// fields are optional (pointer) to support partial updates.
type AcctTokenPlanUpdateReq struct {
	PlanCode      *string                      `json:"plan_code"`
	PlanName      *string                      `json:"plan_name"`
	Description   *string                      `json:"description"`
	Details       *string                      `json:"details"`
	SubjectType   *string                      `json:"subject_type"`
	PriceAmount   *int64                       `json:"price_amount" binding:"omitempty,min=1"`
	Currency      *string                      `json:"currency"`
	BillingPeriod *int                         `json:"billing_period" binding:"omitempty,min=1"`
	Quotas        []AccountTokenPlanModelQuota `json:"quotas"`
	NsUUIDs       []string                     `json:"ns_uuids"`
	Status        *TokenPlanStatus             `json:"status"`
	ID            int64                        `json:"-"`
	CurrentUser   string                       `json:"-"`
}

// AcctTokenPlanListReq is the query parameters for listing token plans.
type AcctTokenPlanListReq struct {
	SubjectType *string          `json:"subject_type" form:"subject_type"`
	Status      *TokenPlanStatus `json:"status" form:"status"`
	Period      *int             `json:"period" form:"period"`
	Search      *string          `json:"search" form:"search"`
	Per         int              `json:"per" form:"per,default=20" binding:"min=1"`
	Page        int              `json:"page" form:"page,default=1" binding:"min=1"`
}

// AccountTokenPlanModelQuota mirrors database.AccountTokenPlanModelQuota for
// cross-layer transport.
type AccountTokenPlanModelQuota struct {
	ModelID           string  `json:"model_id"`
	TaskType          string  `json:"task_type"`
	QuotaTokens       int64   `json:"quota_tokens"`
	QuotaRequestCount int64   `json:"quota_request_count"`
	Weight            float64 `json:"weight"`
}

type AcctTokenPlanDelReq struct {
	ID          int64  `json:"id" binding:"required"`
	CurrentUser string `json:"-"`
}

type AcctTokenPlanGetReq struct {
	ID          int64  `json:"id" binding:"required"`
	CurrentUser string `json:"-"`
}

// AcctTokenPlanNSListReq is the query parameters for listing published token
// plans bound to a namespace.
type AcctTokenPlanNSListReq struct {
	NsUUID      string `json:"-"`
	NsType      string `json:"-"`
	CurrentUser string `json:"-"`
}

// AcctTokenPlanHistoryReq is the query parameters for listing token plan
// subscription history of a namespace.
type AcctTokenPlanHistoryReq struct {
	NsUUID      string `json:"-"`
	CurrentUser string `json:"-"`
	Per         int    `json:"-"`
	Page        int    `json:"-"`
}

// AcctTokenPlanInstanceListReq is the query parameters for the admin listing
// of every token plan subscription instance. Every filter is an optional
// pointer that combines with AND: a nil filter is not applied, while a
// non-nil empty NsUUIDs filters on nothing. NsUUIDs matches any of the given
// namespaces. StartDate and EndDate are inclusive YYYY-MM-DD values passed
// through to the accounting service; the client sends them unchanged and the
// accounting handler performs the single inclusive-to-exclusive conversion.
// CreatedAtStart is inclusive and CreatedAtEnd is exclusive, both in Unix
// seconds, and a nil bound leaves that side of the range open. They are kept
// for callers that already provide resolved bounds. PlanCode matches plans
// whose code contains it, ignoring case.
type AcctTokenPlanInstanceListReq struct {
	NsUUIDs        *[]string           `json:"ns_uuids" form:"ns_uuid"`
	Status         *SubscriptionStatus `json:"status" form:"status"`
	PlanID         *int64              `json:"plan_id" form:"plan_id"`
	PlanCode       *string             `json:"plan_code" form:"plan_code"`
	StartDate      string              `json:"start_date,omitempty" form:"start_date"`
	EndDate        string              `json:"end_date,omitempty" form:"end_date"`
	CreatedAtStart *int64              `json:"created_at_start" form:"created_at_start"`
	CreatedAtEnd   *int64              `json:"created_at_end" form:"created_at_end"`
	Per            int                 `json:"per" form:"per,default=20" binding:"min=1"`
	Page           int                 `json:"page" form:"page,default=1" binding:"min=1"`
}

// AcctTokenPlanPeriodsReq is the query parameters for listing the billing
// periods of a namespace. StartDate and EndDate are optional inclusive
// YYYY-MM-DD dates.
type AcctTokenPlanPeriodsReq struct {
	NsUUID      string        `json:"-"`
	CurrentUser string        `json:"-"`
	StartDate   string        `json:"-"`
	EndDate     string        `json:"-"`
	Status      BillingStatus `json:"-"`
	Per         int           `json:"-"`
	Page        int           `json:"-"`
}

// AcctTokenPlanPeriodsFilter is the resolved database filter for the period
// list. StartAt and EndAt bound the period start time in Unix seconds; a zero
// value leaves that side of the range open, and EndAt is exclusive.
type AcctTokenPlanPeriodsFilter struct {
	NsUUID  string        `json:"-"`
	StartAt int64         `json:"-"`
	EndAt   int64         `json:"-"`
	Status  BillingStatus `json:"-"`
	Per     int           `json:"-"`
	Page    int           `json:"-"`
}

// AcctTokenPlanInstancePeriodsReq is the query parameters for the admin
// listing of the billing periods of one token plan subscription instance.
// StartDate and EndDate are optional inclusive YYYY-MM-DD dates, and Status is
// an optional billing status filter.
type AcctTokenPlanInstancePeriodsReq struct {
	InstanceID int64         `json:"-"`
	StartDate  string        `json:"-"`
	EndDate    string        `json:"-"`
	Status     BillingStatus `json:"-"`
	Per        int           `json:"-"`
	Page       int           `json:"-"`
}

// AcctTokenPlanInstancePeriodsFilter is the resolved database filter for the
// admin instance period list. StartAt and EndAt bound the period start time in
// Unix seconds; a zero value leaves that side of the range open, and EndAt is
// exclusive.
type AcctTokenPlanInstancePeriodsFilter struct {
	InstanceID int64         `json:"-"`
	StartAt    int64         `json:"-"`
	EndAt      int64         `json:"-"`
	Status     BillingStatus `json:"-"`
	Per        int           `json:"-"`
	Page       int           `json:"-"`
}

// AcctTokenPlanPeriodsResp is a page of token plan billing periods. TotalAmount
// and TotalCost are the summed amount and metered cost of every period matching
// the filter, not just the returned page; both are carried in cents, the amount
// as an integer and the cost as a float because fractional cents accumulate on
// the cost_amount column.
type AcctTokenPlanPeriodsResp struct {
	Data        []AcctTokenPlanPeriodItem `json:"data"`
	Total       int                       `json:"total"`
	TotalAmount int64                     `json:"total_amount"`
	TotalCost   float64                   `json:"total_cost"`
}

// AcctTokenPlanInstance is a token plan subscription instance as carried over the
// accounting service boundary. NamespacePath is the path of the namespace that
// owns the instance, and LatestPlan is the immutable plan snapshot bound to its
// latest billing period. Both are nil when the namespace or the latest period
// cannot be resolved.
type AcctTokenPlanInstance struct {
	ID                 int64              `json:"id"`
	NsUUID             string             `json:"ns_uuid"`
	NamespacePath      string             `json:"namespace_path"`
	PlanID             int64              `json:"plan_id"`
	FirstPlanVersionID int64              `json:"first_plan_version_id"`
	OveragePolicy      OveragePolicy      `json:"overage_policy"`
	StartAt            int64              `json:"start_at"`
	EndAt              int64              `json:"end_at"`
	PeriodCount        int64              `json:"period_count"`
	Status             SubscriptionStatus `json:"status"`
	ExpiredAt          int64              `json:"expired_at"`
	CreatedBy          string             `json:"created_by"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	LatestPlan         *AcctTokenPlanInfo `json:"latest_plan,omitempty"`
	TotalAmount        float64            `json:"total_amount"`
	TotalCost          float64            `json:"total_cost"`
}

// AcctTokenPlanInstanceForNS is the tenant-facing projection of a token plan
// subscription instance. It mirrors the database model but omits the internal
// metered cost fields (total_cost and LatestPeriod.CostAmount) that tenants
// must never receive.
type AcctTokenPlanInstanceForNS struct {
	ID                 int64                      `json:"id"`
	NsUUID             string                     `json:"ns_uuid"`
	PlanID             int64                      `json:"plan_id"`
	FirstPlanVersionID int64                      `json:"first_plan_version_id"`
	OveragePolicy      OveragePolicy              `json:"overage_policy"`
	StartAt            int64                      `json:"start_at"`
	EndAt              int64                      `json:"end_at"`
	PeriodCount        int64                      `json:"period_count"`
	Status             SubscriptionStatus         `json:"status"`
	ExpiredAt          int64                      `json:"expired_at"`
	CreatedBy          string                     `json:"created_by"`
	CreatedAt          time.Time                  `json:"created_at"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	NamespacePath      string                     `json:"namespace_path"`
	TotalAmount        float64                    `json:"total_amount"`
	LatestPeriod       *AcctTokenPlanPeriodForNS  `json:"latest_period,omitempty"`
	LatestPlanVersion  *AcctTokenPlanVersionForNS `json:"latest_plan_version,omitempty"`
}

// AcctTokenPlanPeriodForNS is the tenant-facing projection of a billing
// period. It keeps the quota usage needed by the subscription page but omits
// the platform's internal metered cost.
type AcctTokenPlanPeriodForNS struct {
	ID            int64                           `json:"id"`
	InstanceID    int64                           `json:"instance_id"`
	PlanVersionID int64                           `json:"plan_version_id"`
	PeriodNo      int64                           `json:"period_no"`
	StartAt       int64                           `json:"start_at"`
	EndAt         int64                           `json:"end_at"`
	EventID       string                          `json:"event_id"`
	Amount        int64                           `json:"amount"`
	Status        BillingStatus                   `json:"status"`
	Reason        BillingReasion                  `json:"reason"`
	RetryCount    int64                           `json:"retry_count"`
	RetryAt       int64                           `json:"retry_at"`
	CreatedAt     time.Time                       `json:"created_at"`
	UpdatedAt     time.Time                       `json:"updated_at"`
	ModelsUsage   []AcctTokenPlanPeriodModelUsage `json:"models_usage,omitempty"`
	APIKeysUsage  []AcctTokenPlanPeriodKeyUsage   `json:"api_keys_usage,omitempty"`
}

// AcctTokenPlanVersionForNS is the tenant-facing projection of the immutable
// plan version snapshot bound to a subscription's latest period.
type AcctTokenPlanVersionForNS struct {
	ID            int64                        `json:"id"`
	PlanID        int64                        `json:"plan_id"`
	Version       int64                        `json:"version"`
	PlanCode      string                       `json:"plan_code"`
	PlanName      string                       `json:"plan_name"`
	Description   string                       `json:"description"`
	Details       string                       `json:"details"`
	SubjectType   string                       `json:"subject_type"`
	PriceAmount   int64                        `json:"price_amount"`
	Currency      string                       `json:"currency"`
	BillingPeriod int                          `json:"billing_period"`
	PeriodUnit    SkuUnitType                  `json:"period_unit"`
	Status        TokenPlanStatus              `json:"status"`
	CreatedBy     string                       `json:"created_by"`
	PublishedAt   time.Time                    `json:"published_at"`
	DisabledAt    time.Time                    `json:"disabled_at"`
	Quotas        []AccountTokenPlanModelQuota `json:"quotas"`
	NsUUIDs       []string                     `json:"ns_uuids"`
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

// AcctTokenPlanPeriodModelUsage is the tenant-facing runtime usage of one
// model quota within a billing period.
type AcctTokenPlanPeriodModelUsage struct {
	ID                int64     `json:"id"`
	PeriodID          int64     `json:"period_id"`
	ModelID           string    `json:"model_id"`
	TaskType          string    `json:"task_type"`
	QuotaTokens       int64     `json:"quota_tokens"`
	UsedTokens        int64     `json:"used_tokens"`
	QuotaRequestCount int64     `json:"quota_request_count"`
	UsedRequestCount  int64     `json:"used_request_count"`
	LatestUsedAt      int64     `json:"latest_used_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AcctTokenPlanPeriodKeyUsage is the tenant-facing runtime usage of one API
// key quota within a billing period.
type AcctTokenPlanPeriodKeyUsage struct {
	ID                int64     `json:"id"`
	PeriodID          int64     `json:"period_id"`
	TokenID           int64     `json:"token_id"`
	ModelID           string    `json:"model_id"`
	TaskType          string    `json:"task_type"`
	QuotaTokens       int64     `json:"quota_tokens"`
	UsedTokens        int64     `json:"used_tokens"`
	QuotaRequestCount int64     `json:"quota_request_count"`
	UsedRequestCount  int64     `json:"used_request_count"`
	Remark            string    `json:"remark"`
	LatestUsedAt      int64     `json:"latest_used_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AcctTokenPlanInstanceListResp is one page of token plan subscription
// instances for the admin listing. Total counts every instance matching the
// same filters, not only the returned page.
type AcctTokenPlanInstanceListResp struct {
	Data []AcctTokenPlanInstance `json:"data"`
	AcctTokenPlanInstanceTotalValue
}

// AcctTokenPlanInstanceTotalValue carries the totals of an instance listing.
// TotalAmount and TotalCost are floats.
type AcctTokenPlanInstanceTotalValue struct {
	Total       int     `json:"total"`
	TotalAmount float64 `json:"total_amount"`
	TotalCost   float64 `json:"total_cost"`
}

// AcctTokenPlanPeriodItem is one token plan billing period together with the
// immutable plan snapshot bound to that period. CostAmount is nil when the
// caller must not expose cost; a non-nil pointer carries the cost even when it
// is zero.
type AcctTokenPlanPeriodItem struct {
	ID            int64              `json:"id"`
	InstanceID    int64              `json:"instance_id"`
	PlanVersionID int64              `json:"plan_version_id"`
	PeriodNo      int64              `json:"period_no"`
	StartAt       int64              `json:"start_at"`
	EndAt         int64              `json:"end_at"`
	EventID       string             `json:"event_id"`
	Amount        int64              `json:"amount"`
	Status        BillingStatus      `json:"status"`
	Reason        BillingReasion     `json:"reason"`
	RetryCount    int64              `json:"retry_count"`
	RetryAt       int64              `json:"retry_at"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	CostAmount    *float64           `json:"cost_amount,omitempty"`
	Plan          *AcctTokenPlanInfo `json:"plan,omitempty"`
}

// AcctTokenPlanInfo is the basic plan information captured by the plan version
// bound to a billing period, so historical periods keep the plan terms that
// applied when they were created.
type AcctTokenPlanInfo struct {
	PlanID        int64       `json:"plan_id"`
	Version       int64       `json:"version"`
	PlanCode      string      `json:"plan_code"`
	PlanName      string      `json:"plan_name"`
	Description   string      `json:"description"`
	Details       string      `json:"details"`
	SubjectType   string      `json:"subject_type"`
	PriceAmount   int64       `json:"price_amount"`
	Currency      string      `json:"currency"`
	BillingPeriod int         `json:"billing_period"`
	PeriodUnit    SkuUnitType `json:"period_unit"`
}

// AcctTokenPlanPurchaseReq is the request body for purchasing a token plan
// for a namespace.
type AcctTokenPlanPurchaseReq struct {
	PlanID        int64         `json:"plan_id" binding:"required"`
	OveragePolicy OveragePolicy `json:"overage_policy" binding:"required,oneof=pay_as_you_go reject"`
	NsUUID        string        `json:"-"`
	CurrentUser   string        `json:"-"`
	RequestTime   time.Time     `json:"-"`
	EventUUID     uuid.UUID     `json:"-"`
}

// AcctTokenPlanNSGetReq identifies the namespace whose current subscription
// is requested.
type AcctTokenPlanNSGetReq struct {
	NsUUID      string    `json:"-"`
	CurrentUser string    `json:"-"`
	RequestTime time.Time `json:"-"`
	EventUUID   uuid.UUID `json:"-"`
}

// AcctTokenPlanCancelReq identifies the token plan instance to cancel.
// Cancellation stops the automatic renewal at the end of the current billing
// period and keeps the quota of that period; the fee already paid for it is
// not refunded.
type AcctTokenPlanCancelReq struct {
	PlanID      int64     `json:"plan_id" binding:"required"`
	InstanceID  int64     `json:"instance_id" binding:"required"`
	NsUUID      string    `json:"-"`
	CurrentUser string    `json:"-"`
	RequestTime time.Time `json:"-"`
	EventUUID   uuid.UUID `json:"-"`
}

// AcctTokenPlanUpdateNSReq is the request body for switching the overage
// policy of the active token plan subscription bound to a namespace.
type AcctTokenPlanUpdateNSReq struct {
	OveragePolicy OveragePolicy `json:"overage_policy" binding:"required,oneof=pay_as_you_go reject"`
	NsUUID        string        `json:"-"`
	CurrentUser   string        `json:"-"`
	RequestTime   time.Time     `json:"-"`
	EventUUID     uuid.UUID     `json:"-"`
}

// AcctTokenPlanUpgradeReq upgrades an active token plan subscription to a
// different token plan immediately. The current instance is ended and a new
// instance is created in one transaction; the first period fee is the new plan
// price minus the unconsumed value of the last period of the previous plan.
type AcctTokenPlanUpgradeReq struct {
	PlanID        int64         `json:"plan_id" binding:"required"`
	InstanceID    int64         `json:"instance_id" binding:"required"`
	OveragePolicy OveragePolicy `json:"overage_policy" binding:"required,oneof=pay_as_you_go reject"`
	NsUUID        string        `json:"-"`
	CurrentUser   string        `json:"-"`
	RequestTime   time.Time     `json:"-"`
	EventUUID     uuid.UUID     `json:"-"`
}

type AcctTokenPlanRecycleReq = AcctTokenPlanNSGetReq

type TokenPlanQuotaReq struct {
	NsUUID  string
	ModelID string
}

type AcctTokenPlanNSEventReq struct {
	NsUUID      string    `json:"-"`
	CurrentUser string    `json:"-"`
	RequestTime time.Time `json:"-"` // current request time
	EventUUID   uuid.UUID `json:"-"`
	EventTime   time.Time `json:"-"` // event time can be past time
}

func IsTokenMetred(taskType string) bool {
	switch taskType {
	case string(TextGeneration),
		string(TextRanking),
		string(FeatureExtraction),
		string(TypedDecisions):
		return true
	default:
		return false
	}
}
