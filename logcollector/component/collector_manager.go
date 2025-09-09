package component

import (
	"context"
	"fmt"
	"log/slog"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/reporter"
	ltypes "opencsg.com/csghub-server/logcollector/types"
	"time"
)

// LogCollectorManager is the main log collector interface
type LogCollectorManager interface {
	Start(ctx context.Context) error
	Stop() error
	GetStats() *ltypes.CollectorStats
}

// logCollector implements the LogCollector interface
type logCollectorManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	lc         reporter.LogCollector
	podMonitor *PodMonitor
	cluster    *cluster.Cluster
	stats      ltypes.CollectorStats
}

// NewLogCollector creates a new log collector instance
func NewLogCollector(ctx context.Context, config *config.Config, cluster *cluster.Cluster) (LogCollectorManager, error) {
	reporterClientID := fmt.Sprintf("%s-%s", types.ClientTypeLogCollector, cluster.ID)
	logCollector, err := reporter.NewLogCollector(config, types.ClientType(reporterClientID))
	if err != nil {
		return nil, fmt.Errorf("failed to create log collector, %w", err)
	}

	// Health check Loki
	if err := logCollector.HealthCheck(); err != nil {
		return nil, err
	}

	// Get the last reported timestamp to avoid duplicates
	lastReportedTime, err := logCollector.GetSender().GetLastReportedTimestamp(ctx)
	if err != nil {
		// Log the error but continue, we will just start from the beginning
		return nil, err
	} else if !lastReportedTime.IsZero() {
		slog.Info("Will start fetching logs since last reported time", "timestamp", lastReportedTime)
	}

	collectorManager := &logCollectorManager{
		lc: logCollector,
		stats: ltypes.CollectorStats{
			NamespaceStats: make(map[string]ltypes.NamespaceStats),
			LastUpdate:     time.Now(),
		},
		cluster: cluster,
	}

	// Create pod monitor
	collectorManager.podMonitor = NewPodMonitor(
		cluster.Client,
		[]string{config.Cluster.SpaceNamespace},
		config,
		logCollector.GetLogChan(),
		lastReportedTime,
	)

	return collectorManager, nil
}

func NewAndStartLogCollector(ctx context.Context, config *config.Config, cluster *cluster.Cluster) (LogCollectorManager, error) {
	lollectorManager, err := NewLogCollector(ctx, config, cluster)
	if err != nil {
		return nil, fmt.Errorf("error creating log collector, error: %w", err)
	}

	if err := lollectorManager.Start(ctx); err != nil {
		slog.Error("failed to start log collector", "error", err)
	}
	return lollectorManager, nil
}

// Start begins the log collection process
func (c *logCollectorManager) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	// Start components
	err := c.lc.Start(c.ctx)
	if err != nil {
		return fmt.Errorf("failed to start log collector: %w", err)
	}

	// start goroutine of components
	go c.startPodMonitor()

	slog.Info("Log collector started successfully")
	return nil
}

// Stop stops the log collector
func (c *logCollectorManager) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}

	_ = c.lc.Stop()
	c.podMonitor.StopAllStreams()
	slog.Info("Log collector Manager stopped", slog.Any("client_id", c.cluster.ID))
	return nil
}

// GetStats returns current collector statistics
func (c *logCollectorManager) GetStats() *ltypes.CollectorStats {
	// Get stats from pod monitor
	podStats := c.podMonitor.GetStats()
	// Merge with our stats
	stats := c.stats
	stats.ActiveStreams = podStats.ActiveStreams
	stats.TotalLogsCollected = podStats.TotalLogsCollected
	stats.NamespaceStats = podStats.NamespaceStats
	stats.LastUpdate = time.Now()

	return &stats
}

// safeGoroutine
func (c *logCollectorManager) safeGoroutine(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Goroutine panic recovered",
				slog.String("goroutine", name),
				slog.Any("panic", r))
		}
	}()

	fn()
}

// startPodMonitor
func (c *logCollectorManager) startPodMonitor() {
	c.safeGoroutine("pod_monitor", func() {
		if err := c.podMonitor.Start(c.ctx); err != nil {
			slog.Error("Pod monitor failed", slog.Any("error", err))
		}
	})
}
