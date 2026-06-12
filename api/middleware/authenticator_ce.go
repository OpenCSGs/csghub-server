//go:build !saas && !ee

package middleware

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
)

// isValidOAuthToken always returns false because OAuth token exchange is not supported in CE.
func isValidOAuthToken(_ *gin.Context, _ rpc.UserSvcClient, _ string) bool {
	return false
}

// NeedPhoneVerified falls back to the default login requirement in CE.
func NeedPhoneVerified(config *config.Config) gin.HandlerFunc {
	return MustLogin()
}

// MustUserOrgApiKey validates the auth context for CE requests.
func MustUserOrgApiKey(config *config.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authType := httpbase.GetAuthType(ctx)
		apikey := httpbase.GetAccessToken(ctx)
		tokenName := httpbase.GetCurrentTokenName(ctx)
		currentNamespaceUUID := httpbase.GetCurrentNamespaceUUID(ctx)
		if len(currentNamespaceUUID) < 1 {
			currentNamespaceUUID = httpbase.GetCurrentUserUUID(ctx)
			httpbase.SetCurrentNamespaceUUID(ctx, currentNamespaceUUID)
		}
		if authType != httpbase.AuthTypeUserOrgApiKey {
			slog.ErrorContext(ctx.Request.Context(), "invalid auth type", slog.Any("authType", authType), slog.Any("nsuuid", currentNamespaceUUID), slog.String("tokenName", tokenName))
			httpbase.UnauthorizedError(ctx, fmt.Errorf("token %s invalid auth type", tokenName))
			ctx.Abort()
			return
		}
		if len(currentNamespaceUUID) < 1 || len(apikey) < 1 {
			slog.ErrorContext(ctx.Request.Context(), "invalid token",
				slog.Any("nsuuid", currentNamespaceUUID), slog.String("tokenName", tokenName))
			httpbase.UnauthorizedError(ctx, fmt.Errorf("token %s invalid", tokenName))
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}
