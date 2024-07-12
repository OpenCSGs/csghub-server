package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewAccountRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	r.Use(middleware.Authenticator(config))

	// metering
	meterHandler, err := handler.NewMeteringHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating multi sync handler:%w", err)
	}

	apiGroup := r.Group("/api/v1/accounting")

	meterGroup := apiGroup.Group("/metering")
	{
		meterGroup.GET("/:id/statements", meterHandler.QueryMeteringStatementByUserID)
	}

	return r, nil
}
