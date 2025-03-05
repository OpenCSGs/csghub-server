package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
)

func (m *Middleware) GetUserFromAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")

		if authHeader != "" {
			// Get token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			user, err := m.userComponent.FindByAccessToken(context.Background(), token)
			if err != nil {
				slog.Debug("Can not find user by access token", slog.String("token", token))
				c.Next()
				return
			}
			if user != nil {
				httpbase.SetCurrentUser(c, user.Username)
				httpbase.SetAuthType(c, httpbase.AuthTypeAccessToken)
			}
		}

		c.Next()
	}
}
