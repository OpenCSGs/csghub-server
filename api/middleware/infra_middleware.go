package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/common/config"
)

func SetInfraMiddleware(r *gin.Engine, config *config.Config, serviceName string) {
	r.Use(Recovery())
	instrumentation.SetupOtelMiddleware(r, config, serviceName)
	r.Use(Log(config))

	// Unified health check
	// Since readinessProbe cannot send a head request, use the get method
	if serviceName != instrumentation.RProxy {
		r.GET("/healthz", func(ctx *gin.Context) {
			ctx.Status(http.StatusOK)
		})
	}
}
