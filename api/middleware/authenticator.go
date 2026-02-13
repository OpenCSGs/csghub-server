package middleware

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// BuildJwtSession create and save session with jwt from query string
func BuildJwtSession(jwtSignKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("jwt")
		// If no JWT provided, continue with the next middleware
		if token == "" {
			c.Next()
			return
		}
		claims, err := parseJWTToken(jwtSignKey, token)
		if err != nil {
			slog.Debug("fail to parse jwt token", slog.String("token_get", token), slog.Any("error", err))
			httpbase.UnauthorizedError(c, errorx.InvalidAuthHeader(err, nil))
			c.Abort()
			return
		}

		sessions.Default(c).Set(httpbase.CurrentUserCtxVar, claims.CurrentUser)
		sessions.Default(c).Set(httpbase.CurrentUserUUIDCtxVar, claims.UUID)
		err = sessions.Default(c).Save()
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "fail to save session", slog.Any("error", err))
			httpbase.UnauthorizedError(c, errorx.InvalidAuthHeader(err, nil))
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthSession verify user login by session, ans save user name into context if login
func AuthSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userName := session.Get(httpbase.CurrentUserCtxVar)
		uuid := session.Get(httpbase.CurrentUserUUIDCtxVar)
		if userName != nil {
			httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
			httpbase.SetCurrentUser(c, userName.(string))
			httpbase.SetCurrentUserUUID(c, uuid.(string))
		}

		c.Next()
	}
}

func Authenticator(config *config.Config) gin.HandlerFunc {
	svcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userSvcClient := rpc.NewUserSvcHttpClient(svcAddr, rpc.AuthWithApiKey(config.APIToken))
	return func(c *gin.Context) {
		result := isValidBrowserSession(c)
		if result {
			c.Next()
			return
		}

		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		switch {
		case strings.HasPrefix(authHeader, "Bearer "):
			token := strings.TrimPrefix(authHeader, "Bearer ")
			result = isValidApiToken(c, config, token)
			if result {
				c.Next()
				return
			}

			result = isValidJWTToken(c, config, token)
			if result {
				c.Next()
				return
			}

			result = isValidAccessToken(c, userSvcClient, token)
			if result {
				c.Next()
				return
			}

			slog.ErrorContext(c, "invalid Bearer token", slog.String("token", token),
				slog.String("ip", c.ClientIP()),
				slog.String("method", c.Request.Method),
				slog.String("url", c.Request.URL.RequestURI()),
			)
			httpbase.UnauthorizedError(c, errorx.ErrInvalidAuthHeader)
			c.Abort()
		case strings.HasPrefix(authHeader, "Basic "):
			token := strings.TrimPrefix(authHeader, "Basic ")
			result = isValidBasicToken(c, userSvcClient, token)
			if result {
				c.Next()
				return
			}

			slog.ErrorContext(c, "invalid Basic token", slog.String("token", token),
				slog.String("ip", c.ClientIP()),
				slog.String("method", c.Request.Method),
				slog.String("url", c.Request.URL.RequestURI()),
			)
			httpbase.UnauthorizedError(c, errorx.ErrInvalidAuthHeader)
			c.Abort()
		default:
			httpbase.UnauthorizedError(c, errorx.ErrInvalidAuthHeader)
			c.Abort()
		}
	}
}

func isValidBrowserSession(c *gin.Context) bool {
	// check access from UI
	sessionObj, sessionExists := c.Get(sessions.DefaultKey)
	if sessionExists && sessionObj != nil {
		session := sessions.Default(c)
		sessionUserName := session.Get(httpbase.CurrentUserCtxVar)
		if sessionUserName != nil {
			slog.Debug("get username from session", slog.Any("session username", sessionUserName.(string)))
			if len(sessionUserName.(string)) > 0 {
				// login success on UI
				httpbase.SetCurrentUser(c, sessionUserName.(string))
				httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
				return true
			}
		}
	}
	return false
}

func isValidApiToken(c *gin.Context, config *config.Config, token string) bool {
	apiToken := config.APIToken
	if token == apiToken {
		// get current user from query string
		currentUser := c.Query(httpbase.CurrentUserQueryVar)
		if len(currentUser) > 0 {
			httpbase.SetCurrentUser(c, currentUser)
		}
		currentUserUUID := c.Query(httpbase.CurrentUserUUIDQueryVar)
		if len(currentUserUUID) > 0 {
			httpbase.SetCurrentUserUUID(c, currentUserUUID)
		}
		httpbase.SetAuthType(c, httpbase.AuthTypeApiKey)
		return true
	}
	return false
}

func isValidJWTToken(c *gin.Context, config *config.Config, token string) bool {
	if strings.Contains(token, ".") {
		claims, err := parseJWTToken(config.JWT.SigningKey, token)
		if err == nil {
			httpbase.SetCurrentUser(c, claims.CurrentUser)
			httpbase.SetCurrentUserUUID(c, claims.UUID)
			httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
			return true
		} else {
			slog.ErrorContext(c.Request.Context(), "verify jwt token error", slog.Any("error", err))
		}
	}
	return false
}

