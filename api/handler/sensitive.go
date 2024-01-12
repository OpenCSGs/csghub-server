package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type SensitiveHandler struct {
	c component.SensitiveChecker
}

func NewSensitiveHandler(cfg *config.Config) *SensitiveHandler {
	return &SensitiveHandler{
		c: component.NewSensitiveComponent(cfg),
	}
}

func (h *SensitiveHandler) Text(ctx *gin.Context) {
	type req struct {
		Scenario string `json:"scenario"`
		Text     string `json:"text"`
	}
	var (
		r   req
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		slog.Error("Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ok, err := h.c.CheckText(ctx, r.Scenario, r.Text)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	if ok {
		httpbase.OK(ctx, nil)
	} else {
		httpbase.BadRequest(ctx, "sensitive content detected")
	}

}

func (h *SensitiveHandler) Image(ctx *gin.Context) {
	type req struct {
		Scenario      string `json:"scenario"`
		OssBucketName string `json:"oss_bucket_name"`
		OssObjectName string `json:"oss_object_name"`
	}
	var (
		r   req
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		slog.Error("Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	ok, err := h.c.CheckImage(ctx, r.Scenario, r.OssBucketName, r.OssObjectName)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	if ok {
		httpbase.OK(ctx, nil)
	} else {
		httpbase.BadRequest(ctx, "sensitive content detected")
	}

}
