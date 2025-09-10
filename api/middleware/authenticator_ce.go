//go:build !saas

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

func NeedPhoneVerified(config *config.Config) gin.HandlerFunc {
	return MustLogin()
}
