package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type BroadcastHandler struct {
	ec component.BroadcastComponent
}

func NewBroadcastHandler() (*BroadcastHandler, error) {
	return &BroadcastHandler{
		ec: component.NewBroadcastComponent(),
	}, nil
}

// @Summary fetch all broadcasts
// @Tags 		Broadcasts
// @Accept 		json
// @Produce 	json
// @Success 	200	{object}  types.Response{data=[]types.Broadcast} "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /broadcasts [get]
func (h *BroadcastHandler) Index(ctx *gin.Context) {
	var broadcasts []types.Broadcast

	broadcasts, err := h.ec.AllBroadcasts(ctx.Request.Context())

	if err != nil {
		slog.Error("failed to find broadcasts", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	httpbase.OK(ctx, broadcasts)
}

// @Summary create broadcast
// @Tags 		Broadcasts
// @Accept 		json
// @Produce 	json
// @Success 	200	{object}  types.Response{data=types.Broadcast} "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /broadcasts [post]
func (h *BroadcastHandler) Create(ctx *gin.Context) {
	//TODO: authentication?

	var broadcast types.Broadcast
	if err := ctx.ShouldBindJSON(&broadcast); err != nil {
		err = fmt.Errorf("cant parse as Broadcast,%w", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if err := h.ec.NewBroadcast(ctx.Request.Context(), broadcast); err != nil {
		slog.Error("Failed to create events", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// @Summary update broadcast
// @Tags 		Broadcasts
// @Accept 		json
// @Produce 	json
// @Success 	200	{object}  types.Response{data=types.Broadcast} "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /broadcasts/:id [put]
func (h *BroadcastHandler) Update(ctx *gin.Context) {
	var broadcast types.Broadcast
	if err := ctx.ShouldBindJSON(&broadcast); err != nil {
		err = fmt.Errorf("cant parse as Broadcast,%w", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	activeBroadcast, err := h.ec.ActiveBroadcast(ctx.Request.Context())
	if err != nil {
		slog.Error("Faild to retrieve active broadcasts", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if broadcast.Status == "active" && activeBroadcast != nil && activeBroadcast.ID != id {
		slog.Error("Bad request format", "error", "Active Broadcast exits")
		httpbase.BadRequest(ctx, "Active Broadcast exits")
		return
	}

	broadcast.ID = id

	_, err = h.ec.UpdateBroadcast(ctx.Request.Context(), broadcast)

	if err != nil {
		slog.Error("Failed to update Broadcast", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, broadcast)
}

// @Summary get broadcast
// @Tags 		Broadcasts
// @Accept 		json
// @Produce 	json
// @Success 	200	{object}  types.Response{data=types.Broadcast} "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /broadcasts/:id [get]
func (h *BroadcastHandler) Show(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	broadcast, err := h.ec.GetBroadcast(ctx.Request.Context(), id)
	if err != nil {
		slog.Error("Failed to find Broadcast", slog.Any("error", err), "id", id)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, broadcast)
}

// @Summary get active broadcast
// @Tags 		Broadcasts
// @Accept 		json
// @Produce 	json
// @Success 	200	{object}  types.Response{data=types.Broadcast} "OK"
// @Failure		400	{object}  types.APIBadRequest "Bad request"
// @Failure     500	{object}  types.APIInternalServerError "Internal server error"
// @Router /broadcasts/active [get]
func (h *BroadcastHandler) Active(ctx *gin.Context) {
	broadcast, err := h.ec.ActiveBroadcast(ctx.Request.Context())
	if err != nil {
		slog.Error("Failed to find Broadcast", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, broadcast)
}
