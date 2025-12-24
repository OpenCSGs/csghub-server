package types

import (
	"fmt"
	"time"
)

type CreateInvitationResp struct {
	InviteCode string `json:"invite_code"`
}

type Invitation struct {
	ID            int64   `json:"id"`
	InviteCode    string  `json:"invite_code"`
	UserUUID      string  `json:"user_uuid"`
	Invites       int64   `json:"invites"`
	TotalCredit   float64 `json:"total_credit"`
	PendingCredit float64 `json:"pending_credit"`
}

type CreateInvitationActivityReq struct {
	InviterUUID         string    `json:"inviter_uuid"`
	InviteCode          string    `json:"invite_code"`
	InviteeUUID         string    `json:"invitee_uuid"`
	InviteeName         string    `json:"invitee_name"`
	RegisterAt          time.Time `json:"register_at"`
	InviterCreditAmount float64   `json:"inviter_credit_amount"`
	InviteeCreditAmount float64   `json:"invitee_credit_amount"`
	AwardAt             time.Time `json:"award_at"`
}

type InvitationActivityStatus string

const (
	InvitationActivityStatusPending InvitationActivityStatus = "pending"
	InvitationActivityStatusAwarded InvitationActivityStatus = "awarded"
	InvitationActivityStatusFailed  InvitationActivityStatus = "failed"
)

func (s InvitationActivityStatus) String() string {
	return string(s)
}

func (s InvitationActivityStatus) Validate() error {
	switch s {
	case InvitationActivityStatusPending, InvitationActivityStatusAwarded, InvitationActivityStatusFailed:
		return nil
	default:
		return fmt.Errorf("invalid invitation activity status: %s", s)
	}
}

type InvitationActivityFilter struct {
	InviterUUID   string                   `json:"inviter_uuid"`
	InviterStatus InvitationActivityStatus `json:"inviter_status" binding:"omitempty,oneof=pending awarded failed"`
	InviteeStatus InvitationActivityStatus `json:"invitee_status" binding:"omitempty,oneof=pending awarded failed"`
	StartDate     string                   `json:"start_date"`
	EndDate       string                   `json:"end_date"`
	Per           int                      `json:"per"`
	Page          int                      `json:"page"`
}

type InvitationActivity struct {
	ID                  int64                    `json:"id"`
	InviterUUID         string                   `json:"inviter_uuid"`
	InviteCode          string                   `json:"invite_code"`
	InviteeUUID         string                   `json:"invitee_uuid"`
	InviteeName         string                   `json:"invitee_name"`
	RegisterAt          time.Time                `json:"register_at"`
	InviterCreditAmount float64                  `json:"inviter_credit_amount"`
	InviteeCreditAmount float64                  `json:"invitee_credit_amount"`
	InviterStatus       InvitationActivityStatus `json:"inviter_status"`
	InviteeStatus       InvitationActivityStatus `json:"invitee_status"`
	AwardAt             time.Time                `json:"award_at"`
}

type AwardCreditToInviteeReq struct {
	InviteeUUID string    `json:"invitee_uuid" binding:"required"`
	InviteeName string    `json:"invitee_name" binding:"required"`
	RegisterAt  time.Time `json:"register_at" binding:"required"`
}

type AwardCreditToInviterReq struct {
	ActivityID int64 `json:"activity_id" binding:"required"`
}
