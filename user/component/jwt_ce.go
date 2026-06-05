//go:build !ee && !saas

package component

import (
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *jwtComponentImpl) loginClaims(u *database.User) *types.JWTClaims {
	return c.buildClaims(u, time.Now().Add(c.ValidTime))
}

func (c *jwtComponentImpl) refreshClaims(req types.RefreshJWTReq, u *database.User) (*types.JWTClaims, error) {
	return c.loginClaims(u), nil
}

func (c *jwtComponentImpl) createClaims(u *database.User) (*types.JWTClaims, error) {
	return c.loginClaims(u), nil
}
