//go:build ee || saas

package handler

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
	agentComp, err := component.NewAgentComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating agent component:%w", err)
	}
	return &ModelHandler{
		model:          uc,
		sensitive:      sc,
		repo:           repo,
		agentComponent: agentComp,
	}, nil
}

type ModelHandler struct {
	model          component.ModelComponent
	repo           component.RepoComponent
	sensitive      component.SensitiveComponent
	agentComponent component.AgentComponent
}

func (h *ModelHandler) createAgentInstanceTask(ctx context.Context, agent, taskID string, taskType types.AgentTaskType, username string) {
	if agent == "" {
		return
	}
	if err := h.agentComponent.CreateTaskIfInstanceExists(ctx, &types.AgentInstanceTaskReq{
		TaskID:   taskID,
		Agent:    agent,
		Type:     taskType,
		Username: username,
	}); err != nil {
		slog.Warn("failed to create agent instance task",
			slog.String("task_id", taskID),
			slog.String("type", taskType.String()),
			slog.String("agent", agent),
			slog.Any("error", err),
		)
	}
}
