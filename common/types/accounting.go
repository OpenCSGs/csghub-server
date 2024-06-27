package types

import (
	"time"

	"github.com/google/uuid"
)

var (
	REASON_SUCCESS        = 0 // charge success
	REASON_INVALID_FORMAT = 1 // invalid event data format
	REASON_CHARGE_FAIL    = 2 // fail to charge user fee
	REASON_LACK_BALANCE   = 3 // balance <= 0
	REASON_DUPLICATED     = 4 // duplicated charge
)

// generate charge event from client
type ACC_EVENT struct {
	Uuid      uuid.UUID `json:"uuid"`
	UserUUID  string    `json:"user_id"`
	Value     float64   `json:"value"`
	ValueType int       `json:"value_type"`
	Scene     int       `json:"scene"`
	OpUID     int64     `json:"op_uid"`
	CreatedAt time.Time `json:"created_at"`
	Extra     string    `json:"extra"`
}

type ACC_EVENT_EXTRA struct {
	CustomerID       string  `json:"customer_id"`
	CustomerPrice    float64 `json:"customer_price"`
	PriceUnit        string  `json:"price_unit"`
	CustomerDuration float64 `json:"customer_duration"`
}

// notify response to client
type ACC_NOTIFY struct {
	Uuid       uuid.UUID `json:"uuid"`
	UserUUID   string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	ReasonCode int       `json:"reason_code"`
	ReasonMsg  string    `json:"reason_msg"`
}

type ACCT_STATEMENTS_REQ struct {
	CurrentUser  string `json:"current_user"`
	UserUUID     string `json:"user_id"`
	Scene        int    `json:"scene"`
	InstanceName string `json:"instance_name"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Per          int    `json:"per"`
	Page         int    `json:"page"`
}

type ACCT_BILLS_REQ struct {
	UserUUID  string `json:"user_id"`
	Scene     int    `json:"scene"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Per       int    `json:"per"`
	Page      int    `json:"page"`
}

type ACCT_STATEMENTS_RES struct {
	ID          int64     `json:"id"`
	EventUUID   uuid.UUID `json:"event_uuid"`
	UserUUID    string    `json:"user_id"`
	Value       float64   `json:"value"`
	Scene       int       `json:"scene"`
	OpUID       int64     `json:"op_uid"`
	CreatedAt   time.Time `json:"created_at"`
	CustomerID  string    `json:"instance_name"`
	EventDate   time.Time `json:"event_date"`
	Price       float64   `json:"price"`
	PriceUnit   string    `json:"price_unit"`
	Consumption float64   `json:"consumption"`
}

type RECHARGE_REQ struct {
	Value float64 `json:"value"`
	OpUID int64   `json:"op_uid"`
}

type ACCT_QUOTA_REQ struct {
	RepoCountLimit int64 `json:"repo_count_limit"`
	SpeedLimit     int64 `json:"speed_limit"`
	TrafficLimit   int64 `json:"traffic_limit"`
}

type ACCT_QUOTA_STATEMENT_REQ struct {
	RepoPath string `json:"repo_path"`
	RepoType string `json:"repo_type"`
}

type ACCT_SUMMARY struct {
	Total            int     `json:"total"`
	TotalValue       float64 `json:"total_value"`
	TotalConsumption float64 `json:"total_consumption"`
}

type ITEM struct {
	Consumption  float64   `json:"consumption"`
	InstanceName string    `json:"instance_name"`
	Value        float64   `json:"value"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`
}

type BILLS struct {
	Data []ITEM `json:"data"`
	ACCT_SUMMARY
}

type ACCT_STATEMENTS_RESULT struct {
	Data []ACCT_STATEMENTS_RES `json:"data"`
	ACCT_SUMMARY
}
