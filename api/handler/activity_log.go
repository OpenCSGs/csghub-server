package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type ActivityLogHandler struct {
	comp component.ActivityLogComponent
}

func NewActivityLogHandler(comp component.ActivityLogComponent) *ActivityLogHandler {
	return &ActivityLogHandler{comp: comp}
}

func (h *ActivityLogHandler) List(ctx *gin.Context) {
	var req types.QueryActivityLogReq

	afterStr := ctx.Query("after")
	if afterStr != "" {
		after, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			httpbase.BadRequest(ctx, "invalid 'after' time format, use RFC3339")
			return
		}
		req.After = after
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, "invalid pagination params")
		return
	}
	req.Per = per
	req.Page = page

	logs, total, err := h.comp.ListActivityLogs(ctx.Request.Context(), req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OKWithTotal(ctx, logs, total)
}
