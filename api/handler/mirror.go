package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

// create new MirrorHandler
func NewMirrorHandler(config *config.Config) (*MirrorHandler, error) {
	mc, err := component.NewMirrorComponent(config)
	if err != nil {
		return nil, err
	}
	return &MirrorHandler{
		mirror: mc,
	}, nil
}

type MirrorHandler struct {
	mirror component.MirrorComponent
}

// CreateMirrorRepo godoc
// @Security     ApiKey
// @Summary      Create mirror repo
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        body body types.CreateMirrorRepoReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/repo [post]
func (h *MirrorHandler) CreateMirrorRepo(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req types.CreateMirrorRepoReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.CurrentUser = currentUser
	m, err := h.mirror.CreateMirrorRepo(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to create mirror repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	if m != nil {
		slog.Debug("create mirror repo", slog.Any("mirror", m.Repository), slog.Any("req", req))
	}

	httpbase.OK(ctx, nil)
}

// GetMirrorRepos godoc
// @Security     ApiKey
// @Summary      Get mirror repos, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.Response{data=[]types.MirrorRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/repos [get]
func (h *MirrorHandler) Repos(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repos, total, err := h.mirror.Repos(ctx.Request.Context(), per, page)
	if err != nil {
		slog.Error("failed to get mirror repos", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  repos,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// GetMirrors godoc
// @Security     ApiKey
// @Summary      Get mirrors, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.Response{data=[]types.Mirror,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirrors [get]
func (h *MirrorHandler) Index(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	search := ctx.Query("search")
	repos, total, err := h.mirror.Index(ctx.Request.Context(), per, page, search)
	if err != nil {
		slog.Error("failed to get mirror repos", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  repos,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// GetMirrorStatusCounts godoc
// @Security     ApiKey
// @Summary      Get mirror status counts
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]types.MirrorStatusCount} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/statistics [get]
func (h *MirrorHandler) Statistics(ctx *gin.Context) {
	statusCounts, err := h.mirror.Statistics(ctx.Request.Context())
	if err != nil {
		slog.Error("failed to get mirror statistics", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, statusCounts)
}

// BatchCreateMirrors godoc
// @Security     ApiKey
// @Summary      Batch create mirrors
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        body body types.BatchCreateMirrorReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/batch [post]
func (h *MirrorHandler) BatchCreate(ctx *gin.Context) {
	var req types.BatchCreateMirrorReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.mirror.BatchCreate(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to bluk create mirrors", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// GetMirrorTasks godoc
// @Security     ApiKey
// @Summary      Batch create mirrors
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        count query int false "count" default(10)
// @Success      200  {object}  types.Response{data=[]types.MirrorListResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/tasks [get]
func (h *MirrorHandler) Tasks(ctx *gin.Context) {
	count := ctx.Query("count")
	if count == "" {
		count = "10"
	}

	countInt, err := strconv.ParseInt(count, 10, 64)
	if err != nil {
		return
	}
	resp, err := h.mirror.ListQueue(ctx.Request.Context(), countInt)
	if err != nil {
		slog.Error("failed to get mirror tasks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// DeleteMirrorTasks godoc
// @Security     ApiKey
// @Summary      Batch create mirrors
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path int false "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/:id [delete]
func (h *MirrorHandler) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.mirror.Delete(ctx.Request.Context(), idInt)
	if err != nil {
		slog.Error("failed to delete mirror tasks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
