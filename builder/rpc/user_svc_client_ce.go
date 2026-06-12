//go:build !saas && !ee

package rpc

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/common/types"
)

// OAuthExchangeToken is not available in the CE edition.
func (c *UserSvcHttpClient) OAuthExchangeToken(_ context.Context, _ *types.OAuthExchangeTokenReq) (*types.OAuthExchangeTokenResp, error) {
	return nil, errors.New("not implemented")
}
