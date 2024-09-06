package handler

import (
	"log/slog"

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
		mc: mc,
	}, nil
}

type MirrorHandler struct {
	mc *component.MirrorComponent
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
	var req types.CreateMirrorRepoReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	req.CurrentUser = currentUser
	m, err := h.mc.CreateMirrorRepo(ctx, req)
	if err != nil {
		slog.Error("failed to create mirror repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("create mirror repo", slog.Any("mirror", m.Repository), slog.Any("req", req))

	httpbase.OK(ctx, nil)
}

// GetMirrorRepos godoc
// @Security     ApiKey
// @Summary      Get mirror repos
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
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	repos, total, err := h.mc.Repos(ctx, currentUser, per, page)
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
// @Summary      Get mirrors
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
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	repos, total, err := h.mc.Index(ctx, currentUser, per, page)
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
