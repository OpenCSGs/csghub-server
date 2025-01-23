package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
	"opencsg.com/csghub-server/dataviewer/component"
)

type CallbackHandler struct {
	callbackComp component.CallbackComponent
}

func NewCallBackHandler(cfg *config.Config, tc temporal.Client, gs gitserver.GitServer) (*CallbackHandler, error) {
	callbackComp, err := component.NewCallbackComponent(cfg, tc, gs)
	if err != nil {
		return nil, err
	}
	return &CallbackHandler{
		callbackComp: callbackComp,
	}, nil
}

func (h *CallbackHandler) Callback(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad repo request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	branch := ctx.Param("branch")

	req := types.UpdateViewerReq{
		Namespace:   namespace,
		Name:        name,
		Branch:      branch,
		CurrentUser: currentUser,
		RepoType:    types.DatasetRepo,
	}

	if req.Branch == dvCom.ParquetBranch || req.Branch == dvCom.DuckdbBranch {
		httpbase.OK(ctx, nil)
	}

	resp, err := h.callbackComp.TriggerDataviewUpdateWorkflow(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("fail to trigger workflow", slog.Any("req", req), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	httpbase.OK(ctx, resp)
}
