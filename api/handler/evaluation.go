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
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewEvaluationHandler(config *config.Config) (*EvaluationHandler, error) {
	wkf, err := component.NewEvaluationComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &EvaluationHandler{
		evaluation: wkf,
		sensitive:  sc,
	}, nil
}

type EvaluationHandler struct {
	evaluation component.EvaluationComponent
	sensitive  component.SensitiveComponent
}

// create evaluation  godoc
// @Security     ApiKey
// @Summary      run model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body types.EvaluationReq true "body setting of evaluation"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations [post]
func (h *EvaluationHandler) RunEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req types.EvaluationReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}
	req.Username = currentUser
	if req.OwnerNamespace == "" {
		req.OwnerNamespace = currentUser
	}
	evaluation, err := h.evaluation.CreateEvaluation(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create evaluation job", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, evaluation)
}

// get evaluation  godoc
// @Security     ApiKey
// @Summary      get model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.EvaluationRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations/{id} [get]
func (h *EvaluationHandler) GetEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.EvaluationGetReq{}
	req.ID = id
	req.Username = currentUser
	evaluation, err := h.evaluation.GetEvaluation(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get evaluation job", slog.Any("error", err))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, evaluation)

}

// deleteEvaluation  godoc
// @Security     ApiKey
// @Summary      delete model evaluation
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations/{id} [delete]
func (h *EvaluationHandler) DeleteEvaluation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req = &types.EvaluationDelReq{}
	req.ID = id
	req.Username = currentUser
	err = h.evaluation.DeleteEvaluation(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete evaluation job", slog.Any("error", err))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetEvaluationLogs godoc
// @Security     ApiKey
// @Summary      get evaluation job logs
// @Tags         Evaluation
// @Accept       json
// @Produce      json
// @Param        id path string true "evaluation job id or task id"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /evaluations/{id}/logs [get]
func (h *EvaluationHandler) GetLogs(ctx *gin.Context) {
	since := ctx.Query("since")
	currentUser := httpbase.GetCurrentUser(ctx)
	stream := ctx.Query("stream")
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		httpbase.BadRequest(ctx, "id is required")
		return
	}

	req := types.EvaluationLogReq{
		CurrentUser: currentUser,
		Since:       since,
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		req.TaskID = idStr
	} else {
		req.ID = id
	}

	if strings.Trim(stream, " ") == "true" {
		h.readLogInStream(ctx, req)
		return
	}
	h.readLogNonStream(ctx, req)
}

func (h *EvaluationHandler) readLogNonStream(ctx *gin.Context, req types.EvaluationLogReq) {
	logs, err := h.evaluation.ReadJobLogsNonStream(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get evaluation job non-stream logs", slog.Any("error", err), slog.Any("req", req))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, logs)
}

func (h *EvaluationHandler) readLogInStream(ctx *gin.Context, req types.EvaluationLogReq) {
	logReader, err := h.evaluation.ReadJobLogsInStream(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get evaluation job in-stream logs", slog.Any("error", err), slog.Any("req", req))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any evaluation job log"))
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
			if !ok {
				return
			}
			ctx.SSEvent("Container", string(data))
			ctx.Writer.Flush()
		case <-heartbeatTicker.C:
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			time.Sleep(time.Second)
		}
	}
}