func isValidAccessToken(c *gin.Context, userSvcClient rpc.UserSvcClient, token string) bool {
	user, err := userSvcClient.VerifyByAccessToken(c.Request.Context(), token)
	if err != nil {
		slog.ErrorContext(c, "verify access token error", slog.Any("error", err))
		return false
	}
	if user != nil {
		if user.Application == types.AccessTokenAppCSGHub {
			httpbase.SetCurrentUser(c, user.Username)
			httpbase.SetCurrentUserUUID(c, user.UserUUID)
			httpbase.SetAccessToken(c, token)
			httpbase.SetAuthType(c, httpbase.AuthTypeAccessToken)
			return true
		} else if user.Application == types.AccessTokenAppMirror {
			httpbase.SetCurrentUser(c, user.Username)
			httpbase.SetCurrentUserUUID(c, user.UserUUID)
			httpbase.SetAccessToken(c, token)
			httpbase.SetAuthType(c, httpbase.AuthTypeMultiSyncToken)
			return true
		}
	}
	return false
}

func parseJWTToken(signKey, tokenString string) (*types.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(signKey), nil
	})
	if err != nil {
		return nil, errorx.ErrInvalidAuthHeader
	}

	if !token.Valid {
		return nil, errorx.ErrInvalidAuthHeader
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if ok {
		return claims, nil
	}
	err = fmt.Errorf("JWT token claims not match: %+v", *token)
	return nil, errorx.InvalidAuthHeader(err, nil)
}

func isValidBasicToken(c *gin.Context, userSvcClient rpc.UserSvcClient, token string) bool {
	var username, accessToken string
	authInfo, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		slog.ErrorContext(c, "Failed to decode basic auth header", slog.Any("token", token), slog.Any("error", err))
		return false
	}
	username = strings.Split(string(authInfo), ":")[0]
	accessToken = strings.Split(string(authInfo), ":")[1]
	user, err := userSvcClient.VerifyByAccessToken(c.Request.Context(), accessToken)
	if err != nil {
		slog.ErrorContext(c, "verify access token error", slog.Any("error", err))
		return false
	}
	if user.Username == username {
		httpbase.SetCurrentUser(c, username)
		httpbase.SetCurrentUserUUID(c, user.UserUUID)
		return true
	}
	return false
}

func NeedAPIKey(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiToken := config.APIToken

		// Get Authorization token
		authHeader := c.Request.Header.Get("Authorization")

		// Check Authorization Header format
		if authHeader == "" {
			slog.Info("missing authorization header", slog.Any("url", c.Request.URL))
			httpbase.UnauthorizedError(c, errorx.ErrInvalidAuthHeader)
			c.Abort()
			return
		}

		// Get token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == apiToken {
			// get current user from query string
			currentUser := c.Query(httpbase.CurrentUserQueryVar)
			if len(currentUser) > 0 {
				httpbase.SetCurrentUser(c, currentUser)
			}
		} else {
			httpbase.UnauthorizedError(c, errorx.ErrNeedAPIKey)
			c.Abort()
			return
		}

		c.Next()
	}
}

func MustLogin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errorx.ErrUserNotFound)
			ctx.Abort()
			return
		}
	}
}

func NeedAdmin(config *config.Config) gin.HandlerFunc {
	userSvcClient := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))

	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errorx.ErrUserNotFound)
			ctx.Abort()
			return
		}

		user, err := userSvcClient.GetUserInfo(ctx, currentUser, currentUser)

		if err != nil {
			httpbase.ServerError(ctx, fmt.Errorf("failed to find user, cause:%w", err))
			ctx.Abort()
			return
		}

		dbUser := &database.User{
			RoleMask: strings.Join(user.Roles, ","),
		}

		if !dbUser.CanAdmin() {
			httpbase.ForbiddenError(ctx, errorx.ErrUserNotAdmin)
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

func UserMatch() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errorx.ErrUserNotFound)
			ctx.Abort()
			return
		}

		userName := ctx.Param("username")
		if userName != currentUser {
			httpbase.UnauthorizedError(ctx, errorx.ErrUserNotMatch)
			slog.ErrorContext(ctx.Request.Context(), "user not match, try to query user account not owned", "currentUser", currentUser, "userName", userName)
			ctx.Abort()
			return
		}
	}
}

func RestrictMultiSyncTokenToRead() gin.HandlerFunc {
	return func(c *gin.Context) {
		authType := httpbase.GetAuthType(c)
		if authType == httpbase.AuthTypeMultiSyncToken {
			method := c.Request.Method
			allowedMethods := map[string]bool{
				"GET":  true,
				"HEAD": true,
			}
			if !allowedMethods[method] {
				slog.WarnContext(c.Request.Context(), "MultiSyncToken attempted write operation",
					slog.String("method", method),
					slog.String("path", c.Request.URL.Path),
					slog.String("user", httpbase.GetCurrentUser(c)),
				)
				httpbase.ForbiddenError(c, errorx.ErrForbidden)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

type MiddlewareCollection struct {
	Auth struct {
		// only can be accessed by api key
		NeedAPIKey gin.HandlerFunc
		// user need to login first
		NeedLogin gin.HandlerFunc
		//user must be admin role to access
		NeedAdmin gin.HandlerFunc
		// user must be the owner of the resource
		UserMatch gin.HandlerFunc
		// user must have phone verified
		NeedPhoneVerified gin.HandlerFunc
	}

	Repo struct {
		// Check if repo exists
		RepoExists gin.HandlerFunc
	}

	License struct {
		// Check if license is active
		Check gin.HandlerFunc
	}
}
