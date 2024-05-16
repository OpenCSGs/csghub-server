package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found in context, please login first"))
		return
	}

	var req types.CreateMirrorRepoReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	m, err := h.mc.CreateMirrorRepo(ctx, req)
	if err != nil {
		slog.Error("failed to create mirror repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Debug("create mirror repo", slog.Any("mirror", m.Repository), slog.Any("req", req))

	httpbase.OK(ctx, nil)
}
