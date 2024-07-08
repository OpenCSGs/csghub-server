package types

import (
	"time"

	"github.com/google/uuid"
)

type ACCTStatus int

var (
	ACCTSuccess       ACCTStatus = 0 // charge success
	ACCTInvalidFormat ACCTStatus = 1 // invalid event data format
	ACCTChargeFail    ACCTStatus = 2 // fail to charge user fee
	ACCTLackBalance   ACCTStatus = 3 // balance <= 0
	ACCTDuplicated    ACCTStatus = 4 // duplicated charge by event uuid
)

type SKUType int

var (
	SKUReserve  SKUType = 0 // system reserve
	SKUCSGHub   SKUType = 1 // csghub server
	SKUStarship SKUType = 2 // starship
)

type SceneType int

var (
	SceneReserve        SceneType = 0  // system reserve
	ScenePortalCharge   SceneType = 1  // portal charge fee
	SceneModelInference SceneType = 10 // model inference endpoint
	SceneSpace          SceneType = 11 // csghub space
	SceneModelFinetune  SceneType = 12 // model finetune
	SceneMultiSync      SceneType = 13 // multi sync
	SceneStarship       SceneType = 20 // starship
	SceneUnknow         SceneType = 99 // unknow
)

var (
	TimeDurationMinType int = 0
	TokenNumberType     int = 1
	QuotaNumberType     int = 2
)

type ACCT_EVENT_REQ struct {
	EventUUID    uuid.UUID `json:"event_uuid"`
	UserUUID     string    `json:"user_uuid"`
	Value        float64   `json:"value"`
	Scene        SceneType `json:"scene"`
	OpUID        string    `json:"op_uid"`
	CustomerID   string    `json:"customer_id"`
	EventDate    time.Time `json:"event_date"`
	Price        float64   `json:"price"`
	PriceUnit    string    `json:"price_unit"`
	Consumption  float64   `json:"consumption"`
	ValueType    int       `json:"value_type"`
	ResourceID   string    `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	SkuID        int64     `json:"sku_id"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// generate charge event from client
type ACCT_EVENT struct {
	Uuid         uuid.UUID `json:"uuid"`       // event uuid
	UserUUID     string    `json:"user_uuid"`  // user uuid
	Value        int64     `json:"value"`      // time duration in minutes or token number
	ValueType    int       `json:"value_type"` // 0: credit, 1: token
	Scene        int       `json:"scene"`
	OpUID        string    `json:"op_uid"`        // operator uuid
	ResourceID   string    `json:"resource_id"`   // resource id
	ResourceName string    `json:"resource_name"` // resource name
	CustomerID   string    `json:"customer_id"`   // customer_id will be shown in bill
	CreatedAt    time.Time `json:"created_at"`    // time of event happen
	Extra        string    `json:"extra"`
}

// notify response to client
type ACCT_NOTIFY struct {
	Uuid       uuid.UUID `json:"uuid"`
	UserUUID   string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	ReasonCode int       `json:"reason_code"`
	ReasonMsg  string    `json:"reason_msg"`
}

type ACCT_STATEMENTS_REQ struct {
	CurrentUser  string `json:"current_user"`
	UserUUID     string `json:"user_uuid"`
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
	OpUID       string    `json:"op_uid"`
	CreatedAt   time.Time `json:"created_at"`
	CustomerID  string    `json:"instance_name"`
	EventDate   time.Time `json:"event_date"`
	Price       float64   `json:"price"`
	PriceUnit   string    `json:"price_unit"`
	Consumption float64   `json:"consumption"`
}

type RECHARGE_REQ struct {
	Value float64 `json:"value"`
	OpUID int     `json:"op_uid"`
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
	RepoPath     string    `json:"repo_path"`
	DeployID     int64     `json:"deploy_id"`
	DeployName   string    `json:"deploy_name"`
	DeployUser   string    `json:"deploy_user"`
}

type BILLS struct {
	Data []ITEM `json:"data"`
	ACCT_SUMMARY
}

type ACCT_STATEMENTS_RESULT struct {
	Data []ACCT_STATEMENTS_RES `json:"data"`
	ACCT_SUMMARY
}

type METERING_EVENT struct {
	Uuid         uuid.UUID `json:"uuid"`       // event uuid
	UserUUID     string    `json:"user_uuid"`  // user uuid
	Value        int64     `json:"value"`      // time duration in minutes or token number
	ValueType    int       `json:"value_type"` // 0: duration, 1: token
	Scene        int       `json:"scene"`
	OpUID        string    `json:"op_uid"`        // operator uuid
	ResourceID   string    `json:"resource_id"`   // resource id
	ResourceName string    `json:"resource_name"` // resource name
	CustomerID   string    `json:"customer_id"`   // customer_id will be shown in bill
	CreatedAt    time.Time `json:"created_at"`    // time of event happen
	Extra        string    `json:"extra"`
}

type ACCT_PRICE struct {
	SkuType    int    `json:"sku_type"`
	SkuPrice   int64  `json:"sku_price"`
	SkuUnit    int64  `json:"sku_unit"`
	SkuDesc    string `json:"sku_desc"`
	ResourceID string `json:"resource_id"`
}

type ACCT_PRICE_REQ struct {
	SKUType    SKUType   `json:"sku_type"`
	ResourceID string    `json:"resource_id"`
	PriceTime  time.Time `json:"price_time"`
	Per        int       `json:"per"`
	Page       int       `json:"page"`
}
