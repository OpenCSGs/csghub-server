package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewNotebookHandler(config *config.Config) (*NotebookHandler, error) {
	nc, err := component.NewNotebookComponent(config)
	if err != nil {
		return nil, err
	}
	return &NotebookHandler{
		nc:     nc,
		config: config,
	}, nil
}

type NotebookHandler struct {
	nc     component.NotebookComponent
	config *config.Config
}

// CreateNotebook godoc
// @Summary      Create notebook
// @Description  create notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        body body types.CreateNotebookReq false "body"
// @Success      200  {object}  types.NotebookRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks [post]
func (h *NotebookHandler) Create(ctx *gin.Context) {
	var req types.CreateNotebookReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Failed to bind json", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req.CurrentUser = currentUser
	notebook, err := h.nc.CreateNotebook(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to create notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, notebook)
}

// GetNotebook godoc
// @Summary      Get notebook
// @Description  get notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.NotebookRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [get]
func (h *NotebookHandler) Get(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req := &types.GetNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}

	notebook, err := h.nc.GetNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, notebook)
}

// StartNotebook godoc
// @Summary      Start notebook
// @Description  start notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id}/start [put]
func (h *NotebookHandler) Start(ctx *gin.Context) {

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.StartNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.StartNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to start notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// StopNotebook godoc
// @Summary      Stop notebook
// @Description  stop notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id}/stop [put]
func (h *NotebookHandler) Stop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.StopNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.StopNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to stop notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteNotebook godoc
// @Summary      Delete notebook
// @Description  delete notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [delete]
func (h *NotebookHandler) Delete(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.DeleteNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.DeleteNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to delete notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// UpdateNotebook godoc
// @Summary      Update notebook
// @Description  update notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Param        body body types.UpdateNotebookReq false "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [put]
func (h *NotebookHandler) Update(ctx *gin.Context) {
	var req types.UpdateNotebookReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Failed to bind json", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.ID = id
	currentUser := httpbase.GetCurrentUser(ctx)
	req.CurrentUser = currentUser
	err = h.nc.UpdateNotebook(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to update notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
