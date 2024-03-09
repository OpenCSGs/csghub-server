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

func NewSpaceSdkHandler(config *config.Config) (*SpaceSdkHandler, error) {
	ssc, err := component.NewSpaceSdkComponent(config)
	if err != nil {
		return nil, err
	}
	return &SpaceSdkHandler{
		c: ssc,
	}, nil
}

type SpaceSdkHandler struct {
	c *component.SpaceSdkComponent
}

// GetSpaceSdks godoc
// @Security     ApiKey
// @Summary      Get space sdks
// @Description  get space sdks
// @Tags         SpaceSdk
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.SpaceSdk,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_sdks [get]
func (h *SpaceSdkHandler) Index(ctx *gin.Context) {
	spaceSdks, err := h.c.Index(ctx)
	if err != nil {
		slog.Error("Failed to get space sdks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get space sdks successfully")
	httpbase.OK(ctx, spaceSdks)
}

// CreateSpaceSdk godoc
// @Security     ApiKey
// @Summary      Create space sdk
// @Description  create space sdk
// @Tags         SpaceSdk
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceSdkReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceSdk,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_sdks [post]
func (h *SpaceSdkHandler) Create(ctx *gin.Context) {
	var req types.CreateSpaceSdkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	spaceSdk, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create space sdk", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create space sdks successfully")
	httpbase.OK(ctx, spaceSdk)
}

// UpdateSpaceSdk godoc
// @Security     ApiKey
// @Summary      Update a exist space sdk
// @Description  update a exist space sdk
// @Tags         SpaceSdk
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        body body types.UpdateSpaceSdkReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceSdk,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_sdks/{id} [put]
func (h *SpaceSdkHandler) Update(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	var req types.UpdateSpaceSdkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	spaceSdk, err := h.c.Update(ctx, &req)
	if err != nil {
		slog.Error("Failed to update space sdk", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Update space sdks successfully")
	httpbase.OK(ctx, spaceSdk)
}

// DeleteSpaceSdk godoc
// @Security     ApiKey
// @Summary      Delete a exist space sdk
// @Description  delete a exist space sdk
// @Tags         SpaceSdk
// @Accept       json
// @Produce      json
// @Param        写id path int true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_sdks/{id} [delete]
func (h *SpaceSdkHandler) Delete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.c.Delete(ctx, id)
	if err != nil {
		slog.Error("Failed to delete space sdk", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete space sdk successfully")
	httpbase.OK(ctx, nil)
}
