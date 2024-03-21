package router

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewRProxyRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://portal-stg.opencsg.com", "https://portal.opencsg.com"},
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
	}))
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	r.Use(middleware.Authenticator(config))

	handler, err := handler.NewRProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating rproxy handler:%w", err)
	}
	r.Any("/*api", handler.Proxy)

	return r, nil
}
