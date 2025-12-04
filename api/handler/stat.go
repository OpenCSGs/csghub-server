package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type StatHandler struct {
	sc component.StatComponent
}

func NewStatHandler(config *config.Config) (*StatHandler, error) {
	st, err := component.NewStatComponent(config)
	if err != nil {
		return nil, err
	}
	return &StatHandler{
		sc: st,
	}, nil
}

// GetStatSnap godoc
// @Security     ApiKey
// @Summary      Get stat snapshot
// @Description  Retrieve statistical snapshot for a given target and date type
// @Tags         Stat
// @Accept       json
// @Produce      json
// @Param        target_type query string true "Target type"
// @Param        date_type   query string true "Date type" Enums(year,month,week,day)
// @Success      200  {object}  types.Response{data=types.StatSnapshotResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /stat/snapshot [get]
func (h *StatHandler) GetStatSnap(ctx *gin.Context) {
	var req types.StatSnapshotReq

	targetType := ctx.Query("target_type")
	dateType := ctx.Query("date_type")
	if !types.IsValidStatTargetType(targetType) {
		slog.ErrorContext(ctx.Request.Context(), "Bad request target_type", slog.String("target_type", targetType))
		httpbase.BadRequest(ctx, "Bad request target_type")
		return
	}
	if !types.IsValidStatDateType(dateType) {
		slog.ErrorContext(ctx.Request.Context(), "Bad request date_type", slog.String("date_type", dateType))
		httpbase.BadRequest(ctx, "Bad request date_type")
		return
	}
	req.TargetType = types.StatTargetType(targetType)
	req.DateType = types.StatDateType(dateType)

	resp, err := h.sc.GetStatSnap(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get stat snapshot", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data": resp,
	}
	ctx.JSON(http.StatusOK, respData)
}

// StatRunningDeploys godoc
// @Security     ApiKey
// @Summary      Get running deploy statistics
// @Description  Retrieve the number of running deployments, CPU usage, and GPU usage per project
// @Tags         Stat
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=map[int]types.StatRunningDeploy} "OK"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /stat/running-deploys [get]
func (h *StatHandler) StatRunningDeploys(ctx *gin.Context) {
	res, err := h.sc.StatRunningDeploys(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to stat running deploys", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to stat running deploys, %w", err))
		return
	}
	httpbase.OK(ctx, res)
}
