package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	utils "opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/moderation/component"
)

const PRINT_STRING_LEN = 1000

type SensitiveHandler struct {
	c component.SensitiveComponent
}

func NewSensitiveHandler(cfg *config.Config) (*SensitiveHandler, error) {
	return &SensitiveHandler{
		c: component.NewSensitiveComponentFromConfig(cfg),
	}, nil
}

func (h *SensitiveHandler) Text(ctx *gin.Context) {
	type req struct {
		Scenario types.SensitiveScenario `json:"scenario"`
		Text     string                  `json:"text"`
	}
	var (
		r   req
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		r.Text = utils.TruncStringByRune(r.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "Bad request format", slog.String("err", err.Error()), slog.Any("req", r))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	result, err := h.c.PassTextCheck(ctx, r.Scenario, r.Text)
	if err != nil {
		r.Text = utils.TruncStringByRune(r.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "Failed to pass text check", slog.String("err", err.Error()), slog.Any("req", r))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, result)

}

func (h *SensitiveHandler) Image(ctx *gin.Context) {
	type req struct {
		Scenario      types.SensitiveScenario `json:"scenario"`
		OssBucketName string                  `json:"oss_bucket_name"`
		OssObjectName string                  `json:"oss_object_name"`
		ImageURL      string                  `json:"image_url"`
	}
	var (
		r   req
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		slog.ErrorContext(ctx, "Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var result *sensitive.CheckResult
	if r.ImageURL != "" {
		result, err = h.c.PassImageURLCheck(ctx, r.Scenario, r.ImageURL)
	} else if r.OssBucketName != "" && r.OssObjectName != "" {
		result, err = h.c.PassImageCheck(ctx, r.Scenario, r.OssBucketName, r.OssObjectName)
	} else {
		httpbase.BadRequest(ctx, "image_url or oss_bucket_name and oss_object_name are required")
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "Failed to pass image check", slog.String("err", err.Error()), slog.Any("req", r))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, result)
}

func (h *SensitiveHandler) LlmResp(ctx *gin.Context) {
	var req types.LLMCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		req.Text = utils.TruncStringByRune(req.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "Bad request format", slog.String("err", err.Error()), slog.Any("req", req))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	result, err := h.c.PassStreamCheck(ctx, &req)
	if err != nil {
		req.Text = utils.TruncStringByRune(req.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "Failed to pass LLM stream check", slog.String("err", err.Error()), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}

func (h *SensitiveHandler) LlmPrompt(ctx *gin.Context) {
	var req types.LLMCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx, "Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	result, err := h.c.PassLLMQueryCheck(ctx, &req)
	if err != nil {
		req.Text = utils.TruncStringByRune(req.Text, PRINT_STRING_LEN)
		slog.ErrorContext(ctx, "Failed to pass LLM query check", slog.String("err", err.Error()), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, result)
}
