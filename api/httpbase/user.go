package httpbase

import "github.com/gin-gonic/gin"

const (
	CurrentUserCtxVar   = "currentUser"
	CurrentUserQueryVar = "current_user"
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
