package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// @Security ApiKey
// @Summary Optimize engine args
// @Description Return optimized engine args based on runtime environment and user's current values
// @Tags EngineArgs
// @Accept json
// @Produce json
// @Param namespace path string true "namespace"
// @Param name path string true "name"
// @Param body body types.OptimizeEngineArgsReq true "request body"
// @Success 200 {object} types.Response{data=types.OptimizeEngineArgsResp}
// @Router /models/{namespace}/{name}/engine_args/optimize [post]
func (h *ModelHandler) OptimizeEngineArgs(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	allow, err := h.repo.AllowReadAccess(ctx.Request.Context(), types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Info("user not allowed to optimize engine args", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user is not authorized to read this repository"))
		return
	}

	var req types.OptimizeEngineArgsReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	resp, err := h.model.OptimizeEngineArgs(ctx.Request.Context(), namespace, name, currentUser, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to optimize engine args",
			slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}
