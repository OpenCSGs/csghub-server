//go:build !ee && !saas

package handler

import (
	"context"
	"fmt"

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
