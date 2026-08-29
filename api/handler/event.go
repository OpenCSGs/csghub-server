package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type EventHandler struct {
	ec component.EventComponent
}

func NewEventHandler() (*EventHandler, error) {
	return &EventHandler{
		ec: component.NewEventComponent(),
	}, nil
}

// @Summary Report client events
// @Tags 		Events
// @Accept 		json
// @Produce 	json
// @Param 		events body []types.Event true "Events"
// @Success 	200	{object}  object "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /events [post]
func (h *EventHandler) Create(ctx *gin.Context) {
	var events []types.Event
	if err := ctx.ShouldBindJSON(&events); err != nil {
		err = fmt.Errorf("cant parse as Event array,%w", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if len(events) > 300 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
			fmt.Errorf("too many events: %d", len(events)), nil))
		return
	}

	userUUID := httpbase.GetCurrentUserUUID(ctx)
	clientIP := ctx.ClientIP()
	for i := range events {
		if events[i].Module == "" || events[i].ID == "" {
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
				fmt.Errorf("event[%d] missing module or id", i), nil))
			return
		}
		if len(events[i].Extension) > 4096 {
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(
				fmt.Errorf("event[%d] extension too long", i), nil))
			return
		}
		// overwrite with server side info
		events[i].ClientID = userUUID
		events[i].ClientIP = clientIP
	}

	if err := h.ec.NewEvents(ctx.Request.Context(), events); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create events", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
