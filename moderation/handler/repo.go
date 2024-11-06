package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
	"opencsg.com/csghub-server/moderation/workflow"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

type RepoHandler struct {
	rc     *component.RepoComponent
	config *config.Config
}

func NewRepoHandler(config *config.Config) (*RepoHandler, error) {
	c, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}

	return &RepoHandler{
		rc:     c,
		config: config,
	}, nil
}

func (h *RepoHandler) FullCheck(c *gin.Context) {
	type request struct {
		Namespace string               `json:"namespace"`
		Name      string               `json:"name"`
		RepoType  types.RepositoryType `json:"repo_type"`
	}

	var req request
	// binding request and check error
	if err := c.BindJSON(&req); err != nil {
		slog.Error("invalid request for full check", slog.Any("error", err))
		httpbase.BadRequest(c, err.Error())
		return
	}

	//start workflow to do full check
	workflowClient := workflow.GetWorkflowClient()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: "moderation_repo_full_check_queue",
	}

	we, err := workflowClient.ExecuteWorkflow(context.Background(), workflowOptions, workflow.RepoFullCheckWorkflow,
		common.Repo{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}, h.config)
	if err != nil {
		httpbase.ServerError(c, fmt.Errorf("failed to start repo full check workflow, error: %w", err))
		return
	}

	slog.Info("start repo full check workflow", slog.String("workflow_id", we.GetID()))
	httpbase.OK(c, nil)
}
