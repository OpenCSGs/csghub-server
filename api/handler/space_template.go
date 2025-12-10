package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewSpaceTemplateHandler(config *config.Config) (*SpaceTemplateHandler, error) {
	ssc, err := component.NewSpaceTemplateComponent(config)
	if err != nil {
		return nil, err
	}
	return &SpaceTemplateHandler{
		c: ssc,
	}, nil
}

type SpaceTemplateHandler struct {
	c component.SpaceTemplateComponent
}

// GetSpaceTemplates godoc
// @Security     ApiKey
// @Summary      Get all space templates
// @Description  Get all space templates
// @Tags         SpaceTemplate
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]database.SpaceTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_templates [get]
func (h *SpaceTemplateHandler) Index(ctx *gin.Context) {
	templates, err := h.c.Index(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get space templates", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, templates)
}

// CreateSpaceTemplate godoc
// @Security     ApiKey
// @Summary      Create space template
// @Description  create space template
// @Tags         SpaceTemplate
// @Accept       json
// @Produce      json
// @Param        body body types.SpaceTemplateReq true "body"
// @Success      200  {object}  types.Response{data=database.SpaceTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_templates [post]
func (h *SpaceTemplateHandler) Create(ctx *gin.Context) {
	var req types.SpaceTemplateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	res, err := h.c.Create(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create space template", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, res)
}

// UpdateSpaceTemplate godoc
// @Security     ApiKey
// @Summary      Update a exist space template
// @Description  update a exist space template
// @Tags         SpaceTemplate
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        body body types.UpdateSpaceTemplateReq true "body"
// @Success      200  {object}  types.Response{data=database.SpaceTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_templates/{id} [put]
func (h *SpaceTemplateHandler) Update(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	var req *types.UpdateSpaceTemplateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	res, err := h.c.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update space template", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, res)
}

// DeleteSpaceTemplate godoc
// @Security     ApiKey
// @Summary      Delete a exist space template
// @Description  delete a exist space template
// @Tags         SpaceTemplate
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_templates/{id} [delete]
func (h *SpaceTemplateHandler) Delete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.c.Delete(ctx.Request.Context(), id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete space template", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetSpaceTemplatesByType godoc
// @Security     ApiKey
// @Summary      Get space templates by type
// @Description  get space templates by type
// @Tags         SpaceTemplate
// @Accept       json
// @Produce      json
// @Param        type path int true "type"
// @Success      200  {object}  types.Response{data=[]database.SpaceTemplate} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_templates/{type} [get]
func (h *SpaceTemplateHandler) List(ctx *gin.Context) {
	templateType := ctx.Param("type")
	templates, err := h.c.FindAllByType(ctx.Request.Context(), templateType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to list space templates", slog.Any("templateType", templateType), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, templates)
}
