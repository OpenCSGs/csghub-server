//go:build !ee && !saas

package handler

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/config"
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
	return &FinetuneHandler{
		ftComp:    ftComp,
		sensitive: sc,
	}, nil
}

type FinetuneHandler struct {
	ftComp    component.FinetuneComponent
	sensitive component.SensitiveComponent
}

func (h *FinetuneHandler) createAgentInstanceTask(_ context.Context, _ string, _ string, _ string) {
}
