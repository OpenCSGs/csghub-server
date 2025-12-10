package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type InvitationComponent interface {
	CreateInvitation(ctx context.Context, userUUID string) (string, error)
	GetInvitationByUserUUID(ctx context.Context, userUUID string) (*types.Invitation, error)
	GetInvitationByInviteCode(ctx context.Context, invitationCode string) (*types.Invitation, error)
	ListInvitationActivities(ctx context.Context, req types.InvitationActivityFilter) ([]types.InvitationActivity, int, error)
	AwardCreditToInvitee(ctx context.Context, req types.AwardCreditToInviteeReq) error
	AwardCreditToInviter(ctx context.Context, activityID int64) error
}
