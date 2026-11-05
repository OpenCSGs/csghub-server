package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/handler"
)

// adminCancelHandler documents the shared cancellation operation for each administrator submodule.
type adminCancelHandler struct{ target gin.HandlerFunc }

// createAdminRoutes applies the caller module to exclusive and shared synchronization operations.
func createAdminRoutes(apiGroup *gin.RouterGroup, platformAdmin func(...types.AdminPermission) gin.HandlerFunc, manager *handler.ManagerHandler) {
	apiGroup.POST("/lfs_sync/sync_now", platformAdmin(types.AdminAssetMirror), manager.SyncNow)
	apiGroup.GET("/mirror/tasks", platformAdmin(types.AdminAssetMirror), manager.ListTasks)
	h := &adminCancelHandler{target: manager.Cancel}
	apiGroup.POST("/admin/asset/mirror/lfs_sync/cancel", platformAdmin(types.AdminAssetMirror), h.MirrorCancel)
	apiGroup.POST("/admin/asset/sync/lfs_sync/cancel", platformAdmin(types.AdminAssetSync), h.SyncCancel)
}

// MirrorCancel cancels a synchronization task after downstream administrator authorization.
// @Security ApiKey
// @Summary Cancels a synchronization task
// @Description Requires asset/mirror administrator permission; the downstream service independently verifies the original user.
// @Tags Admin asset mirror
// @Accept json
// @Produce json
// @Param body body handler.CancelReq true "Synchronization task ID"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} map[string]string "Invalid task ID payload"
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} map[string]string "Synchronization task not found"
// @Failure 500 {object} map[string]interface{} "Authorization or cancellation failure"
// @Router /admin/asset/mirror/lfs_sync/cancel [post]
func (h *adminCancelHandler) MirrorCancel(ctx *gin.Context) { h.target(ctx) }

// SyncCancel cancels a synchronization task after downstream administrator authorization.
// @Security ApiKey
// @Summary Cancels a synchronization task
// @Description Requires asset/sync administrator permission; the downstream service independently verifies the original user.
// @Tags Admin asset sync
// @Accept json
// @Produce json
// @Param body body handler.CancelReq true "Synchronization task ID"
// @Success 200 {object} types.Response{}
// @Failure 400 {object} map[string]string "Invalid task ID payload"
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} map[string]string "Synchronization task not found"
// @Failure 500 {object} map[string]interface{} "Authorization or cancellation failure"
// @Router /admin/asset/sync/lfs_sync/cancel [post]
func (h *adminCancelHandler) SyncCancel(ctx *gin.Context) { h.target(ctx) }
