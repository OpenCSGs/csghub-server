package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
)

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
		slog.Error("Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	result, err := h.c.PassTextCheck(ctx, r.Scenario, r.Text)
	if err != nil {
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
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, result)
}

func (h *SensitiveHandler) LlmResp(ctx *gin.Context) {
	type request struct {
		Service           string `json:"Service"`
		ServiceParameters struct {
			Content   string `json:"content"`
			SessionId string `json:"sessionId"`
		} `json:"ServiceParameters"`
	}
	var (
		r   request
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		slog.Error("Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	result, err := h.c.PassStreamCheck(ctx, types.ScenarioLLMResModeration, r.ServiceParameters.Content, r.ServiceParameters.SessionId)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}

func (h *SensitiveHandler) LlmPrompt(ctx *gin.Context) {
	type request struct {
		Service           string `json:"Service"`
		ServiceParameters struct {
			Content   string `json:"content"`
			AccountId string `json:"accountId"`
		} `json:"ServiceParameters"`
	}
	var (
		r   request
		err error
	)
	if err = ctx.ShouldBindJSON(&r); err != nil {
		slog.Error("Bad request format", slog.String("err", err.Error()))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	result, err := h.c.PassLLMQueryCheck(ctx, types.ScenarioLLMQueryModeration, r.ServiceParameters.Content, r.ServiceParameters.AccountId)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}
