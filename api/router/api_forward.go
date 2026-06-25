package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func createForwardRoutes(apiGroup *gin.RouterGroup, config *config.Config) error {
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

	academyGrp := apiGroup.Group("/academy")
	academyGrp.Any("/*any", academyHandler.Proxy)

	supplydemandGrp := apiGroup.Group("/supplydemand")
	supplydemandGrp.Any("/*any", supplydemandHandler.Proxy)

	return nil
}
