package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/common/config"
)

const apiSecretHeaderName = "Gitlab-Shell-Api-Request"

func parseGitlabShellJWTToken(signKey, tokenString string) (bool, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(signKey), nil
	})
	if err != nil {
		return false, fmt.Errorf("invilid JWT token,%w", err)
	}

	if !token.Valid {
		return false, errors.New("invalid JWT token")
	}
	return true, nil
}

func CheckGitlabShellJWTToken(config *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Request.Header.Get(apiSecretHeaderName)
		pass, err := parseGitlabShellJWTToken(config.GitalyServer.JWTSecret, tokenString)
		if err != nil {
			slog.Debug("fail to parse gitlab-shell jwt token", slog.String("token_get", tokenString), slog.Any("error", err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		if !pass {
			slog.Debug("invalid gilab-shell jwt token", slog.String("token_get", tokenString))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid gitlab-shell jwt token"})
			return
		}
		c.Next()
	}
}
