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
	UserID    string    `json:"user_id"`
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
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	ReasonCode int       `json:"reason_code"`
	ReasonMsg  string    `json:"reason_msg"`
}

type RECHARGE_REQ struct {
	Value float64 `json:"value"`
	OpUID int64   `json:"op_uid"`
}
