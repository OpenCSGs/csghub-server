package types

import (
	"time"

	"github.com/stripe/stripe-go/v82"
)

var (
	StripeStatusPaid     = "paid"
	StripeStatusComplete = "complete"
)

type CreateStripeSessionReq struct {
	UserUUID    string `json:"-"`
	Lang        string `json:"-"`
	AmountTotal int64  `json:"amount_total" binding:"required,min=400"`
	SuccessUrl  string `json:"success_url" binding:"required"`
	CancelUrl   string `json:"cancel_url" binding:"required"`
}

type CreateStripeSessionRes struct {
	ID          int64  `json:"id"`
	SessionID   string `json:"session_id"`
	AmountTotal int64  `json:"amount_total"`
	Currency    string `json:"currency"`
	URL         string `json:"url"`
	SuccessURL  string `json:"success_url"`
	CancelURL   string `json:"cancel_url"`
}

type StripeSessionReq struct {
	UserUUID  string `json:"-"`
	SessionID string `json:"session_id"`
}

type StripeWebhookReq struct {
	Event stripe.Event
}

type StripeSessionGetReq struct {
	CurrentUser string `json:"-"`
	UserUUID    string `json:"-"`
	ID          int64  `json:"id"`
}

type StripeSessionCloseReq = StripeSessionGetReq

type StripeSessionRes struct {
	ID                 int64     `json:"id"`
	ClientReferenceID  string    `json:"client_reference_id"`
	UserUUID           string    `json:"user_uuid"`
	AmountTotal        int64     `json:"amount_total"`
	Currency           string    `json:"currency"`
	SessionID          string    `json:"session_id"`
	SessionStatus      string    `json:"session_status"`
	PaymentStatus      string    `json:"payment_status"`
	SessionCreatedAt   time.Time `json:"session_created_at"`
	SessionCompletedAt time.Time `json:"session_completed_at"`
	SessionExpiresAt   time.Time `json:"session_expires_at"`
	Mode               string    `json:"mode"`
	LiveMode           bool      `json:"live_mode"`
	PaymentIntentID    string    `json:"payment_intent_id"`
	Url                string    `json:"url"`
	SuccessURL         string    `json:"success_url"`
	CancelURL          string    `json:"cancel_url"`
}

type StripeSessionListReq struct {
	CurrentUser     string
	CurrentUserUUID string
	QueryUserUUID   string
	Per             int
	Page            int
	SessionStatus   string
	StartDate       string
	EndDate         string
}

type StripeSessionAllRes struct {
	TotalAmount int64              `json:"total_amount"`
	TotalCount  int                `json:"total"`
	Data        []StripeSessionRes `json:"data"`
}
