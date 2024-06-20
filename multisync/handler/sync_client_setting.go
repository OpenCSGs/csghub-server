package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/component"
	"opencsg.com/csghub-server/multisync/types"
)

type SyncClientSettingHandler struct {
	c *component.SyncClientSettingComponent
}

func NewSyncClientSettingHandler(config *config.Config) (*SyncClientSettingHandler, error) {
	c, err := component.NewSyncClientSettingComponent(config)
	if err != nil {
		return nil, err
	}
	return &SyncClientSettingHandler{
		c: c,
	}, nil
}

func (h *SyncClientSettingHandler) Create(ctx *gin.Context) {
	var req types.CreateSyncClientSettingReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ms, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}
