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
