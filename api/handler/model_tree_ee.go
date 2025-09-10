//go:build saas || ee

package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewModelTreeHandler(config *config.Config) (*ModelTreeHandler, error) {
	mc, err := component.NewModelTreeComponent(config)
	if err != nil {
		return nil, err
	}
	return &ModelTreeHandler{
		mc:     mc,
		config: config,
	}, nil
}

type ModelTreeHandler struct {
	mc     component.ModelTreeComponent
	config *config.Config
}

// GetModelTree godoc
// @Summary      Get model tree
// @Description  get model tree
// @Tags         Model
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ModelTree "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /modeltrees/{namespace}/{name}/lineage [get]
func (h *ModelTreeHandler) Index(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	tree, err := h.mc.GetModelTree(ctx.Request.Context(), currentUser, namespace, name)
	if err != nil {
		slog.Error("Get model tree failed", "error", err)
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}
	httpbase.OK(ctx, tree)
}

// ScanModelTree godoc
// @Security     ApiKey
// @Summary      scan model tree
// @Description  scan model tree
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        body body types.ScanModels false "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /modeltrees/scan [post]
func (h *ModelTreeHandler) Scan(ctx *gin.Context) {
	req := types.ScanModels{}
	contentLength := ctx.GetHeader("Content-Length")
	if contentLength != "0" {
		err := ctx.ShouldBindJSON(&req)
		if err != nil {
			slog.Error("Failed to bind json", slog.Any("error", err))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}
	//start workflow to do full scaning
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
	}

	_, err := workflowClient.ExecuteWorkflow(
		ctx.Request.Context(), workflowOptions, workflow.ScanModelTreeWorkflow, req,
	)
	if err != nil {
		slog.Error("failed to scan model tree", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
