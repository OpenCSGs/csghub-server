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
		sessions.Default(c).Save()

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
