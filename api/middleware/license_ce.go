//go:build !ee && !saas

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

func CheckLicense(_ *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
