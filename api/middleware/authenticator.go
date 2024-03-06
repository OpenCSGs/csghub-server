package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func Authenticator(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken := config.APIToken

		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")

		// Check Authorization Header formt
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		// Get token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token != authToken {
			valid, msg, err := checkJWTToken(config, token)
			if err != nil {
				slog.Debug("JWT token is invalid", slog.String("token_get", token))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
				return
			}

			if !valid {
				slog.Debug("Authenticator token is invalid", slog.String("token_get", token), slog.String("token_expected", authToken))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api token mismatch, it must be in format 'Bearer xxx'"})
				return
			}

			err = setCurrentUser(c, config, token)
			if err != nil {
				slog.Debug("Error parsing claims from JWT token", slog.String("token_get", token))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "error parsing claims from JWT token"})
				return
			}
		}

		c.Next()
	}
}

func checkJWTToken(config *config.Config, tokenString string) (bool, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWT.SigningKey), nil
	})
	if err != nil {
		return false, "Invilid JWT token", err
	}

	switch {
	case token.Valid:
		return true, "", nil
	case errors.Is(err, jwt.ErrTokenMalformed):
		return false, "This is not a JWT token", nil
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return false, "Invilid JWT token", nil
	case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
		return false, "The token has been expired", nil
	default:
		return false, "Could not handle this token", nil
	}
}

func setCurrentUser(ctx *gin.Context, config *config.Config, tokenString string) error {
	token, err := jwt.ParseWithClaims(tokenString, &types.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWT.SigningKey), nil
	})
	if err != nil {
		return err
	}

	claims, ok := token.Claims.(*types.JWTClaims)
	if ok {
		ctx.Set("currentUser", claims.CurrentUser)
		slog.Info("user jwt token validated", slog.Any("currentUser", claims.CurrentUser))
		return nil
	}
	return fmt.Errorf("error parsing claims: %v", token)
}
