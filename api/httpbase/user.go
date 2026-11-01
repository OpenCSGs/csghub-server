package httpbase

import (
	"context"

	"github.com/gin-gonic/gin"
)

const (
	CurrentUserCtxVar         = "currentUser"
	CurrentUserUUIDCtxVar     = "currentUserUUID"
	CurrentNamespaceUUIDVar   = "currentNamespaceUUID"
	AccessTokenCtxVar         = "accessToken"
	AuthTypeCtxVar            = "authType"
	CurrentUserQueryVar       = "current_user"
	CurrentUserUUIDQueryVar   = "current_user_uuid"
	HeaderLanguageKey         = "Accept-Language"
	AccessTokenNameCtxVar     = "accessTokenName"
	IPctxVar                  = "ip_address"
	APIKeyModelScopeCtxVar    = "ApiKeyModelScope"
	APIKeyPriorityScopeCtxVar = "ApiKeyPriorityScope"
	AccessTokenIDCtxVar       = "accessTokenID"
	// CurrentUserPriorityCtxVar carries the authenticated user's upstream
	// channel priority, resolved by the gateway user-tags middleware. It is
	// set only for restricted users (those carrying a routing tag); a
	// missing value means unrestricted.
	CurrentUserPriorityCtxVar = "currentUserPriority"
)

type AuthType string

const (
	AuthTypeSystemApiKey   AuthType = "ApiKey"
	AuthTypeJwt            AuthType = "JWT"
	AuthTypeAccessToken    AuthType = "AccessToken"
	AuthTypeMultiSyncToken AuthType = "MultiSyncToken"
	AuthTypeUserOrgApiKey  AuthType = "UserOrgApiKey"
	AuthTypeGitAccessToken AuthType = "GitAccessToken"
)

// GetCurrentUser returns the current user name from the context.
//
// user name could be previously set by parsing query string or jwt token
func GetCurrentUser(ctx *gin.Context) string {
	return ctx.GetString(CurrentUserCtxVar)
}

// SetCurrentUser preserves the authenticated username in both transport and request contexts.
func SetCurrentUser(ctx *gin.Context, user string) {
	setAuditContextValue(ctx, CurrentUserCtxVar, user)
}

func GetAccessToken(ctx *gin.Context) string {
	return ctx.GetString(AccessTokenCtxVar)
}

// SetAccessToken preserves the authenticated token in both contexts for existing audit consumers.
func SetAccessToken(ctx *gin.Context, token string) {
	setAuditContextValue(ctx, AccessTokenCtxVar, token)
}

func GetAuthType(ctx *gin.Context) AuthType {
	return AuthType(ctx.GetString(AuthTypeCtxVar))
}

// SetAuthType preserves the authentication method in both transport and request contexts.
func SetAuthType(ctx *gin.Context, t AuthType) {
	setAuditContextValue(ctx, AuthTypeCtxVar, string(t))
}

func GetCurrentUserUUID(ctx *gin.Context) string {
	return ctx.GetString(CurrentUserUUIDCtxVar)
}

// SetCurrentUserUUID preserves the authenticated operator UUID in both contexts.
func SetCurrentUserUUID(ctx *gin.Context, userUUID string) {
	setAuditContextValue(ctx, CurrentUserUUIDCtxVar, userUUID)
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

// SetIPAddress preserves the resolved client IP in both contexts for audit attribution.
func SetIPAddress(ctx *gin.Context, ip string) {
	setAuditContextValue(ctx, IPctxVar, ip)
}

// setAuditContextValue keeps existing Gin readers and request-context audit readers consistent.
// String keys preserve the audit hook's existing contract; a missing request still permits Gin-only use.
// Deriving from the current request context retains authorization, tracing and cancellation.
func setAuditContextValue(ctx *gin.Context, key, value string) {
	ctx.Set(key, value)
	if ctx.Request != nil {
		ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), key, value))
	}
}

func GetIPAddress(ctx *gin.Context) string {
	return ctx.GetString(IPctxVar)
}

func SetAPIKeyModelScope(ctx *gin.Context, scope string) {
	ctx.Set(APIKeyModelScopeCtxVar, scope)
}

func GetAPIKeyModelScope(ctx *gin.Context) string {
	return ctx.GetString(APIKeyModelScopeCtxVar)
}

func SetAPIKeyPriorityScope(ctx *gin.Context, scope string) {
	ctx.Set(APIKeyPriorityScopeCtxVar, scope)
}

func GetAPIKeyPriorityScope(ctx *gin.Context) string {
	return ctx.GetString(APIKeyPriorityScopeCtxVar)
}

func GetCurrentTokenID(ctx *gin.Context) int64 {
	return ctx.GetInt64(AccessTokenIDCtxVar)
}

func SetCurrentTokenID(ctx *gin.Context, id int64) {
	ctx.Set(AccessTokenIDCtxVar, id)
}

// SetCurrentUserPriority preserves the authenticated user's upstream channel
// priority in the gin context. It is set only for restricted users; 0 is a
// meaningful tier (the low-priority channel), so presence carries the
// "restricted" bit.
func SetCurrentUserPriority(ctx *gin.Context, priority int) {
	ctx.Set(CurrentUserPriorityCtxVar, priority)
}

// GetCurrentUserPriority returns the authenticated user's channel priority,
// or nil when the user is unrestricted (no routing tag resolved).
func GetCurrentUserPriority(ctx *gin.Context) *int {
	value, ok := ctx.Get(CurrentUserPriorityCtxVar)
	if !ok {
		return nil
	}
	priority, ok := value.(int)
	if !ok {
		return nil
	}
	return &priority
}
