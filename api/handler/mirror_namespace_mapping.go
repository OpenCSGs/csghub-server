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

func NewMirrorNamespaceMappingHandler(config *config.Config) (*MirrorNamespaceMappingHandler, error) {
	c, err := component.NewMirrorNamespaceMappingComponent(config)
	if err != nil {
		return nil, err
	}
	return &MirrorNamespaceMappingHandler{
		mirrorNamespaceMapping: c,
	}, nil
}

type MirrorNamespaceMappingHandler struct {
	mirrorNamespaceMapping component.MirrorNamespaceMappingComponent
}

// CreateMirrorNamespaceMapping godoc
// @Security     ApiKey
// @Summary      Create mirror namespace mapping, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        body body types.CreateMirrorNamespaceMappingReq true "body"
// @Success      200  {object}  types.Response{data=database.MirrorNamespaceMapping} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror_namespace_mappings [post]
func (h *MirrorNamespaceMappingHandler) Create(ctx *gin.Context) {
	var msReq types.CreateMirrorNamespaceMappingReq
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	ms, err := h.mirrorNamespaceMapping.Create(ctx.Request.Context(), msReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create mirror namespace mapping", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// GetMirrorNamespaceMappings godoc
// @Security     ApiKey
// @Summary      Get mirror namespace mappings, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]database.MirrorNamespaceMapping} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror_namespace_mappings [get]
func (h *MirrorNamespaceMappingHandler) Index(ctx *gin.Context) {
	search := ctx.Query("search")
	ms, err := h.mirrorNamespaceMapping.Index(ctx.Request.Context(), search)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get mirror namespace mappings", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// UpdateMirrorNamespaceMapping godoc
// @Security     ApiKey
// @Summary      Update mirror namespace mapping, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Param        body body types.UpdateMirrorNamespaceMappingReq true "body"
// @Success      200  {object}  types.Response{data=database.MirrorNamespaceMapping} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror_namespace_mappings/{id} [put]
func (h *MirrorNamespaceMappingHandler) Update(ctx *gin.Context) {
	var msReq types.UpdateMirrorNamespaceMappingReq
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror namespace mapping id")
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msReq.ID = msId
	ms, err := h.mirrorNamespaceMapping.Update(ctx.Request.Context(), msReq)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get mirror namespace mappings", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// GetMirrorNamespaceMapping godoc
// @Security     ApiKey
// @Summary      Get mirror namespace mapping, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=database.MirrorNamespaceMapping} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror_namespace_mappings/{id} [get]
func (h *MirrorNamespaceMappingHandler) Get(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror namespace mapping id")
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	ms, err := h.mirrorNamespaceMapping.Get(ctx.Request.Context(), msId)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get mirror namespace mapping", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

// DeleteMirrorNamespaceMapping godoc
// @Security     ApiKey
// @Summary      Delete mirror namespace mapping, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror_namespace_mappings/{id} [delete]
func (h *MirrorNamespaceMappingHandler) Delete(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror namespace mapping id")
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.mirrorNamespaceMapping.Delete(ctx.Request.Context(), msId)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete mirror namespace mapping", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
