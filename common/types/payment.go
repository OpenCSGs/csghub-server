package types

import (
	"opencsg.com/csghub-server/common/utils/payment/consts"
	"time"
)

type PaymentNotifyEvent struct {
	PaymentUUID   string    `json:"payment_uuid"`
	TransactionNo string    `json:"transaction_no"`
	OrderNo       string    `json:"order_no"`
	Channel       string    `json:"channel"`
	Paid          bool      `json:"paid"`
	Reversed      bool      `json:"reversed"`
	TimePaid      time.Time `json:"time_paid"`
	TimeExpire    time.Time `json:"time_expire"`
	Amount        int64     `json:"amount"`
	Extra         string    `json:"extra"`
}

type CreatePaymentReq struct {
	OrderNo string                `json:"order_no"`
	Amount  float64               `json:"amount"` //unit yuan
	Channel consts.PaymentChannel `json:"channel"`
	Subject string                `json:"subject"`
	Body    string                `json:"body"`
	Extra   string                `json:"extra"`
}

type CreatePaymentResp struct {
	PaymentUUID string `json:"payment_uuid"`
	OrderNo     string `json:"order_no"`
	CodeUrl     string `json:"code_url"`
}
