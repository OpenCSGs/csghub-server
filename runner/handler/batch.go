//go:build saas || ee

package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/runner/component"
	rTypes "opencsg.com/csghub-server/runner/types"
)

type BatchHandler struct {
	batchComp *component.BatchComponent
}

func NewBatchHandler(batchComp *component.BatchComponent) *BatchHandler {
	return &BatchHandler{batchComp: batchComp}
}

func (h *BatchHandler) BatchStatus(c *gin.Context) {
	var req rTypes.BatchStatusRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	// Group items by type
	var ksvcNames, sandboxNames, wfIDs []string
	for _, item := range req.Items {
		switch item.Type {
		case rTypes.ResourceTypeKsvc:
			ksvcNames = append(ksvcNames, item.Name)
		case rTypes.ResourceTypeSandbox:
			sandboxNames = append(sandboxNames, item.Name)
		case rTypes.ResourceTypeWorkflow:
			wfIDs = append(wfIDs, item.Name) // Name field holds the K8s Argo workflow name (TaskId)
		}
	}

	// K8s-direct batch queries
	ksvcResults := h.batchKsvc(ctx, req.ClusterID, ksvcNames)
	sandboxResults := h.batchSandbox(ctx, req.ClusterID, sandboxNames)
	wfResults := h.batchWorkflow(ctx, req.ClusterID, wfIDs)

	// Assemble response
	resp := rTypes.BatchStatusResponse{
		Items: make([]rTypes.BatchStatusItemResult, 0, len(req.Items)),
	}
	for _, item := range req.Items {
		var r *rTypes.BatchStatusItemResult
		switch item.Type {
		case rTypes.ResourceTypeKsvc:
			r = ksvcResults[item.Name]
		case rTypes.ResourceTypeSandbox:
			r = sandboxResults[item.Name]
		case rTypes.ResourceTypeWorkflow:
			r = wfResults[item.Name]
		default:
			r = &rTypes.BatchStatusItemResult{Error: "unsupported type: " + string(item.Type)}
		}
		if r == nil {
			r = &rTypes.BatchStatusItemResult{Type: item.Type, Name: item.Name, Error: "not found"}
		}
		resp.Items = append(resp.Items, *r)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *BatchHandler) batchKsvc(ctx context.Context, clusterID string, names []string) map[string]*rTypes.BatchStatusItemResult {
	if len(names) == 0 {
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	results, err := h.batchComp.BatchKsvcStatus(ctx, clusterID, names)
	if err != nil {
		slog.ErrorContext(ctx, "batch: ksvc failed", "cluster_id", clusterID, "count", len(names), "error", err)
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	return results
}

func (h *BatchHandler) batchSandbox(ctx context.Context, clusterID string, names []string) map[string]*rTypes.BatchStatusItemResult {
	if len(names) == 0 {
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	results, err := h.batchComp.BatchSandboxStatus(ctx, clusterID, names)
	if err != nil {
		slog.ErrorContext(ctx, "batch: sandbox failed", "cluster_id", clusterID, "count", len(names), "error", err)
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	return results
}

func (h *BatchHandler) batchWorkflow(ctx context.Context, clusterID string, names []string) map[string]*rTypes.BatchStatusItemResult {
	if len(names) == 0 {
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	results, err := h.batchComp.BatchWorkflowStatus(ctx, clusterID, names)
	if err != nil {
		slog.ErrorContext(ctx, "batch: workflow failed", "cluster_id", clusterID, "count", len(names), "error", err)
		return make(map[string]*rTypes.BatchStatusItemResult)
	}
	return results
}
