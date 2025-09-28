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

var _ WebHookExecutor = (*clusterExecutorImpl)(nil)

type ClusterExecutor interface {
	UpdateCluster(ctx context.Context, cluster types.ClusterEvent) error
}

type clusterExecutorImpl struct {
	cfg          *config.Config
	clusterStore database.ClusterInfoStore
}

func NewClusterExecutor(config *config.Config) (WebHookExecutor, error) {
	executor := &clusterExecutorImpl{
		cfg:          config,
		clusterStore: database.NewClusterInfoStore(),
	}
	// register the cluster executor for webhook callback func ProcessEvent
	err := RegisterWebHookExecutor(types.RunnerClusterCreate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register heartbeat executor: %w", err)
	}
	err = RegisterWebHookExecutor(types.RunnerClusterUpdate, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to register heartbeat executor: %w", err)
	}
	return executor, nil
}

func (h *clusterExecutorImpl) ProcessEvent(ctx context.Context, event *types.WebHookRecvEvent) error {
	slog.Info("cluster_event_received", slog.Any("event", event))
	//parse event data to types.ClusterEvent
	var clusterEvent types.ClusterEvent
	err := json.Unmarshal(event.Data, &clusterEvent)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cluster event data: %w", err)
	}
	slog.Info("processing cluster event", slog.Any("event", clusterEvent))
	switch event.EventType {
	case types.RunnerClusterCreate:
		err = h.AddCluster(ctx, clusterEvent)
		if err != nil {
			return fmt.Errorf("failed to add cluster: %w", err)
		}
	case types.RunnerClusterUpdate:
		err = h.UpdateCluster(ctx, clusterEvent)
		if err != nil {
			return fmt.Errorf("failed to update cluster: %w", err)
		}
	default:
		return fmt.Errorf("unsupported cluster event type: %s", event.EventType)
	}

	return err
}

func (h *clusterExecutorImpl) AddCluster(ctx context.Context, cluster types.ClusterEvent) error {
	_, err := h.clusterStore.AddByClusterID(ctx, cluster.ClusterID, cluster.Region)
	return err
}

func (h *clusterExecutorImpl) UpdateCluster(ctx context.Context, cluster types.ClusterEvent) error {
	err := h.clusterStore.UpdateByClusterID(ctx, cluster)
	return err
}
