package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
)

func GetUserFromAccessToken() gin.HandlerFunc {
	userStore := database.NewUserStore()
	return func(c *gin.Context) {
		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")

		if authHeader != "" {
			// Get token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			user, err := userStore.FindByAccessToken(context.Background(), token)
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
