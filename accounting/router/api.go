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

	// Credit routes
	creditHandler, err := handler.NewCreditHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating credit handler:%w", err)
	}

	apiGroup := r.Group("/api/v1/accounting")

	creditGroup := apiGroup.Group("/credit")
	{
		creditGroup.GET("/balance", creditHandler.QueryAllUsersBalance)
		creditGroup.GET("/:id/balance", creditHandler.QueryBalanceByUserID)
		creditGroup.GET("/:id/statement", creditHandler.QueryStatementByUserID)
	}

	return r, nil
}
