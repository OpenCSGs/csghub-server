package apiserver

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/httpbase"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/serverhost"
	"github.com/gin-gonic/gin"
)

type ServerOpt struct {
	Port          int
	Logger        log.Logger
	DB            *model.DB
	Cache         *cache.Cache
	EnableSwagger bool
	ServerHost    *serverhost.ServerHost
}

// NewServer fires a new server
func NewServer(opt *ServerOpt) (server *httpbase.GracefulServer, err error) {
	router := gin.New()

	// health check
	router.GET("/healthz", func(c *gin.Context) {
		httpbase.SkipLogging(c)
		c.String(200, "OK")
	})
	server = httpbase.NewGracefulServer(httpbase.GraceServerOpt{
		Logger: opt.Logger,
		Port:   opt.Port,
	}, router)

	return
}
