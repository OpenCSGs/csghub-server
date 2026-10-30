package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

// createForwardRoutes initializes proxy handlers and registers their access gates.
func createForwardRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, config *config.Config) error {
	// dataflow proxy
	dataflowHandler, err := handler.NewDataflowProxyHandler(config)
	if err != nil {
		return fmt.Errorf("error creating data flow proxy handler:%w", err)
	}

	academyHandler, err := handler.NewAcademyProxyHandler(config)
	if err != nil {
		return fmt.Errorf("error creating academy proxy handler:%w", err)
	}

	supplydemandHandler, err := handler.NewSupplydemandProxyHandler(config)
	if err != nil {
		return fmt.Errorf("error creating supplydemand proxy handler:%w", err)
	}

	dataflowGrp := apiGroup.Group("/dataflow")
	dataflowGrp.Use(middleware.MustLogin())
	dataflowGrp.Any("/*any", dataflowHandler.Proxy)

	createAcademyForwardRoutes(apiGroup, academyHandler.Proxy)
	createAdminAcademyRoutes(apiGroup, middlewareCollection, academyHandler.AcademyProxyToAPI)

	supplydemandGrp := apiGroup.Group("/supplydemand")
	supplydemandGrp.Any("/*any", supplydemandHandler.Proxy)

	return nil
}

// createAcademyForwardRoutes preserves public and personal academy requests.
// The proxy handler rejects legacy administration paths before forwarding.
func createAcademyForwardRoutes(apiGroup *gin.RouterGroup, forward gin.HandlerFunc) {
	academyGrp := apiGroup.Group("/academy")
	academyGrp.Any("/*any", forward)
}
