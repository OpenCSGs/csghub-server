//go:build !ee && !saas

package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type userEdition struct{}

// manualVerificationUUID preserves the existing manual verification contract in CE.
func (uv *UserVerifyComponentImpl) manualVerificationUUID(_ context.Context, req *types.UserVerifyReq) (string, error) {
	return req.UUID, nil
}

// enrichVerification preserves the existing CE verification response without identity decryption.
func (uv *UserVerifyComponentImpl) enrichVerification(*database.UserVerify) error { return nil }

// authorizeVerificationQuery preserves the existing CE verification access contract.
func (uv *UserVerifyComponentImpl) authorizeVerificationQuery(context.Context, string) error {
	return nil
}

func (c *userEdition) initEdition(*config.Config) error                         { return nil }
func (c *userEdition) enrichProfile(context.Context, *types.User)               {}
func (c *userComponentImpl) afterUserCreated(*database.User)                    {}
func (c *userComponentImpl) afterSSOUserCreated(*database.User)                 {}
func (c *userComponentImpl) afterProfileUpdated(*database.User, *database.User) {}
func (c *userComponentImpl) checkContactUpdateRisk(context.Context, string, string, string) error {
	return nil
}

func (c *userComponentImpl) checkContactCodeRequestRisk(context.Context, string, string, string) error {
	return nil
}

// OAuthExchangeToken validates a Casdoor JWT access token and returns a
// locally issued JWT for the mapped user.
func (c *userComponentImpl) OAuthExchangeToken(ctx context.Context, req *types.OAuthExchangeTokenReq) (*types.OAuthExchangeTokenResp, error) {
	return nil, errors.New("not implemented")
}
