package types

import (
	"time"
)

type VoucherStatus string

const (
	VoucherStatusPending VoucherStatus = "pending"
	VoucherStatusActive  VoucherStatus = "active"
	VoucherStatusExpired VoucherStatus = "expired"
	VoucherStatusRevoked VoucherStatus = "revoked"
)

type VoucherMatchType string

const (
	VoucherMatchTypeBoth    VoucherMatchType = "both"
	VoucherMatchTypeXPU     VoucherMatchType = "xpu"
	VoucherMatchTypeCluster VoucherMatchType = "cluster"
	VoucherMatchTypeNone    VoucherMatchType = "none"
)

type VoucherRules struct {
	ClusterIDs []string `json:"cluster_ids"`
	XPUModels  []string `json:"xpu_models"`
}

type VoucherFilter struct {
	TargetType string        `json:"target_type" form:"target_type"`
	Status     VoucherStatus `json:"status" form:"status"`
	Search     string        `json:"search" form:"search"`
	Per        int           `json:"per" form:"per"`
	Page       int           `json:"page" form:"page"`
}

type CreateVoucherReq struct {
	TargetUUID string         `json:"target_uuid" binding:"required"`
	Total      float64        `json:"total" binding:"required,min=1"`
	BeginDate  time.Time      `json:"begin_date" binding:"required"`
	EndDate    time.Time      `json:"end_date" binding:"required"`
	Rules      []VoucherRules `json:"rules" binding:"required"`
	Notes      string         `json:"notes"`
	IssueUUID  string         `json:"-"`
	IssueName  string         `json:"-"`
}

type UpdateVoucherReq struct {
	Total     *float64       `json:"total"`
	BeginDate *time.Time     `json:"begin_date"`
	EndDate   *time.Time     `json:"end_date"`
	Rules     []VoucherRules `json:"rules"`
	Notes     *string        `json:"notes"`
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
