package handler

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type SyncHandler struct {
	c component.MultiSyncComponent
}

func NewSyncHandler(config *config.Config) (*SyncHandler, error) {
	c, err := component.NewMultiSyncComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi sync component: %w", err)
	}
	return &SyncHandler{
		c: c,
	}, nil
}

// Latest
// @Security     ApiKey
// @Summary      Get latest version
// @Tags         Sync
// @Produce      json
// @Param        cur query string true "current version"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=types.SyncVersionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /sync/version/latest [get]
func (h *SyncHandler) Latest(ctx *gin.Context) {
	varCur := ctx.Query("cur")
	cur, err := strconv.ParseInt(varCur, 10, 64)

	if err != nil {
		httpbase.BadRequest(ctx, fmt.Sprintf("invalid param cur: %s", err.Error()))
		return
	}
	const limit int64 = 100
	versions, err := h.c.More(ctx.Request.Context(), cur, limit)
	if err != nil {
		httpbase.ServerError(ctx, fmt.Errorf("failed to get more versions: %w", err))
		return
	}

	var resp types.SyncVersionData
	resp.Versions = versions
	if len(versions) == int(limit) {
		resp.HasMore = true
	}
	httpbase.OK(ctx, resp)
}
