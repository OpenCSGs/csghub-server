package httpbase

import (
	"github.com/gin-gonic/gin"
)

const (
	CurrentUserCtxVar       = "currentUser"
	CurrentUserUUIDCtxVar   = "currentUserUUID"
	CurrentNamespaceUUIDVar = "currentNamespaceUUID"
	AccessTokenCtxVar       = "accessToken"
	AuthTypeCtxVar          = "authType"
	CurrentUserQueryVar     = "current_user"
	CurrentUserUUIDQueryVar = "current_user_uuid"
	HeaderLanguageKey       = "Accept-Language"
	AccessTokenNameCtxVar   = "accessTokenName"
)

type AuthType string

const (
	AuthTypeSystemApiKey   AuthType = "ApiKey"
	AuthTypeJwt            AuthType = "JWT"
	AuthTypeAccessToken    AuthType = "AccessToken"
	AuthTypeMultiSyncToken AuthType = "MultiSyncToken"
	AuthTypeUserOrgApiKey  AuthType = "UserOrgApiKey"
)

// GetCurrentUser returns the current user name from the context.
//
// user name could be previously set by parsing query string or jwt token
func GetCurrentUser(ctx *gin.Context) string {
	return ctx.GetString(CurrentUserCtxVar)
}

func SetCurrentUser(ctx *gin.Context, user string) {
	ctx.Set(CurrentUserCtxVar, user)
}

func GetAccessToken(ctx *gin.Context) string {
	return ctx.GetString(AccessTokenCtxVar)
}

func SetAccessToken(ctx *gin.Context, user string) {
	ctx.Set(AccessTokenCtxVar, user)
}

func GetAuthType(ctx *gin.Context) AuthType {
	return AuthType(ctx.GetString(AuthTypeCtxVar))
}

func SetAuthType(ctx *gin.Context, t AuthType) {
	ctx.Set(AuthTypeCtxVar, string(t))
}

func GetCurrentUserUUID(ctx *gin.Context) string {
	return ctx.GetString(CurrentUserUUIDCtxVar)
}

func SetCurrentUserUUID(ctx *gin.Context, userUUID string) {
	ctx.Set(CurrentUserUUIDCtxVar, userUUID)
}

func GetCurrentNamespaceUUID(ctx *gin.Context) string {
	return ctx.GetString(CurrentNamespaceUUIDVar)
}

func SetCurrentNamespaceUUID(ctx *gin.Context, namespaceUUID string) {
	ctx.Set(CurrentNamespaceUUIDVar, namespaceUUID)
}

func GetCurrentUserLanguage(ctx *gin.Context) string {
	return ctx.GetHeader(HeaderLanguageKey)
}

func GetCurrentTokenName(ctx *gin.Context) string {
	return ctx.GetString(AccessTokenNameCtxVar)
}

func SetCurrentTokenName(ctx *gin.Context, name string) {
	ctx.Set(AccessTokenNameCtxVar, name)
}
