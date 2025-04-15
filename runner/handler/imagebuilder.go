package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/runner/types"
)

type ImagebuilderHandler struct {
	ibc component.ImagebuilderComponent
}

func NewImagebuilderHandler(ctx context.Context, config *config.Config, clusterPool *cluster.ClusterPool) (*ImagebuilderHandler, error) {
	ibc, err := component.NewImagebuilderComponent(ctx, config, clusterPool)
	if err != nil {
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
// @Param   request body types.SpaceBuilderConfig true "Build Configuration"
// @Success 200 {object} types.ImagebuilderStatusRes "Build task accepted"
// @Failure 400 {object} map[string]string "Invalid request format"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /imagebuilder/build [post]
func (ibh *ImagebuilderHandler) Build(ctx *gin.Context) {
	var spaceConfig types.SpaceBuilderConfig
	if err := ctx.ShouldBindJSON(&spaceConfig); err != nil {
		slog.Error("bad params imagebuilder request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad params imagebuilder request format:" + err.Error()})
		return
	}

	ibm, err := ibh.ibc.Build(ctx.Request.Context(), spaceConfig)
	if err != nil {
		slog.Error("fail to image builder", slog.Any("error", err), slog.Any("req", spaceConfig))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "fail to imagebuilder build:" + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, types.ImagebuilderStatusRes{
		WorkName: ibm.WorkName,
		Status:   string(ibm.WorkStatus),
	})
}

// Status checks build workflow status
// @Summary Get build status
// @Description Check image building workflow status
// @Tags ImageBuilder
// @Produce json
// @Param   namespace path string true "Organization namespace"
// @Param   name path string true "Space name"
// @Param   build_id query string true "Unique build identifier"
// @Success 200 {object} map[string]interface{} "Status codes: 0=Success, 1=Failed, 2=InProgress"
// @Failure 400 {object} map[string]string "Missing build_id parameter"
// @Failure 404 {object} map[string]string "Build not found"
// @Router /imagebuilder/{namespace}/{name}/status [get]
func (ibh *ImagebuilderHandler) Status(ctx *gin.Context) {
	namespace := ctx.Param("namespace")
	name := ctx.Param("name")

	build_id := ctx.Query("build_id")
	if build_id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "imagebuilder build_id is required"})
		return
	}

	build_id = types.JointSpaceNameBuildId(namespace, name, build_id)
	ibw, err := ibh.ibc.Status(ctx.Request.Context(), build_id)
	if err != nil {
		slog.Error("fail to get image builder status", slog.Any("error", err), slog.Any("build_id", build_id))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("fail to get imagebuilder status: %s", err.Error())})
		return
	}

	var ret = make(map[string]interface{})

	switch ibw.WorkStatus {
	case string(v1alpha1.WorkflowSucceeded):
		ret[ibw.ImagePath] = 0
	case string(v1alpha1.WorkflowFailed):
		ret[ibw.ImagePath] = 1
	default:
		ret[ibw.ImagePath] = 2
	}

	ctx.JSON(http.StatusOK, ret)

}

// Logs streams real-time build logs
// @Summary Stream build logs
// @Description Get real-time logs of image building process (SSE protocol)
// @Tags ImageBuilder
// @Produce text/event-stream
// @Param   namespace path string true "Organization namespace"
// @Param   name path string true "Space name"
// @Param   build_id query string true "Unique build identifier"
// @Success 200 {string} string "Event stream with log lines"
// @Failure 400 {object} map[string]string "Missing build_id parameter"
// @Failure 500 {object} map[string]string "Log stream error"
// @Router /imagebuilder/{namespace}/{name}/logs [get]
func (ibh *ImagebuilderHandler) Logs(ctx *gin.Context) {
	namespace := ctx.Param("namespace")
	name := ctx.Param("name")
	build_id := ctx.Query("build_id")
	if build_id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "build_id is required"})
		return
	}

	build_id = types.JointSpaceNameBuildId(namespace, name, build_id)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	ch, err := ibh.ibc.Logs(ctx.Request.Context(), build_id)
	if err != nil {
		slog.Error("fail to get image builder logs", slog.Any("error", err), slog.Any("build_id", build_id))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("fail to get imagebuilder logs: %s", err.Error())})
		return
	}

	for log := range ch {
		_, err := ctx.Writer.Write(log)
		if err != nil {
			slog.Error("fail to write imagebuilder logs", slog.Any("error", err), slog.Any("build_id", build_id))
			return
		}
		ctx.Writer.Flush()
	}
}
