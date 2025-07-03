package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/runner/component"
)

type ArgoHandler struct {
	clusterPool        *cluster.ClusterPool
	workflowNameSpace  string
	modelDockerRegBase string
	config             *config.Config
	wfc                component.WorkFlowComponent
}

func NewArgoHandler(config *config.Config, clusterPool *cluster.ClusterPool) (*ArgoHandler, error) {
	wfc := component.NewWorkFlowComponent(config, clusterPool)
	//watch workflows events
	go wfc.RunInformer(clusterPool, config)
	return &ArgoHandler{
		clusterPool:        clusterPool,
		config:             config,
		wfc:                wfc,
		modelDockerRegBase: config.Model.DockerRegBase,
		workflowNameSpace:  config.Argo.Namespace,
	}, nil
}

// create workflow
func (a *ArgoHandler) CreateWorkflow(ctx *gin.Context) {
	var req types.ArgoWorkFlowReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("bad order request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf, err := a.wfc.CreateWorkflow(ctx, req)
	if err != nil {
		slog.Error("fail to create workflow", slog.Any("error", err), slog.Any("req", req))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res := types.ArgoWorkFlowRes{
		ID:         wf.ID,
		Username:   wf.Username,
		TaskName:   wf.TaskName,
		TaskId:     wf.TaskId,
		TaskType:   wf.TaskType,
		TaskDesc:   wf.TaskDesc,
		RepoIds:    wf.RepoIds,
		RepoType:   wf.RepoType,
		SubmitTime: wf.SubmitTime,
	}

	ctx.JSON(http.StatusOK, res)
}

// list workflows
func (a *ArgoHandler) ListWorkflows(ctx *gin.Context) {
	username := ctx.Query("username")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	wfs, total, err := a.wfc.FindWorkFlows(ctx, username, per, page)
	if err != nil {
		slog.Error("fail to list workflows", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"total": total,
		"list":  wfs,
	})
}

func (a *ArgoHandler) DeleteWorkflow(ctx *gin.Context) {
	id := ctx.Param("id")
	var req = &types.ArgoWorkFlowDeleteReq{}
	err := ctx.BindJSON(req)
	if err != nil {
		slog.Error("bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idInt64, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("fail to convert id to int64", slog.Any("error", err), slog.Any("id", id))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = a.wfc.DeleteWorkflow(ctx, idInt64, req.Username)
	if err != nil {
		slog.Error("fail to delete workflow", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	slog.Info("Deleted argo workflow successfully", slog.String("id", id))
	httpbase.OK(ctx, nil)
}

func (a *ArgoHandler) GetWorkflow(ctx *gin.Context) {
	id := ctx.Param("id")
	var req = &types.ArgoWorkFlowGetReq{}
	err := ctx.BindJSON(req)
	if err != nil {
		slog.Error("bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	idInt64, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("fail to convert id to int64", slog.Any("error", err), slog.Any("id", id))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wf, err := a.wfc.GetWorkflow(ctx, idInt64, req.Username)
	if err != nil {
		slog.Error("fail to get workflow", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res := types.ArgoWorkFlowRes{
		ID:           wf.ID,
		Username:     wf.Username,
		TaskName:     wf.TaskName,
		Image:        wf.Image,
		TaskId:       wf.TaskId,
		TaskType:     wf.TaskType,
		TaskDesc:     wf.TaskDesc,
		RepoIds:      wf.RepoIds,
		RepoType:     wf.RepoType,
		SubmitTime:   wf.SubmitTime,
		StartTime:    wf.StartTime,
		EndTime:      wf.EndTime,
		Datasets:     wf.Datasets,
		ResultURL:    wf.ResultURL,
		DownloadURL:  wf.DownloadURL,
		FailuresURL:  wf.FailuresURL,
		Status:       wf.Status,
		Reason:       wf.Reason,
		ResourceId:   wf.ResourceId,
		ResourceName: wf.ResourceName,
	}
	ctx.JSON(http.StatusOK, res)
}
