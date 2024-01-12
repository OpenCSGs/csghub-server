package middleware

import (
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
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		// Get token
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token != authToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		c.Next()
	}
}
