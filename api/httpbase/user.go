package httpbase

import "github.com/gin-gonic/gin"

const (
	CurrentUserCtxVar   = "currentUser"
	AccessTokenCtxVar   = "accessToken"
	AuthTypeCtxVar      = "authType"
	CurrentUserQueryVar = "current_user"
)

type AuthType string

const (
	AuthTypeApiKey      AuthType = "ApiKey"
	AuthTypeJwt         AuthType = "JWT"
	AuthTypeAccessToken AuthType = "AccessToken"
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
