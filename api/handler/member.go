package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type MemberHandler struct {
	c *component.MemberComponent
}

func NewMemberHandler(config *config.Config) (*MemberHandler, error) {
	mc, err := component.NewMemberComponent(config)
	if err != nil {
		return nil, err
	}
	return &MemberHandler{
		c: mc,
	}, nil
}

func (h *MemberHandler) Index(ctx *gin.Context) {
	members, err := h.c.Index(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, members)
}

func (h *MemberHandler) Update(ctx *gin.Context) {
	member, err := h.c.Update(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, member)
}

func (h *MemberHandler) Create(ctx *gin.Context) {
	member, err := h.c.Create(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, member)
}

func (h *MemberHandler) Delete(ctx *gin.Context) {
	err := h.c.Delete(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
