//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
)

// createAdminSystemConfigSelectorRoutes is empty in CE, which has no system configuration module.
func createAdminSystemConfigSelectorRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.ModelHandler, _ *handler.TagsHandler, _ *handler.AccountingHandler) {
}

// createAdminCommercialAPISelectorRoutes is empty in CE, which has no commercial API administration module.
func createAdminCommercialAPISelectorRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.ModelHandler, _ *handler.TagsHandler, _ *handler.AccountingHandler) {
}

// createAdminBillingResourceRoutes is empty because billing selectors are SaaS-only.
func createAdminBillingResourceRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.SpaceResourceHandler) {
}

// createAdminBillingClusterRoutes is empty because billing selectors are SaaS-only.
func createAdminBillingClusterRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.ClusterHandler) {
}

// createAdminBillingUserProxyRoutes is empty because billing selectors are SaaS-only.
func createAdminBillingUserProxyRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.InternalServiceProxyHandler) {
}
