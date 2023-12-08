package apiserver

import (
	"opencsg.com/starhub-server/config"
	"opencsg.com/starhub-server/pkg/httpbase"
	"opencsg.com/starhub-server/pkg/log"
	"opencsg.com/starhub-server/pkg/router"
)

// type ServerOpt struct {
// 	Port          int
// 	Logger        log.Logger
// 	DB            *model.DB
// 	Cache         *cache.Cache
// 	EnableSwagger bool
// 	ServerHost    *serverhost.ServerHost
// }

// NewServer fires a new server
// func NewServer(opt *ServerOpt) (server *httpbase.GracefulServer, err error) {
// 	router := gin.New()

// 	// health check
// 	router.GET("/healthz", func(c *gin.Context) {
// 		httpbase.SkipLogging(c)
// 		c.String(200, "OK")
// 	})

// 	setUpAPIRoutes(router)
// 	server = httpbase.NewGracefulServer(httpbase.GraceServerOpt{
// 		Logger: opt.Logger,
// 		Port:   opt.Port,
// 	}, router)

// 	return
// }

// func setUpAPIRoutes(router *gin.Engine) {
// 	api := router.Group("/api/v1")

// 	api.POST("/models")
// }

func NewServer(config *config.Config, logger log.Logger, router *router.Router) (server *httpbase.GracefulServer) {
	return httpbase.NewGracefulServer(httpbase.GraceServerOpt{
		Port:   config.APIServer.Port,
		Logger: logger,
	}, router)
}
