package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
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
			slog.Debug("Authenticator token is invalid", slog.String("token_get", token), slog.String("token_expected", authToken))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api token mismatch, it must be in format 'Bearer xxx'"})
			return
		}

		c.Next()
	}
}
