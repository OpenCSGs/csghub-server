//go:build ee || saas

package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func extendRoutes(v1Group *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, config *config.Config) error {
	agentProxy, err := handler.NewAgentProxyHandler(config)
	if err != nil {
		return fmt.Errorf("error creating agent proxy handler :%w", err)
	}
	createAgentRoute(v1Group, agentProxy, middlewareCollection)
	return nil
}

func createAgentRoute(v1Group *gin.RouterGroup, agentProxy handler.AgentProxyHandler, middlewareCollection middleware.MiddlewareCollection) {
	agentGroup := v1Group.Group("/agent", middlewareCollection.License.Check, middlewareCollection.Auth.NeedPhoneVerified)
	agentGroup.Any("/:type/*any", agentProxy.ProxyToApi("/api/v1%s", "any"))
}
