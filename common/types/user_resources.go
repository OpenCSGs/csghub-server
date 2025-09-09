package types

import "time"

type CreateUserResourceReq struct {
	Username     string               `json:"-"`
	UserUID      string               `json:"-"`
	OrderDetails []AcctOrderDetailReq `json:"order_details"`
}

type GetUserResourceReq struct {
	CurrentUser string `json:"current_user"`
	UserUID     string `json:"-"`
	PageOpts
}

type UserResourcesResp struct {
	ID            int64     `json:"id"`
	OrderId       string    `json:"order_id"`
	OrderDetailId int64     `json:"order_detail_id"`
	ResourceId    int64     `json:"resource_id"`
	XPUNum        int       `json:"xpu_num"`
	PayMode       string    `json:"pay_mode"`
	Price         float64   `json:"price"`
	CreatedAt     time.Time `json:"created_at"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Resource      string    `json:"resource"`
	DeployName    string    `json:"deploy_name"`
	RepoPath      string    `json:"repo_path"`
	DeployID      int64     `json:"deploy_id"`
	ResourceType  string    `json:"resource_type"`
	DeployType    int       `json:"deploy_type"`
}
