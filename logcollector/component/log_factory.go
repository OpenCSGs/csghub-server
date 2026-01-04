package component

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/logcollector/types"
)

// LogFactory defines the interface for the log collection component.
// It serves as the main entry point for external consumers.
type LogFactory interface {
	Start() error
	Stop()
	GetStats() map[string]*types.CollectorStats
}

// LogFactory acts as the "factory" that produces and manages "product" instances (logCollectorManager).
// It implements the LogCollector interface and is the sole public-facing struct.
type logFactory struct {
	ctx    context.Context
	cancel context.CancelFunc

	// workers holds the "products" (logCollectorManager instances) produced by the factory.
	workers map[string]LogCollectorManager
}

func NewLogFactory(ctx context.Context, config *config.Config) (LogFactory, error) {
	clusterPool, err := cluster.NewClusterPool(config)
	if err != nil {
		return nil, err
	}

	// Initialize the factory.
	factoryCtx, factoryCancel := context.WithCancel(context.Background())
	lf := &logFactory{
		ctx:     factoryCtx,
		cancel:  factoryCancel,
		workers: make(map[string]LogCollectorManager),
	}

	// The factory "produces" a worker for each cluster.
	clusters := clusterPool.GetAllCluster()
	for _, cluster := range clusters {
		// Each worker gets its own context, derived from the factory's context.
		// This allows the factory to stop all workers centrally.
		worker, err := NewLogCollector(ctx, config, cluster)
		if err != nil {
			return nil, err
		}
		lf.workers[cluster.ID] = worker
	}

	return lf, nil
}

// Start delegates the start command to all worker instances.
func (lf *logFactory) Start() error {
	slog.Info("Log factory starting all workers...")
	for clusterID, worker := range lf.workers {
		err := worker.Start(lf.ctx) // Use a private start method for the worker
		if err != nil {
			slog.Error("Error starting worker:", slog.Any("clusterID", clusterID), slog.Any("error", err))
			return err
		}
	}
	slog.Info("All workers have been instructed to start.")
	return nil
}

// Stop stops the factory and all its workers by canceling the shared parent context.
func (lf *logFactory) Stop() {
	slog.Info("Log factory stopping all workers...")
	lf.cancel()
	for clusterID, worker := range lf.workers {
		err := worker.Stop()
		if err != nil {
			slog.Error("Error starting worker:", slog.Any("clusterID", clusterID), slog.Any("error", err))
		}
	}
	slog.Info("All workers have been instructed to stop.")
}

// GetStats aggregates statistics from all workers.
func (lf *logFactory) GetStats() map[string]*types.CollectorStats {
	var allClusterStats = make(map[string]*types.CollectorStats)
	for clusterID, worker := range lf.workers {
		allClusterStats[clusterID] = worker.GetStats()
	}
	return allClusterStats
}
