package types

import (
	"time"

	"github.com/google/uuid"
)

type ACCTStatus int

var (
	UnitMinute string = "minute"
	UnitToken  string = "token"
	UnitRepo   string = "repository"
	UnitByte   string = "byte"
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
