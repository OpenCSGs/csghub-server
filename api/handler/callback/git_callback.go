package callback

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	component "opencsg.com/csghub-server/component/callback"
)

type GitCallbackHandler struct {
	cbc    component.GitCallbackComponent
	config *config.Config
}

func NewGitCallbackHandler(config *config.Config) (*GitCallbackHandler, error) {
	cbc, err := component.NewGitCallback(config)
	if err != nil {
		return nil, err
	}
	cbc.SetRepoVisibility(true)

	return &GitCallbackHandler{cbc: cbc, config: config}, nil
}

func (h *GitCallbackHandler) Handle(c *gin.Context) {
	event := c.Request.Header.Get("X-Gitea-Event")
	switch event {
	case "push":
		h.handlePush(c)
	default:
		slog.Error("Unknown git callback event", "event", event)
		httpbase.BadRequest(c, "unknown git callback event:"+event)
	}

}

func (h *GitCallbackHandler) handlePush(c *gin.Context) {
	var req types.GiteaCallbackPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("[git_callback] Bad git callback request format", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	//start workflow to handle push request
	workflowClient := temporal.GetClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: workflow.HandlePushQueueName,
	}

	we, err := workflowClient.ExecuteWorkflow(
		c, workflowOptions, workflow.HandlePushWorkflow, &req,
	)
	if err != nil {
		slog.Error("[git_callback] failed to handle git push callback", slog.Any("error", err), slog.Any("repo_path", req.Repository.FullName))
		httpbase.ServerError(c, err)
		return
	}

	slog.Info("[git_callback] start handle push workflow", slog.String("workflow_id", we.GetID()), slog.Any("repo_path", req.Repository.FullName), slog.String("run_id", we.GetRunID()), slog.Any("req", &req))
	httpbase.OK(c, nil)
}
