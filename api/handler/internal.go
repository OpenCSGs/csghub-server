package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func NewInternalHandler(config *config.Config) (*InternalHandler, error) {
	uc, err := component.NewInternalComponent(config)
	if err != nil {
		return nil, err
	}
	emptyRepoPath := filepath.Join(fmt.Sprintf("%ss", string(config.RepoTemplate.EmptyRepoType)),
		config.RepoTemplate.EmptyNameSpace, config.RepoTemplate.EmptyRepoName)
	return &InternalHandler{
		internal:       uc,
		config:         config,
		temporalClient: temporal.GetClient(),
		emptyRepoPath:  emptyRepoPath,
	}, nil
}

type InternalHandler struct {
	internal       component.InternalComponent
	config         *config.Config
	temporalClient temporal.Client
	emptyRepoPath  string
}

// TODO: add prmission check
func (h *InternalHandler) Allowed(ctx *gin.Context) {
	allowed, err := h.internal.Allowed(ctx.Request.Context())
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.PureJSON(http.StatusOK, gin.H{
		"status":  allowed,
		"message": "allowed",
	})
}

func (h *InternalHandler) SSHAllowed(ctx *gin.Context) {
	var (
		req      types.SSHAllowedReq
		rawReq   types.GitalyAllowedReq
		repoPath string
		gitEnv   types.GitEnv
	)
	if err := ctx.ShouldBind(&rawReq); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	slog.Debug("SSHAllowed check path", slog.Any("GlRepository", rawReq.GlRepository), slog.Any("emptyRepoPath", h.emptyRepoPath))
	if rawReq.GlRepository == h.emptyRepoPath {
		ctx.PureJSON(http.StatusOK, gin.H{
			"status":  true,
			"message": "allowed",
		})
		return
	}

	err := json.Unmarshal([]byte(rawReq.Env), &gitEnv)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	rawReq.GitEnv = gitEnv
	valid, err := h.internal.CheckGitCallback(ctx.Request.Context(), rawReq)

	if !valid {
		ctx.PureJSON(http.StatusOK, gin.H{
			"status":  false,
			"message": err.Error(),
		})
		return
	}

	if rawReq.Protocol == "ssh" {
		if rawReq.GlRepository != "" {
			repoPath = rawReq.GlRepository
		} else {
			repoPath = rawReq.Project
		}
		req.RepoType, req.Namespace, req.Name = getRepoInfoFronClonePath(repoPath)
		req.Action = rawReq.Action
		req.Changes = rawReq.Changes
		req.KeyID = rawReq.KeyID
		req.Protocol = rawReq.Protocol
		req.CheckIP = rawReq.CheckIP

		resp, err := h.internal.SSHAllowed(ctx.Request.Context(), req)
		if err != nil {
			httpbase.ServerError(ctx, err)
			return
		}

		ctx.PureJSON(http.StatusOK, resp)
	} else {
		ctx.PureJSON(http.StatusOK, gin.H{
			"status":  true,
			"message": "allowed",
		})
	}
}

func (h *InternalHandler) LfsAuthenticate(ctx *gin.Context) {
	var req types.LfsAuthenticateReq
	if err := ctx.ShouldBind(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.RepoType, req.Namespace, req.Name = getRepoInfoFronClonePath(req.Repo)
	resp, err := h.internal.LfsAuthenticate(ctx.Request.Context(), req)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.PureJSON(http.StatusOK, resp)
}

func (h *InternalHandler) PreReceive(ctx *gin.Context) {
	ctx.PureJSON(http.StatusOK, gin.H{
		"reference_counter_increased": true,
	})
}

func (h *InternalHandler) PostReceive(ctx *gin.Context) {
	successResp := gin.H{
		"reference_counter_decreased": true,
		"messages": []Messages{
			{
				Message: "Welcome to OpenCSG!",
				Type:    "alert",
			},
		},
	}

	var req types.PostReceiveReq
	if err := ctx.ShouldBind(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.GlRepository == h.emptyRepoPath {
		ctx.PureJSON(http.StatusOK, successResp)
		return
	}

	strs := strings.Split(req.Changes, " ")
	// the format of originalRef is refs/heads/main
	originalRef := strings.ReplaceAll(strs[2], "\n", "")
	ref := strings.Split(strs[2], "/")[2]
	// the format of ref is main
	ref = strings.ReplaceAll(ref, "\n", "")
	paths := strings.Split(req.GlRepository, "/")

	diffReq := types.GetDiffBetweenTwoCommitsReq{
		LeftCommitId:  strs[0],
		RightCommitId: strs[1],
		Namespace:     paths[1],
		Name:          paths[2],
		Ref:           ref,
		RepoType:      types.RepositoryType(strings.TrimSuffix(paths[0], "s")),
	}
	callback, err := h.internal.GetCommitDiff(ctx.Request.Context(), diffReq)
	if err != nil {
		slog.Error("post receive: failed to get commit diff", slog.Any("error", err))
		if diffReq.RightCommitId == types.NoCommitID {
			// delete branch action
			ctx.PureJSON(http.StatusOK, successResp)
			return
		} else {
			httpbase.ServerError(ctx, err)
			return
		}
	}
	callback.Ref = originalRef

	//start workflow to handle push request
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
	}

	we, err := h.temporalClient.ExecuteWorkflow(
		ctx.Request.Context(), workflowOptions, workflow.HandlePushWorkflow, callback,
	)
	if err != nil {
		slog.Error("failed to handle git push callback", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if diffReq.RepoType == types.DatasetRepo {
		h.CallDataViewer(ctx.Request.Context(), diffReq.Namespace, diffReq.Name, diffReq.Ref)
	}

	slog.Info("start handle push workflow", slog.String("workflow_id", we.GetID()), slog.Any("req", callback))

	ctx.PureJSON(http.StatusOK, successResp)
}

func (h *InternalHandler) CallDataViewer(ctx context.Context, namespace, name, branch string) {
	if branch == dvCom.ParquetBranch || branch == dvCom.DuckdbBranch {
		return
	}
	req := types.UpdateViewerReq{
		Namespace: namespace,
		Name:      name,
		Branch:    branch,
		RepoType:  types.DatasetRepo,
	}
	res, err := h.internal.TriggerDataviewerWorkflow(ctx, req)
	if err != nil {
		slog.Error("fail to read dataviewer response", slog.Any("req", req), slog.Any("err", err))
		return
	}
	slog.Info("dataviewer callback response", slog.Any("req", req), slog.Any("res", res))
}

func (h *InternalHandler) GetAuthorizedKeys(ctx *gin.Context) {
	key := ctx.Query("key")
	sshKey, err := h.internal.GetAuthorizedKeys(ctx.Request.Context(), key)
	if err != nil {
		slog.Error("failed to get authorize keys", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.PureJSON(http.StatusOK, gin.H{
		"id":  sshKey.ID,
		"key": sshKey.Content,
	})
}

type Messages struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func getRepoInfoFronClonePath(clonePath string) (repoType types.RepositoryType, namespace, name string) {
	repoWithoutSuffix := strings.TrimSuffix(clonePath, ".git")
	repoWithoutPrefix := strings.TrimPrefix(repoWithoutSuffix, "/")
	paths := strings.Split(repoWithoutPrefix, "/")
	repoType = types.RepositoryType(strings.TrimSuffix(paths[0], "s"))
	namespace = paths[1]
	name = paths[2]
	return
}
