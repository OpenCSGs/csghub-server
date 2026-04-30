package types

import "time"

type QueryAuditLogReq struct {
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	UserName    string     `json:"user_name"`
	Token       string     `json:"token"`
	Action      string     `json:"action"`
	TableName   string     `json:"table_name"`
	AuthType    string     `json:"auth_type"`
	CurrentUser string     `json:"current_user"`
	Per         int        `json:"per"`
	Page        int        `json:"page"`
}
