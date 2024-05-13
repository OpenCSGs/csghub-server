package handler

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewMirrorSourceHandler(config *config.Config) (*MirrorSourceHandler, error) {
	c, err := component.NewMirrorSourceComponent(config)
	if err != nil {
		return nil, err
	}
	return &MirrorSourceHandler{
		c: c,
	}, nil
}

type MirrorSourceHandler struct {
	c *component.MirrorSourceComponent
}

func (h *MirrorSourceHandler) Create(ctx *gin.Context) {
	var msReq types.CreateMirrorSourceReq
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ms, err := h.c.Create(ctx, msReq)
	if err != nil {
		slog.Error("Failed to create mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

func (h *MirrorSourceHandler) Index(ctx *gin.Context) {
	ms, err := h.c.Index(ctx)
	if err != nil {
		slog.Error("Failed to get mirror sources", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

func (h *MirrorSourceHandler) Update(ctx *gin.Context) {
	var msReq types.UpdateMirrorSourceReq
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if err := ctx.ShouldBindJSON(&msReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msReq.ID = msId
	ms, err := h.c.Update(ctx, msReq)
	if err != nil {
		slog.Error("Failed to get mirror sources", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

func (h *MirrorSourceHandler) Get(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ms, err := h.c.Get(ctx, msId)
	if err != nil {
		slog.Error("Failed to get mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, ms)
}

func (h *MirrorSourceHandler) Delete(ctx *gin.Context) {
	var msId int64
	id := ctx.Param("id")
	if id == "" {
		err := fmt.Errorf("invalid mirror source id")
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	msId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.c.Delete(ctx, msId)
	if err != nil {
		slog.Error("Failed to delete mirror source", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}
