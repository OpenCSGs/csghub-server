package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/store/database"
)

func GetUserFromAccessToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Auzhorization token
		authHeader := c.Request.Header.Get("Authorization")

		if authHeader != "" {
			// Get token
			token := strings.TrimPrefix(authHeader, "Bearer ")
			userStore := database.NewUserStore()
			user, err := userStore.FindByAccessToken(context.Background(), token)
			if err != nil {
				slog.Debug("Can not find user by access token", slog.String("token", token))
				c.Next()
				return
			}
			if user != nil {
				c.Set("currentUser", user.Username)
			}
		}

		c.Next()
	}
}
