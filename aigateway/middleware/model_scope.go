//go:build !saas

// Package middleware provides gin middlewares specific to the aigateway
// service.
package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
)

// NewApiKeyScopeMiddleware is a no-op outside the saas edition: API key
// model scopes only exist there.
func NewApiKeyScopeMiddleware(config *config.Config) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
	}
}
