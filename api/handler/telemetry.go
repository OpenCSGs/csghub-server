package handler

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types/telemetry"
	"opencsg.com/csghub-server/component"
)

type TelemetryHandler struct {
	c *component.TelemetryComponent
}

func NewTelemetryHandler() (*TelemetryHandler, error) {
	c, err := component.NewTelemetryComponent()
	if err != nil {
		return nil, fmt.Errorf("fail to create TelemetryComponent,%w", err)
	}
	return &TelemetryHandler{
		c: c,
	}, nil
}

// Usage  godoc
// @Security     ApiKey
// @Summary      Submit telemetry data for a client
// @Tags         Telemetry
// @Accept       json
// @Produce      json
// @Param        body   body  telemetry.Usage true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /telemetry/usage [post]
func (th *TelemetryHandler) Usage(ctx *gin.Context) {
	var usage telemetry.Usage
	if err := ctx.ShouldBindJSON(&usage); err != nil {
		newErr := fmt.Errorf("bad request format, %w", err).Error()
		slog.Error(newErr)
		httpbase.BadRequest(ctx, newErr)
		return
	}
	err := th.c.SaveUsageData(ctx.Request.Context(), usage)
	if err != nil {
		slog.Error("fail to save usage data", slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("fail to save usage data"))
		return
	}

	httpbase.OK(ctx, nil)
}
