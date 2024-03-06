package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewSpaceResourceHandler(config *config.Config) (*SpaceResourceHandler, error) {
	src, err := component.NewSpaceResourceComponent(config)
	if err != nil {
		return nil, err
	}
	return &SpaceResourceHandler{
		c: src,
	}, nil
}

type SpaceResourceHandler struct {
	c *component.SpaceResourceComponent
}

// GetSpaceResources godoc
// @Security     ApiKey
// @Summary      Get space resources
// @Description  get space resources
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources [get]
func (h *SpaceResourceHandler) Index(ctx *gin.Context) {
	spaceResources, err := h.c.Index(ctx)
	if err != nil {
		slog.Error("Failed to get space resources", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Get space resources successfully")
	httpbase.OK(ctx, spaceResources)
}

// CreateSpaceResource godoc
// @Security     ApiKey
// @Summary      Create space resource
// @Description  create space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceResourceReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources [post]
func (h *SpaceResourceHandler) Create(ctx *gin.Context) {
	var req *types.CreateSpaceResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	spaceResource, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create space resources", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create space resources successfully")
	httpbase.OK(ctx, spaceResource)
}

// UpdateSpaceResource godoc
// @Security     ApiKey
// @Summary      Update a exist space resource
// @Description  update a exist space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param         id path int true "id"
// @Param        body body types.UpdateSpaceResourceReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources/{id} [put]
func (h *SpaceResourceHandler) Update(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	var req *types.UpdateSpaceResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.ID = id

	spaceResource, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update space resource", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Update space resources successfully")
	httpbase.OK(ctx, spaceResource)
}

// DeleteSpaceResource godoc
// @Security     ApiKey
// @Summary      Delete a exist space resource
// @Description  delete a exist space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources/{id} [delete]
func (h *SpaceResourceHandler) Delete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	err = h.c.Delete(ctx, id)
	if err != nil {
		slog.Error("Failed to delete space resource", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Delete space resource successfully")
	httpbase.OK(ctx, nil)
}
