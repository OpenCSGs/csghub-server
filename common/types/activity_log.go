package types

import "time"

type ActivityLog struct {
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	Action        string    `json:"action"`
	AuthType      string    `json:"auth_type"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    int64     `json:"resource_id"`
	ResourceName  string    `json:"resource_name"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent"`
	OperationTime time.Time `json:"operation_time"`
}

type QueryActivityLogReq struct {
	After time.Time `json:"after" form:"after"`
	Per   int       `json:"per" form:"per"`
	Page  int       `json:"page" form:"page"`
}
