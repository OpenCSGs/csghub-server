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

func NewFinetuneHandler(config *config.Config) (*FinetuneHandler, error) {
	ftComp, err := component.NewFinetuneComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	agentComp, err := component.NewAgentComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating agent component:%w", err)
	}
	return &FinetuneHandler{
		ftComp:         ftComp,
		sensitive:      sc,
		agentComponent: agentComp,
	}, nil
}

type FinetuneHandler struct {
	ftComp         component.FinetuneComponent
	sensitive      component.SensitiveComponent
	agentComponent component.AgentComponent
}

func (h *FinetuneHandler) createAgentInstanceTask(ctx context.Context, agent, taskID, username string) {
	if agent == "" {
		return
	}
	if err := h.agentComponent.CreateTaskIfInstanceExists(ctx, &types.AgentInstanceTaskReq{
		TaskID:   taskID,
		Agent:    agent,
		Type:     types.AgentTaskTypeFinetuneJob,
		Username: username,
	}); err != nil {
		slog.Warn("failed to create agent instance task",
			slog.String("task_id", taskID),
			slog.String("type", types.AgentTaskTypeFinetuneJob.String()),
			slog.String("agent", agent),
			slog.Any("error", err),
		)
	}
}
