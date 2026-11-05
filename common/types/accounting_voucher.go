package types

import (
	"time"
)

type VoucherType string

const (
	VoucherTypeCompute   VoucherType = "compute"
	VoucherTypeToken     VoucherType = "token"
	VoucherTypeUniversal VoucherType = "universal"
)

type VoucherStatus string

const (
	VoucherStatusPending VoucherStatus = "pending"
	VoucherStatusActive  VoucherStatus = "active"
	VoucherStatusExpired VoucherStatus = "expired"
	VoucherStatusRevoked VoucherStatus = "revoked"
)

type RuleItemType string

const (
	RuleItemTypeAny     RuleItemType = "any"
	RuleItemTypeInclude RuleItemType = "include"
)

type VoucherRules struct {
	ComputeLimits []VoucherComputeRule `json:"compute_limits"`
	TokenLimits   VoucherTokenRule     `json:"token_limits"`
}

type VoucherComputeRule struct {
	ClusterIDs RuleItem `json:"cluster_ids"`
	XPUModels  RuleItem `json:"xpu_models"`
}

type VoucherTokenRule struct {
	Models RuleItem `json:"models"`
}

type RuleItem struct {
	Mode   RuleItemType `json:"mode"`
	Values []string     `json:"values"`
}

type CreateVoucherReq struct {
	Type       VoucherType  `json:"type" binding:"required"`
	Name       string       `json:"name" binding:"required"`
	BatchNo    string       `json:"batch_no" binding:"required"`
	TargetUUID string       `json:"target_uuid" binding:"required"`
	Total      float64      `json:"total" binding:"required,min=1"`
	BeginDate  time.Time    `json:"begin_date" binding:"required"`
	EndDate    time.Time    `json:"end_date" binding:"required"`
	Rules      VoucherRules `json:"rules" binding:"required"`
	Notes      string       `json:"notes"`
	IssueUUID  string       `json:"-"`
	IssueName  string       `json:"-"`
}

type UpdateVoucherReq struct {
	Type        *VoucherType  `json:"type"`
	Name        *string       `json:"name"`
	BatchNo     *string       `json:"batch_no"`
	Total       *float64      `json:"total"`
	BeginDate   *time.Time    `json:"begin_date"`
	EndDate     *time.Time    `json:"end_date"`
	Rules       *VoucherRules `json:"rules"`
	Notes       *string       `json:"notes"`
	CurrentUser string        `json:"-"` // operator passed to the internal accounting service
}

type VoucherFilter struct {
	Type        VoucherType   `json:"type" form:"type"`
	Name        string        `json:"name" form:"name"`
	BatchNo     string        `json:"batch_no" form:"batch_no"`
	TargetType  string        `json:"target_type" form:"target_type"`
	Status      VoucherStatus `json:"status" form:"status"`
	Search      string        `json:"search" form:"search"`
	Per         int           `json:"per" form:"per"`
	Page        int           `json:"page" form:"page"`
	CurrentUser string        `json:"-" form:"-"` // operator passed to the internal accounting service
}

type VoucherBillReq struct {
	CurrentUser  string    `json:"-"`
	TargetUUID   string    `json:"-"`
	VoucherNo    string    `json:"voucher_no" form:"voucher_no" binding:"required"`
	StartDate    string    `json:"start_date" form:"start_date" binding:"required"`
	EndDate      string    `json:"end_date" form:"end_date" binding:"required"`
	Scene        SceneType `json:"scene" form:"scene"`
	InstanceName string    `json:"instance_name" form:"instance_name"`
}

type VoucherBillGroupedRes struct {
	Scene      SceneType `json:"scene"`
	CustomerID string    `json:"customer_id"`
	Value      float64   `json:"value"`
}

type VoucherDashboardReq struct {
	CurrentUser string `json:"-"`
	TargetUUID  string `json:"-"`
	ClusterID   string `json:"cluster_id" form:"cluster_id"`
	XPUModel    string `json:"xpu_model" form:"xpu_model"`
}

type VoucherDashboardStatusItem struct {
	Status VoucherStatus `json:"status"`
	Total  float64       `json:"total"`
	Count  int           `json:"count"`
	Used   float64       `json:"used"`
}

type VoucherDashboardResp struct {
	Items []VoucherDashboardStatusItem `json:"items"`
}

type VoucherNamespaceFilter struct {
	CurrentUser string        `json:"-"`
	TargetUUID  string        `json:"-"`
	Status      VoucherStatus `json:"status" form:"status"`
	Per         int           `json:"per" form:"per"`
	Page        int           `json:"page" form:"page"`
}
