//go:build !saas

package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

func NewApiKeyQuotaMiddleware(config *config.Config) gin.HandlerFunc {
	return ApiKeyQuotas(config)
}

func ApiKeyQuotas(config *config.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
	}
}
