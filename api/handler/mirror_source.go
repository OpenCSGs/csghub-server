package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewMirrorSourceHandler(config *config.Config) (*MirrorSourceHandler, error) {
	c, err := component.NewMirrorSourceComponent(config)
	if err != nil {
		return nil, err
	}
	return &MirrorSourceHandler{
		c: c,
	}, nil
}

type MirrorSourceHandler struct {
	c *component.MirrorSourceComponent
}

// CreateMirrorSource godoc
// @Security     ApiKey
// @Summary      Create mirror source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        body body types.CreateMirrorSourceReq true "body"
// @Success      200  {object}  types.Response{data=database.MirrorSource} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/sources [post]
func (h *MirrorSourceHandler) Create(ctx *gin.Context) {
	var msReq types.CreateMirrorSourceReq
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	msReq.CurrentUser = currentUser
	ms, err := h.c.Create(ctx, msReq)
	if err != nil {
		slog.Error("Failed to create mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// GetMirrorSources godoc
// @Security     ApiKey
// @Summary      Get mirror sources
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]database.MirrorSource} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/sources [get]
func (h *MirrorSourceHandler) Index(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	ms, err := h.c.Index(ctx, currentUser)
	if err != nil {
		slog.Error("Failed to get mirror sources", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// UpdateMirrorSource godoc
// @Security     ApiKey
// @Summary      Update mirror source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Param        body body types.UpdateMirrorSourceReq true "body"
// @Success      200  {object}  types.Response{data=database.MirrorSource} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/sources/{id} [put]
func (h *MirrorSourceHandler) Update(ctx *gin.Context) {
	var msReq types.UpdateMirrorSourceReq
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msReq.ID = msId
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	msReq.CurrentUser = currentUser
	ms, err := h.c.Update(ctx, msReq)
	if err != nil {
		slog.Error("Failed to get mirror sources", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// GetMirrorSource godoc
// @Security     ApiKey
// @Summary      Get mirror source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=database.MirrorSource} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/sources/{id} [get]
func (h *MirrorSourceHandler) Get(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	ms, err := h.c.Get(ctx, msId, currentUser)
	if err != nil {
		slog.Error("Failed to get mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// DeleteMirrorSource godoc
// @Security     ApiKey
// @Summary      Delete mirror source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/sources/{id} [delete]
func (h *MirrorSourceHandler) Delete(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	err = h.c.Delete(ctx, msId, currentUser)
	if err != nil {
		slog.Error("Failed to delete mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
