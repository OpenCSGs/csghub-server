package router

import (
	"context"
	"fmt"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/logcollector/component"
	"opencsg.com/csghub-server/logcollector/handler"
)

func NewHttpServer(ctx context.Context, config *config.Config) (*gin.Engine, component.LogFactory, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))

	needAPIKey := middleware.NeedAPIKey(config)

	//add router for golang pprof
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")

	logFactory, err := component.NewLogFactory(ctx, config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating log collector, error: %w", err)
	}
	err = logFactory.Start()
	if err != nil {
		return nil, nil, fmt.Errorf("error starting log collector, error: %w", err)
	}

	logCollectorHandler, err := handler.NewLogCollectorHandler(logFactory)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build NewHttpServer,%w", err)
	}

	apiGroup := r.Group("/api/v1")
	logcollectorGroup := apiGroup.Group("/logcollector/")
	{
		logcollectorGroup.POST("/health", logCollectorHandler.Health)
		logcollectorGroup.POST("/stats", logCollectorHandler.GetStats)
	}
	return r, logFactory, nil
}
