package types

import "time"

type ActivityStatus string

const (
	ActivityStatusActive  ActivityStatus = "active"
	ActivityStatusExpired ActivityStatus = "expired"
)

type ActivityReq struct {
	ID              int64         `json:"id" binding:"min=1"`                   // activate ID
	Value           float64       `json:"value" binding:"min=0"`                // charge credit number for activity
	OpUID           string        `json:"op_uid" binding:"required"`            // operator id
	OpDesc          string        `json:"desc" binding:"required"`              // activate description
	ValidDuration   time.Duration `json:"valid_duration" binding:"omitempty"`   // valid duration
	ParticipantUUID string        `json:"participant_uuid" binding:"omitempty"` // participant UUID
}

// NewUserPresentReq is the caller input for the new-user registration present.
// The accounting service fixes the activity itself, so callers only supply the
// operator identity and the description recorded on the present; they cannot
// select another activity or choose the granted amount.
type NewUserPresentReq struct {
	OpUID string `json:"op_uid" binding:"required"` // operator id
}

var StarShipNewUser = ActivityReq{
	ID:     1001,
	Value:  0,  // 0 credit
	OpUID:  "", // fill in user name
	OpDesc: "create starship access token for first time",
}

// expired present
var AutoHubNewUser = ActivityReq{
	ID:     1002,
	Value:  0,  // 0 credit
	OpUID:  "", // fill in user name
	OpDesc: "use autohub for first time",
}

var InviteeCredit = ActivityReq{
	ID:              1003,
	Value:           9900, // 99 CNY, can be updated by config
	OpUID:           "",   // fill in user name
	OpDesc:          "present credit to invitee",
	ValidDuration:   90 * 24 * time.Hour, // 90 days
	ParticipantUUID: "",                  // fill in inviter UUID
}

var InviterCredit = ActivityReq{
	ID:              1004,
	Value:           9900, // 99 CNY, can be updated by config
	OpUID:           "",   // fill in user name
	OpDesc:          "present credit to inviter",
	ValidDuration:   90 * 24 * time.Hour, // 90 days
	ParticipantUUID: "",                  // fill in invitee UUID
}

var CSGHubNewUser = ActivityReq{
	ID:     1005,
	Value:  0,  // 0 credit
	OpUID:  "", // fill in user name
	OpDesc: "register csghub new user and fill in phone and email",
}
