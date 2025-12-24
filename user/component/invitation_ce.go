//go:build !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type invitationComponentImpl struct {
}

func NewInvitationComponent(_ *config.Config) (InvitationComponent, error) {
	return &invitationComponentImpl{}, nil
}

func (c *invitationComponentImpl) CreateInvitation(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *invitationComponentImpl) GetInvitationByInviteCode(_ context.Context, _ string) (*types.Invitation, error) {
	return &types.Invitation{}, nil
}

func (c *invitationComponentImpl) GetInvitationByUserUUID(_ context.Context, _ string) (*types.Invitation, error) {
	return &types.Invitation{}, nil
}

func (c *invitationComponentImpl) ListInvitationActivities(_ context.Context, _ types.InvitationActivityFilter) ([]types.InvitationActivity, int, error) {
	return nil, 0, nil
}

func (c *invitationComponentImpl) AwardCreditToInvitee(_ context.Context, _ types.AwardCreditToInviteeReq) error {
	return nil
}

func (c *invitationComponentImpl) AwardCreditToInviter(_ context.Context, _ int64) error {
	return nil
}
