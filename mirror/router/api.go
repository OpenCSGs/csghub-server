package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/handler"
)

// same router for both repo sync and lfs sync service
func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))
	needAdmin := middleware.NeedAdmin(config)
	authMiddleware := middleware.Authenticator(config)

	managerHandler, err := handler.NewManagerHandler(config)
	if err != nil {
		return nil, err
	}
	r.POST("/api/v1/lfs_sync/stop_worker_by_id", authMiddleware, needAdmin, managerHandler.StopWorkerByID)
	r.POST("/api/v1/lfs_sync/sync_now", authMiddleware, needAdmin, managerHandler.SyncNow)
	r.POST("/api/v1/lfs_sync/cancel", authMiddleware, needAdmin, managerHandler.Cancel)
	r.GET("/api/v1/mirror/tasks", authMiddleware, needAdmin, managerHandler.ListTasks)

	return r, nil
}
