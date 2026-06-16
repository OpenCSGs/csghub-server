//go:build !ee && !saas

package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// OAuthExchangeToken validates a Casdoor JWT access token and returns a
// locally issued JWT for the mapped user.
func (c *userComponentImpl) OAuthExchangeToken(ctx context.Context, req *types.OAuthExchangeTokenReq) (*types.OAuthExchangeTokenResp, error) {
	return nil, errors.New("not implemented")
}

func (c *userComponentImpl) processAwardSelfRegisterCredit(user *database.User) {
}
