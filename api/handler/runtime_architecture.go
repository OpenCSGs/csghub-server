package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewRuntimeArchitectureHandler(config *config.Config) (*RuntimeArchitectureHandler, error) {
	nrc, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component, %w", err)
	}
	nrac, err := component.NewRuntimeArchitectureComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create runtime arch component, %w", err)
	}

	return &RuntimeArchitectureHandler{
		repo:           nrc,
		runtimeArch:    nrac,
		temporalClient: temporal.GetClient(),
	}, nil
}

type RuntimeArchitectureHandler struct {
	repo           component.RepoComponent
	runtimeArch    component.RuntimeArchitectureComponent
	temporalClient temporal.Client
}

// GetArchitectures godoc
// @Security     ApiKey
// @Summary      Get runtime framework architectures
// @Description  get runtime framework architectures
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/architecture [get]
func (r *RuntimeArchitectureHandler) ListByRuntimeFrameworkID(ctx *gin.Context) {
	strID := ctx.Param("id")
	id, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		slog.Error("invalid runtime framework ID", slog.Any("error", err))
		httpbase.BadRequest(ctx, "invalid runtime framework ID format")
		return
	}
	resp, err := r.runtimeArch.ListByRuntimeFrameworkID(ctx.Request.Context(), id)
	if err != nil {
		slog.Error("fail to list runtime architectures", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// UpdateArchitectures godoc
// @Security     ApiKey
// @Summary      Set runtime framework architectures
// @Description  set runtime framework architectures
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param        body body types.RuntimeArchitecture true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/architecture [put]
func (r *RuntimeArchitectureHandler) UpdateArchitecture(ctx *gin.Context) {
	var req types.RuntimeArchitecture
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request runtime framework id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	res, err := r.runtimeArch.SetArchitectures(ctx.Request.Context(), id, req.Architectures)
	if err != nil {
		slog.Error("Failed to set architectures", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, res)
}

// DeleteArchitectures godoc
// @Security     ApiKey
// @Summary      Delete runtime framework architectures
// @Description  Delete runtime framework architectures
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param        body body types.RuntimeArchitecture true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/architecture [delete]
func (r *RuntimeArchitectureHandler) DeleteArchitecture(ctx *gin.Context) {
	var req types.RuntimeArchitecture
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request runtime framework id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	list, err := r.runtimeArch.DeleteArchitectures(ctx.Request.Context(), id, req.Architectures)
	if err != nil {
		slog.Error("Failed to delete architectures", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, list)
}

// ScanArchitecture godoc
// @Security     ApiKey
// @Summary      Scan runtime architecture
// @Description  Scan runtime architecture
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        id path int true "runtime framework id"
// @Param 		 scan_type query int false "scan_type(0:all models, 1:new models, 2:old models)" Enums(0, 1, 2)
// @Param        task query string false "task" Enums(text-generation, text-to-image)
// @Param        body body types.RuntimeFrameworkModels true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id}/scan [post]
func (r *RuntimeArchitectureHandler) ScanArchitecture(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request runtime framework id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	scanTypeStr := ctx.Query("scan_type")
	if scanTypeStr == "" {
		slog.Error("Bad request scan type")
		httpbase.BadRequest(ctx, "bad request scan type")
		return
	}
	scanType, err := strconv.Atoi(scanTypeStr)
	if err != nil {
		slog.Error("Bad request scan format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.RuntimeFrameworkModels{}
	contentLength := ctx.GetHeader("Content-Length")
	if contentLength != "0" {
		err := ctx.ShouldBindJSON(&req)
		if err != nil {
			slog.Error("Failed to bind json", slog.Any("error", err))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}

	taskStr := ctx.Query("task")
	req.Task = types.PipelineTask(taskStr)
	req.ID = id
	req.ScanType = scanType

	//start workflow to do full scaning
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
	}

	_, err = r.temporalClient.ExecuteWorkflow(
		ctx.Request.Context(), workflowOptions, workflow.RuntimeFrameworkWorkflow, req,
	)
	if err != nil {
		slog.Error("Failed to scan architecture", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
