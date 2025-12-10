package handler

import (
	"fmt"

	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type LfsHandler struct {
	lfsComp component.LfsComponent
}

func NewLfsHandler(config *config.Config, mqFactory bldmq.MessageQueueFactory) (*LfsHandler, error) {
	lfsComp, err := component.NewLfsComponent(config, mqFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create lfs component error: %w", err)
	}
	err = lfsComp.DispatchLfsXnetProcessed()
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch lfs xnet processed event error: %w", err)
	}
	return &LfsHandler{
		lfsComp: lfsComp,
	}, nil
}
