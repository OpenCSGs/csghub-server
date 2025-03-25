package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		sessions.Default(c).Set(httpbase.CurrentUserCtxVar, claims.CurrentUser)
		err = sessions.Default(c).Save()
		if err != nil {
			slog.Error("fail to save session", slog.Any("error", err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
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
		if userName != nil {
			httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
			httpbase.SetCurrentUser(c, userName.(string))
		}

		c.Next()
	}
}

func Authenticator(config *config.Config) gin.HandlerFunc {
	//TODO:change to component
	userStore := database.NewUserStore()
	return func(c *gin.Context) {
		sessionObj, sessionExists := c.Get(sessions.DefaultKey)
		if sessionExists && sessionObj != nil {
			session := sessions.Default(c)
			sessionUserName := session.Get(httpbase.CurrentUserCtxVar)
			if sessionUserName != nil {
				slog.Debug("get username from session", slog.Any("session username", sessionUserName.(string)))
				if len(sessionUserName.(string)) > 0 {
					httpbase.SetCurrentUser(c, sessionUserName.(string))
					httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
					c.Next()
					return
				}
			}
		}

		apiToken := config.APIToken

		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			httpbase.UnauthorizedError(c, errors.New("authorization header must starts with `Bearer `"))
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
			httpbase.SetAuthType(c, httpbase.AuthTypeApiKey)
			c.Next()
			return
		}

		if strings.Contains(token, ".") {
			claims, err := parseJWTToken(config.JWT.SigningKey, token)
			if err == nil {
				httpbase.SetCurrentUser(c, claims.CurrentUser)
				httpbase.SetAuthType(c, httpbase.AuthTypeJwt)
				return
			} else {
				slog.Error("verify jwt token error", slog.Any("error", err))
			}
		} else {
			//TODO:use cache to check access token
			user, _ := userStore.FindByAccessToken(context.Background(), token)
			if user != nil {
				httpbase.SetCurrentUser(c, user.Username)
				httpbase.SetAccessToken(c, token)
				httpbase.SetAuthType(c, httpbase.AuthTypeAccessToken)
				c.Next()
				return
			}
		}

		slog.ErrorContext(c, "invalid Bearer token", slog.String("token", token),
			slog.String("ip", c.ClientIP()),
			slog.String("method", c.Request.Method),
			slog.String("url", c.Request.URL.RequestURI()),
		)
		httpbase.UnauthorizedError(c, errors.New("invalid Bearer token"))
		c.Abort()
	}
}

func parseJWTToken(signKey, tokenString string) (*types.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(signKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invilid JWT token,%w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid JWT token")
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if ok {
		return claims, nil
	}
	return nil, fmt.Errorf("JWT token claims not match: %+v", *token)
}

func OnlyAPIKeyAuthenticator(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiToken := config.APIToken

		// Get Authorization token
		authHeader := c.Request.Header.Get("Authorization")

		// Check Authorization Header format
		if authHeader == "" {
			slog.Info("missing authorization header", slog.Any("url", c.Request.URL))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "please use API key for authentication"})
			return
		}

		c.Next()
	}
}

func MustLogin() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, errors.New("unknown user, please login first"))
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
			httpbase.UnauthorizedError(ctx, errors.New("unknown user, please login first"))
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
			httpbase.ForbiddenError(ctx, errors.New("only admin user can access"))
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
			httpbase.UnauthorizedError(ctx, errors.New("unknown user, please login first"))
			ctx.Abort()
			return
		}

		userName := ctx.Param("username")
		if userName != currentUser {
			httpbase.UnauthorizedError(ctx, errors.New("user not match, try to query user account not owned"))
			slog.Error("user not match, try to query user account not owned", "currentUser", currentUser, "userName", userName)
			ctx.Abort()
			return
		}
	}
}

type AuthenticatorCollection struct {
	// only can be accessed by api key
	NeedAPIKey gin.HandlerFunc
	// user need to login first
	NeedLogin gin.HandlerFunc
	//user must be admin role to access
	NeedAdmin gin.HandlerFunc
	// user must be the owner of the resource
	UserMatch gin.HandlerFunc
}
