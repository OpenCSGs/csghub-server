//go:build !saas && !ee

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
)

func NeedPhoneVerified(config *config.Config) gin.HandlerFunc {
	return MustLogin()
}

func MustUserOrgApiKey(config *config.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authType := httpbase.GetAuthType(ctx)
		apikey := httpbase.GetAccessToken(ctx)
		currentNamespaceUUID := httpbase.GetCurrentNamespaceUUID(ctx)
		if authType != httpbase.AuthTypeUserOrgApiKey || currentNamespaceUUID == "" || apikey == "" {
			httpbase.UnauthorizedError(ctx, errorx.ErrUnauthorized)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
