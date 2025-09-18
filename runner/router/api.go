package router

import (
	"context"
	"fmt"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"log/slog"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter"
	"opencsg.com/csghub-server/runner/handler"
)

func NewHttpServer(ctx context.Context, config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	middleware.SetInfraMiddleware(r, config, instrumentation.Runner)

	needAPIKey := middleware.NeedAPIKey(config)

	//add router for golang pprof
	debugGroup := r.Group("/debug", needAPIKey)
	pprof.RouteRegister(debugGroup, "pprof")

	clusterPool, err := cluster.NewClusterPool(config)
	if err != nil {
		slog.Error("failed to build cluster pool by auto detect environment", slog.Any("error", err))
		return nil, fmt.Errorf("failed to build cluster pool by auto detect environment error: %w", err)
	}
	logReporter, err := reporter.NewAndStartLogCollector(ctx, config, types.ClientTypeRunner)
	if err != nil {
		return nil, fmt.Errorf("failed to start logReporter error: %w", err)
	}
	// runner apis
	k8sHandler, err := handler.NewK8sHandler(config, clusterPool, logReporter)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewK8sHandler error: %w", err)
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
		service.GET("/:service/get", k8sHandler.GetServiceByName)
		service.GET("/:service/replica", k8sHandler.GetReplica)
		service.DELETE("/:service/purge", k8sHandler.PurgeService)
	}

	// cluster api
	clusterHandler, err := handler.NewClusterHandler(config, clusterPool)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewClusterHandler error: %w", err)
	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("/:id", clusterHandler.GetClusterInfoByID)
	}

	// argo for evaluation
	argoHandler, err := handler.NewArgoHandler(config, clusterPool, logReporter)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewArgoHandler error: %w", err)
	}
	workflows := apiGroup.Group("/workflows")
	{
		workflows.POST("", argoHandler.CreateWorkflow)
		workflows.GET("", argoHandler.ListWorkflows)
		workflows.DELETE("/:id", argoHandler.DeleteWorkflow)
		workflows.GET("/:id", argoHandler.GetWorkflow)
	}

	// image builder
	imagebuilderHandler, err := handler.NewImagebuilderHandler(ctx, config, clusterPool, logReporter)
	if err != nil {
		return nil, fmt.Errorf("failed to build NewImagebuilderHandler error: %w", err)
	}
	imagebuilderGroup := apiGroup.Group("/imagebuilder")
	{
		imagebuilderGroup.POST("/builder", imagebuilderHandler.Build)
		imagebuilderGroup.PUT("/stop", imagebuilderHandler.Stop)
	}

	return r, nil
}
