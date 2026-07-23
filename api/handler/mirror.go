package handler

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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
// @Failure      404  {object}  types.APINotFound "Not found"
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
	if strings.TrimSpace(req.ForkNamespace) == "" || strings.TrimSpace(req.ForkName) == "" {
		httpbase.BadRequest(ctx, "fork_namespace and fork_name are required")
		return
	}

	req.CurrentUser = currentUser
	m, err := h.mirror.CreateMirrorRepo(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to create mirror repo", slog.Any("error", err))
		if errors.Is(err, errorx.ErrMirrorSourceRepoAuthInvalid) || errors.Is(err, errorx.ErrBadRequest) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrRepoNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repos, total, err := h.mirror.Repos(ctx.Request.Context(), per, page)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get mirror repos", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  repos,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// Index godoc
// @Security     ApiKey
// @Summary      Get mirrors, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Param        search query string false "search"
// @Param        status query string false "status" Enums(all, waiting, running)
// @Success      200  {object}  types.Response{data=[]types.MirrorSyncSummary,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirrors [get]
func (h *MirrorHandler) Index(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := types.MirrorSyncListReq{Page: page, Per: per, Search: ctx.Query("search")}
	switch status := strings.TrimSpace(ctx.Query("status")); status {
	case "", "all":
	case string(types.MirrorSyncOverallWaiting):
		req.Status = types.MirrorSyncOverallWaiting
	case string(types.MirrorSyncOverallRunning):
		req.Status = types.MirrorSyncOverallRunning
	default:
		httpbase.BadRequest(ctx, "status must be one of all, waiting, or running")
		return
	}
	result, err := h.mirror.ListMirrorSyncs(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list mirror syncs", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  result.Items,
		"total": result.Total,
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
		slog.ErrorContext(ctx.Request.Context(), "failed to get mirror statistics", slog.Any("error", err))
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
		slog.ErrorContext(ctx.Request.Context(), "failed to bluk create mirrors", slog.Any("error", err))
		if errors.Is(err, errorx.ErrMirrorSourceRepoAuthInvalid) || errors.Is(err, errorx.ErrBadRequest) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
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
		slog.ErrorContext(ctx.Request.Context(), "failed to delete mirror tasks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// ResolveNamespace godoc
// @Security     ApiKey
// @Summary      Resolve target namespace and name from source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        source_namespace query string true "source namespace"
// @Param        source_name query string true "source name"
// @Param        repo_type query string true "repo type (model, dataset, code)"
// @Success      200  {object}  types.Response{data=types.ResolveNamespaceResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/namespace/resolve [get]
func (h *MirrorHandler) ResolveNamespace(ctx *gin.Context) {
	var req types.ResolveNamespaceReq
	err := ctx.ShouldBindQuery(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	resp, err := h.mirror.ResolveNamespace(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to resolve namespace", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}
