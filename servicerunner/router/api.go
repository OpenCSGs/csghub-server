package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/servicerunner/handler"
)

func NewHttpServer(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())

	handler, err := handler.NewK8sHander(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewHttpServer,%w", err)
	}
	apiGroup := r.Group("/api/v1")
	service := apiGroup.Group("/service")
	{
		service.POST("/:service/run", handler.RunService)
		service.PUT("/:service/update", handler.UpdateService)
		service.POST("/:service/stop", handler.StopService)
		service.GET("/:service/status", handler.ServiceStatus)
		service.GET("/:service/logs", handler.ServiceLogs)
		service.GET("/:service/logs/:pod_name", handler.ServiceLogsByPod)
		service.GET("/:service/info", handler.GetServiceInfo)
		service.GET("/status-all", handler.ServiceStatusAll)
		service.GET("/:service/get", handler.GetServiceByName)
		service.GET("/:service/replica", handler.GetReplica)
		service.DELETE("/:service/purge", handler.PurgeService)

	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", handler.GetClusterInfo)
		cluster.GET("/:id", handler.GetClusterInfoByID)
		cluster.PUT("/:id", handler.UpdateCluster)
	}

	return r, nil
}
