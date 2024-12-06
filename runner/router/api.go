package router

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/runner/handler"
)

func NewHttpServer(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())

	clusterPool, err := cluster.NewClusterPool()
	if err != nil {
		slog.Error("falied to build kubeconfig", "error", err)
		return nil, fmt.Errorf("failed to build kubeconfig,%w", err)
	}

	k8sHandler, err := handler.NewK8sHandler(config, clusterPool)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewHttpServer,%w", err)
	}
	apiGroup := r.Group("/api/v1")
	service := apiGroup.Group("/service")
	{
		service.POST("/:service/run", k8sHandler.RunService)
		service.PUT("/:service/update", k8sHandler.UpdateService)
		service.POST("/:service/stop", k8sHandler.StopService)
		service.GET("/:service/status", k8sHandler.ServiceStatus)
		service.GET("/:service/logs", k8sHandler.ServiceLogs)
		service.GET("/:service/logs/:pod_name", k8sHandler.ServiceLogsByPod)
		service.GET("/:service/info", k8sHandler.GetServiceInfo)
		service.GET("/status-all", k8sHandler.ServiceStatusAll)
		service.GET("/:service/get", k8sHandler.GetServiceByName)
		service.GET("/:service/replica", k8sHandler.GetReplica)
		service.DELETE("/:service/purge", k8sHandler.PurgeService)

	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", k8sHandler.GetClusterInfo)
		cluster.GET("/:id", k8sHandler.GetClusterInfoByID)
		cluster.PUT("/:id", k8sHandler.UpdateCluster)
	}
	argoHandler, err := handler.NewArgoHandler(config, clusterPool)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewHttpServer,%w", err)
	}

	workflows := apiGroup.Group("/workflows")
	{
		workflows.POST("", argoHandler.CreateWorkflow)
		workflows.GET("", argoHandler.ListWorkflows)
		workflows.DELETE("/:id", argoHandler.DeleteWorkflow)
		workflows.GET("/:id", argoHandler.GetWorkflow)
	}

	return r, nil
}
