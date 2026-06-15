//go:build !saas && !ee

package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/types"
)

func TestUserSvcHttpClient_OAuthExchangeToken_CE(t *testing.T) {
	client := NewUserSvcHttpClient("http://user-service")

	resp, err := client.OAuthExchangeToken(context.Background(), &types.OAuthExchangeTokenReq{
		AccessToken: "casdoor-oauth-token",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
}
