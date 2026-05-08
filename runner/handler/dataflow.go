package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/component"
)

// DataflowHandler handles dataflow job requests
type DataflowHandler struct {
	clusterPool cluster.Pool
	config      *config.Config
	dfc         component.DataflowComponent
}

func NewDataflowHandler(config *config.Config, clusterPool cluster.Pool) (*DataflowHandler, error) {
	dfc := component.NewDataflowComponent(config, clusterPool)
	return &DataflowHandler{
		clusterPool: clusterPool,
		config:      config,
		dfc:         dfc,
	}, nil
}

// CreateDataflowWorkflow creates a new dataflow workflow
func (h *DataflowHandler) CreateDataflowWorkflow(ctx *gin.Context) {
	var req types.DataflowArgoJobReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx, "bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.dfc.CreateWorkflow(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create dataflow workflow", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetDataflowStatus gets the status of a dataflow workflow
func (h *DataflowHandler) GetDataflowStatus(ctx *gin.Context) {
	taskID := ctx.Param("task_id")
	clusterID := ctx.Query("cluster_id")

	req := types.DataflowArgoReq{
		ArgoTaskID: taskID,
		ClusterID:  clusterID,
	}

	status, err := h.dfc.GetStatus(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get dataflow status", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, status)
}

// DeleteDataflowWorkflow deletes a dataflow workflow
func (h *DataflowHandler) DeleteDataflowWorkflow(ctx *gin.Context) {
	taskID := ctx.Param("task_id")
	clusterID := ctx.Query("cluster_id")

	req := types.DataflowArgoReq{
		ArgoTaskID: taskID,
		ClusterID:  clusterID,
	}

	err := h.dfc.DeleteWorkflow(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete dataflow workflow", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "dataflow workflow deleted successfully"})
}
