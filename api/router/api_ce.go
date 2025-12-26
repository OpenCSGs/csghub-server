//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func useAdvancedMiddleware(r *gin.Engine, config *config.Config) {}

func createAdvancedRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, config *config.Config) error {
	return nil
}

func createExtendedUserRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxyHandler *handler.InternalServiceProxyHandler) {
}

func createXnetRoutes(_ *gin.Engine, _ middleware.MiddlewareCollection, _ *config.Config) error {
	return nil
}
