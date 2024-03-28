package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewListHandler(config *config.Config) (*ListHandler, error) {
	uc, err := component.NewListComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create space component,%w", err)
	}
	return &ListHandler{
		c:  uc,
		sc: sc,
	}, nil
}

type ListHandler struct {
	c  *component.ListComponent
	sc *component.SpaceComponent
}

// ListTrendingModels   godoc
// @Security     ApiKey
// @Summary      List models by paths
// @Description  list models by paths
// @Tags         List
// @Accept       json
// @Produce      json
// @Param        body body types.ListByPathReq true "body"
// @Success      200  {object}  types.Response{data=[]types.ModelResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /list/models_by_path [post]
func (h *ListHandler) ListModelsByPath(ctx *gin.Context) {
	var listTrendingReq types.ListByPathReq
	if err := ctx.ShouldBindJSON(&listTrendingReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	resp, err := h.c.ListModelsByPath(ctx, &listTrendingReq)
	if err != nil {
		slog.Error("Failed to update dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// ListTrendingDatasets   godoc
// @Security     ApiKey
// @Summary      List datasets by paths
// @Description  list datasets by paths
// @Tags         List
// @Accept       json
// @Produce      json
// @Param        body body types.ListByPathReq true "body"
// @Success      200  {object}  types.Response{data=[]types.DatasetResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /list/datasets_by_path [post]
func (h *ListHandler) ListDatasetsByPath(ctx *gin.Context) {
	var listTrendingReq types.ListByPathReq
	if err := ctx.ShouldBindJSON(&listTrendingReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	resp, err := h.c.ListDatasetsByPath(ctx, &listTrendingReq)
	if err != nil {
		slog.Error("Failed to update dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// ListTrendingSpaces   godoc
// @Security     ApiKey
// @Summary      List spaces by paths
// @Tags         List
// @Accept       json
// @Produce      json
// @Param        body body types.ListByPathReq true "body"
// @Success      200  {object}  types.Response{data=[]types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /list/spaces_by_path [post]
func (h *ListHandler) ListSpacesByPath(ctx *gin.Context) {
	var listTrendingReq types.ListByPathReq
	if err := ctx.ShouldBindJSON(&listTrendingReq); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	resp, err := h.sc.ListByPath(ctx, listTrendingReq.Paths)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}
