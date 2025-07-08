package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type MonitorHandler struct {
	monitor component.MonitorComponent
}

func NewMonitorHandler(cfg *config.Config) (*MonitorHandler, error) {
	monitorComp, err := component.NewMonitorComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("fail to create monitor component, error: %w", err)
	}
	return &MonitorHandler{
		monitor: monitorComp,
	}, nil
}

// CPUUsage      godoc
// @Security     ApiKey
// @Summary      Get instance cpu usage
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,spaces" Enums(models,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Success      200  {object}  types.Response{data=types.MonitorCPUResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/{type}/{id}/cpu/{instance}/usage [get]
func (h *MonitorHandler) CPUUsage(ctx *gin.Context) {
	req, err := getRequestParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.CPUUsage(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get cpu usage", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// CPUUsage      godoc
// @Security     ApiKey
// @Summary      Get instance cpu usage for evaluation
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models" Enums(models)
// @Success      200  {object}  types.Response{data=types.MonitorCPUResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/evaluations/{id}/cpu/{instance}/usage [get]
func (h *MonitorHandler) CPUUsageEvaluation(ctx *gin.Context) {
	req, err := getEvaluationParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.CPUUsage(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get cpu usage", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// MemoryUsage   godoc
// @Security     ApiKey
// @Summary      Get instance memory usage
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,spaces" Enums(models,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Success      200  {object}  types.Response{data=types.MonitorMemoryResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/{type}/{id}/memory/{instance}/usage [get]
func (h *MonitorHandler) MemoryUsage(ctx *gin.Context) {
	req, err := getRequestParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.MemoryUsage(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get memory usage", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// MemoryUsage   godoc
// @Security     ApiKey
// @Summary      Get instance memory usage for evaluation
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models" Enums(models)
// @Success      200  {object}  types.Response{data=types.MonitorMemoryResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/evaluations/{id}/memory/{instance}/usage [get]
func (h *MonitorHandler) MemoryUsageEvaluation(ctx *gin.Context) {
	req, err := getEvaluationParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.MemoryUsage(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get memory usage", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// Requestcount  godoc
// @Security     ApiKey
// @Summary      Get instance request count
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,spaces" Enums(models,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Success      200  {object}  types.Response{data=types.MonitorRequestCountResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/{type}/{id}/request/{instance}/count [get]
func (h *MonitorHandler) RequestCount(ctx *gin.Context) {
	req, err := getRequestParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.RequestCount(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get request count", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// Requestlatency  godoc
// @Security     ApiKey
// @Summary      Get instance request latency
// @Tags         Monitor
// @Accept       json
// @Produce      json
// @Param		 repo_type path string true "models,spaces" Enums(models,spaces)
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Success      200  {object}  types.Response{data=types.MonitorRequestLatencyResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/{type}/{id}/request/{instance}/latency [get]
func (h *MonitorHandler) RequestLatency(ctx *gin.Context) {
	req, err := getRequestParameters(ctx)
	if err != nil {
		slog.Error("Failed to get request parameters", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	resp, err := h.monitor.RequestLatency(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get request latency", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

func getRequestParameters(ctx *gin.Context) (*types.MonitorReq, error) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("bad request format for namespace and name, error: %w", err)
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad request format for deploy id, error: %w", err)
	}
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		return nil, fmt.Errorf("bad request format for instance")
	}
	deployType := ctx.Param("type")
	if len(deployType) < 1 {
		return nil, fmt.Errorf("bad request format for type")
	}
	lastDuration, timeRange := common.GetValidTimeDuration(ctx)
	req := &types.MonitorReq{
		CurrentUser:  currentUser,
		Namespace:    namespace,
		Name:         name,
		RepoType:     repoType,
		DeployID:     deployID,
		DeployType:   deployType,
		Instance:     instance,
		LastDuration: lastDuration,
		TimeRange:    timeRange,
	}
	return req, nil
}

func getEvaluationParameters(ctx *gin.Context) (*types.MonitorReq, error) {
	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad request format for deploy id, error: %w", err)
	}
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		return nil, fmt.Errorf("bad request format for instance")
	}
	lastDuration, timeRange := common.GetValidTimeDuration(ctx)
	req := &types.MonitorReq{
		CurrentUser:  currentUser,
		RepoType:     repoType,
		DeployID:     deployID,
		DeployType:   "evaluation",
		Instance:     instance,
		LastDuration: lastDuration,
		TimeRange:    timeRange,
	}
	return req, nil
}
