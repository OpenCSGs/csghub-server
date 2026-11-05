//go:build !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
)

// createAdminTokenPlanModelRoutes leaves subscription administration unavailable outside SaaS.
func createAdminTokenPlanModelRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.OpenAIHandlerImpl, _ gin.HandlerFunc) {
}
