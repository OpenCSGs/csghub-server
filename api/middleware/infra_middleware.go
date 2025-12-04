package middleware

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/common/config"
)

func SetInfraMiddleware(r *gin.Engine, config *config.Config, serviceName string) {
	r.Use(Recovery())
	instrumentation.SetupOtelMiddleware(r, config, serviceName)
	r.Use(Log(config))
}
