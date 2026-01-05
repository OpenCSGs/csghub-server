package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/component/reporter"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	ctypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/component"
)

type ImagebuilderHandler struct {
	ibc component.ImagebuilderComponent
}

func NewImagebuilderHandler(
	ctx context.Context,
	config *config.Config,
	clusterPool cluster.Pool,
	logReporter reporter.LogCollector) (*ImagebuilderHandler, error) {
	clusters := clusterPool.GetAllCluster()
	if clusterPool == nil || len(clusters) == 0 {
		slog.ErrorContext(ctx, "cluster pool is nil")
		return nil, errors.New("cluster pool is nil")
	}
	ibc, err := component.NewImagebuilderComponent(ctx, config, clusterPool, logReporter)
	if err != nil {
		slog.ErrorContext(ctx, "fail to create imagebuilder component", slog.Any("error", err))
		return nil, err
	}
	return &ImagebuilderHandler{
		ibc: ibc,
	}, nil
}

// Build triggers new image building workflow
// @Summary Create image build task
// @Description Start new Docker image building process
// @Tags ImageBuilder
// @Accept json
// @Produce json
// @Param   request body rtypes.ImageBuilderRequest true "Build Configuration"
// @Success 200 {object} rtypes.ImageBuilderResponse "Build task accepted"
// @Failure 400 {object} map[string]string "Invalid request format"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /imagebuilder/builder [post]
func (ibh *ImagebuilderHandler) Build(ctx *gin.Context) {
	var req ctypes.ImageBuilderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx, "bad params imagebuilder request format", slog.Any("error", err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad params imagebuilder request format:" + err.Error()})
		return
	}
	err := ibh.ibc.Build(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx, "fail to image builder", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "fail to imagebuilder build:" + err.Error()})
		return
	}

	httpbase.OK(ctx, nil)
}

func (ibh *ImagebuilderHandler) Stop(ctx *gin.Context) {
	var req ctypes.ImageBuildStopReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx, "bad params imagebuilder request format", slog.Any("error", err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad params imagebuilder request format:" + err.Error()})
		return
	}
	slog.InfoContext(ctx, "stop image builder", slog.Any("req", req))
	err := ibh.ibc.Stop(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx, "fail to stop image builder", slog.Any("error", err), slog.Any("deploy_id", req.DeployId))
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	slog.InfoContext(ctx, "stop image builder success", slog.Any("req", req))

	httpbase.OK(ctx, nil)
}
