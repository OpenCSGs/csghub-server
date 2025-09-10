package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type HeartbeatExecutor interface {
	UpdateClusterStatus(ctx context.Context, eventData *types.HearBeatEvent) error
}

type heartbeatExecutorImpl struct {
	cfg          *config.Config
	clusterStore database.ClusterInfoStore
}

var _ HeartbeatExecutor = (*heartbeatExecutorImpl)(nil)
var _ WebHookExecutor = (*heartbeatExecutorImpl)(nil)

func NewHeartbeatExecutor(config *config.Config) (HeartbeatExecutor, error) {
	executor := &heartbeatExecutorImpl{
		cfg:          config,
		clusterStore: database.NewClusterInfoStore(),
	}
	// register the heartbeat executor for webhook callback func ProcessEvent
	err := RegisterWebHookExecutor(types.RunnerHeartbeat, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register heartbeat executor: %w", err)
	}
	return executor, nil
}

func (h *heartbeatExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	slog.Info("heartbeat event invoked", slog.Any("event", event))

	eventData := &types.HearBeatEvent{}

	err := json.Unmarshal(event.Data, &eventData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal webhook heartbeat event error: %w", err)
	}

	return h.UpdateClusterStatus(ctx, eventData)
}

func (h *heartbeatExecutorImpl) UpdateClusterStatus(ctx context.Context, eventData *types.HearBeatEvent) error {
	// update cluster status in database
	err := h.clusterStore.BatchUpdateStatus(ctx, eventData)
	if err != nil {
		return fmt.Errorf("failed to batch update cluster in heartbeat event error: %w", err)
	}
	return nil
}
