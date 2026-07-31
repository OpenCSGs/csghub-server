//go:build !ee && !saas

package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewModelHandler(config *config.Config) (*ModelHandler, error) {
	uc, err := component.NewModelComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	repo, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo component:%w", err)
	}
	return &ModelHandler{
		model:     uc,
		sensitive: sc,
		repo:      repo,
	}, nil
}

type ModelHandler struct {
	model     component.ModelComponent
	repo      component.RepoComponent
	sensitive component.SensitiveComponent
}

func (h *ModelHandler) createAgentInstanceTask(_ context.Context, _ string, _ string, _ types.AgentTaskType, _ string) {
}

func (h *ModelHandler) SDKModelInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	ref := ctx.Param("ref")
	blobs, err := strconv.ParseBool(ctx.Query("blobs"))
	if err != nil {
		blobs = false
	}
	mappedBranch := ctx.Param("branch_mapped")
	if mappedBranch != "" {
		ref = mappedBranch
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	modelInfo, err := h.model.SDKModelInfo(ctx.Request.Context(), namespace, name, ref, currentUser, blobs)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get sdk model info", slog.String("namespace", namespace), slog.String("name", name), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, modelInfo)
}

func (h *ModelHandler) SDKModelTree(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format for model tree", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	ref := ctx.Param("ref")

	mappedBranch := ctx.Param("branch_mapped")
	if mappedBranch != "" {
		ref = mappedBranch
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	pathInRepo := ctx.Param("path_in_repo")
	recursive := ctx.Query("recursive") == "true"
	slog.Debug("get path_in_repo in model repo", slog.Any("pathInRepo", pathInRepo), slog.Bool("recursive", recursive))

	tree, err := h.model.SDKModelTree(ctx.Request.Context(), namespace, name, ref, currentUser, pathInRepo, recursive)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) || errors.Is(err, errorx.ErrUnauthorized) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "fail to get model tree", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.PureJSON(http.StatusOK, tree)
}


