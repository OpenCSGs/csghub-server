package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewInferenceArchHandler() (*InferenceArchHandler, error) {
	inferenceArchComponent := component.NewInferenceArchComponent()
	return &InferenceArchHandler{
		inferenceArch: inferenceArchComponent,
	}, nil
}

type InferenceArchHandler struct {
	inferenceArch component.InferenceArchComponent
}

// GetInferenceArch godoc
// @Summary Get inference arch configuration
// @Description Get the inference arch configuration
// @Tags inference-arch
// @Accept json
// @Produce json
// @Success 200 {object} types.InferenceArch
// @Router /api/v1/inference-arch [get]
func (h *InferenceArchHandler) GetInferenceArch(ctx *gin.Context) {
	arch, err := h.inferenceArch.GetInferenceArch(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get inference arch", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, arch)
}

// UpdateInferenceArch godoc
// @Summary Update inference arch configuration
// @Description Update the inference arch configuration (only one record allowed)
// @Tags inference-arch
// @Accept json
// @Produce json
// @Param request body types.CreateInferenceArchReq true "Update inference arch request"
// @Success 200 {object} types.InferenceArch
// @Router /api/v1/inference-arch [put]
func (h *InferenceArchHandler) UpdateInferenceArch(ctx *gin.Context) {
	var req types.CreateInferenceArchReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	arch, err := h.inferenceArch.UpdateInferenceArch(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update inference arch", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, arch)
}
