package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

// create finetune  godoc
// @Security     ApiKey
// @Summary      run fineune with model and dataset
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        body body types.FinetuneReq true "body setting of finetune"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes [post]
func (h *FinetuneHandler) RunFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req types.FinetuneReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad finetune request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to check sensitive request for create finetune", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	if req.LearningRate <= 0 {
		req.LearningRate = 0.0001
	}
	req.Username = currentUser
	finetune, err := h.ftComp.CreateFinetuneJob(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to create finetune job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	h.createAgentInstanceTask(ctx.Request.Context(), req.Agent, finetune.TaskId, currentUser)

	httpbase.OK(ctx, finetune)
}

// get finetune  godoc
// @Security     ApiKey
// @Summary      get Finetune job by id
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.EvaluationRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes/{id} [get]
func (h *FinetuneHandler) GetFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format for get finetune", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.FinetineGetReq{}
	req.ID = id
	req.Username = currentUser
	finetune, err := h.ftComp.GetFinetuneJob(ctx.Request.Context(), *req)
	if err != nil {
		slog.Error("Failed to get finetune job by id", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, finetune)
}

// deleteFinetune  godoc
// @Security     ApiKey
// @Summary      delete finetune job by id
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes/{id} [delete]
func (h *FinetuneHandler) DeleteFinetuneJob(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format for delete finetune", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.ArgoWorkFlowDeleteReq{}
	req.ID = id
	req.Username = currentUser
	err = h.ftComp.DeleteFinetuneJob(ctx.Request.Context(), *req)
	if err != nil {
		slog.Error("failed to delete finetune job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetFinetuneLogs   godoc
// @Security     JWT token
// @Summary      get finetune job logs
// @Tags         Finetune
// @Accept       json
// @Produce      json
// @Param        id path string true "finetune job id"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /finetunes/{id}/logs [get]
func (h *FinetuneHandler) GetLogs(ctx *gin.Context) {
	since := ctx.Query("since")
	currentUser := httpbase.GetCurrentUser(ctx)
	stream := ctx.Query("stream")

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format for finetune job", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := types.FinetuneLogReq{
		CurrentUser: currentUser,
		Since:       since,
		ID:          id,
	}

	allow, err := h.ftComp.CheckUserPermission(ctx.Request.Context(), req)
	if !allow {
		slog.Error("user not allowed to read finetune job logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to read finetune job logs"))
		return
	}

	if strings.Trim(stream, " ") == "true" {
		h.readLogInStream(ctx, req)
	} else {
		h.readLogNonStream(ctx, req)
	}
}

func (h *FinetuneHandler) readLogNonStream(ctx *gin.Context, req types.FinetuneLogReq) {
	logs, err := h.ftComp.ReadJobLogsNonStream(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get finetune job non-stream logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, logs)
}

func (h *FinetuneHandler) readLogInStream(ctx *gin.Context, req types.FinetuneLogReq) {
	logReader, err := h.ftComp.ReadJobLogsInStream(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get finetune job in-stream logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any finetune job log"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}
