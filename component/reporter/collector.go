package reporter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component/reporter/sender"

	"opencsg.com/csghub-server/common/types"
	ltypes "opencsg.com/csghub-server/logcollector/types"
)

// LogCollector defines the interface for a log collection service.
type LogCollector interface {
	Start(ctx context.Context) error
	Stop() error
	HealthCheck() error
	GetLogChan() chan types.LogEntry
	GetStats() ltypes.CollectorStats
	GetSender() sender.LogSender
	Report(entry types.LogEntry)
}

// logCollector implements the LogCollector interface.
type logCollector struct {
	config    *ltypes.LogCollectorConfig
	logSender sender.LogSender
	// IsSendReady indicates whether the log sender is ready to send logs.
	isSendReady bool

	// logChan is the channel for incoming log entries.
	logChan chan types.LogEntry
	// batchChan is the channel for batched log entries ready to be sent.
	batchChan chan []types.LogEntry

	// ctx and cancel are used for managing the collector's lifecycle.
	ctx    context.Context
	cancel context.CancelFunc

	// stats holds the collector's operational statistics.
	stats      ltypes.CollectorStats
	statsMutex sync.RWMutex

	// clientID is the unique identifier for this collector instance.
	clientID types.ClientType
}

// NewLogCollector creates a new log collector instance.
func NewLogCollector(config *config.Config, clientID types.ClientType) (LogCollector, error) {
	logCollectorCfg := Config(config)
	logChan := make(chan types.LogEntry, logCollectorCfg.BatchSize*10) // Large buffer

	logSender, err := sender.NewLokiClient(logCollectorCfg.LokiURL, clientID, config)
	if err != nil {
		slog.Error("failed to create loki client sender", slog.Any("error", err))
	}

	collector := &logCollector{
		config:    logCollectorCfg,
		logSender: logSender,
		logChan:   logChan,
		batchChan: make(chan []types.LogEntry, 100),
		stats: ltypes.CollectorStats{
			NamespaceStats: make(map[string]ltypes.NamespaceStats),
			LastUpdate:     time.Now(),
		},
		clientID:    clientID,
		isSendReady: false,
	}
	return collector, nil
}

// NewAndStartLogCollector creates a new log collector instance and starts it immediately.

func NewAndStartLogCollector(ctx context.Context, config *config.Config, clientID types.ClientType) (LogCollector, error) {
	logCollector, err := NewLogCollector(config, clientID)
	if err != nil {
		return nil, fmt.Errorf("error creating log collector, error: %w", err)
	}

	if err := logCollector.Start(ctx); err != nil {
		slog.Error("failed to start log collector", "error", err)
	}
	return logCollector, nil
}

// Start begins the log collection process.
func (c *logCollector) Start(ctx context.Context) error {
	// Health check Loki
	if err := c.HealthCheck(); err != nil {
		slog.Warn("Loki is unready")
	} else {
		c.isSendReady = true
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// send logs from producer logChan to consumer batchChan
	go c.startLogBatcherToConsumer()
	// Consumer batchChan to send to Sender client
	go c.startBatchSender()
	go c.startHealthChecker()

	slog.Info("Log collector started successfully", slog.Any("client_id", c.clientID))
	return nil
}

// Stop gracefully stops the log collector.
func (c *logCollector) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	slog.Info("Log collector stopped", slog.Any("client_id", c.clientID))
	return nil
}

// Report sends a single log entry to the collector asynchronously.
func (c *logCollector) Report(entry types.LogEntry) {
	// LogReporter is a plugin. when log reporter is not ready, log entry will be dropped.
	if !c.isSendReady {
		slog.Warn("Log entry dropped because log collector is not ready")
		return
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	// Add time at fronst of the log message
	// e.g. 2025-08-29T11:30:08.357042244+08:00 [36mINFO[0m[0021] This is container message \n
	entry.Message = fmt.Sprintf("%s %s", entry.Timestamp.Format(time.RFC3339Nano), entry.Message)
	entry.Category = types.LogCategoryPlatform
	go func(e types.LogEntry) {
		select {
		case c.logChan <- e:
		case <-time.After(c.config.DropMsgTimeout * time.Second):
			slog.Error("Log chan send timeout (one minute), discard log", slog.String("message", e.Message))
		}
	}(entry)
}

// GetLogChan returns the channel for submitting log entries.
func (c *logCollector) GetLogChan() chan types.LogEntry {
	return c.logChan
}

func (c *logCollector) GetSender() sender.LogSender {
	return c.logSender
}

// HealthCheck performs a health check on the log sending backend (e.g., Loki).
func (c *logCollector) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.logSender.Health(ctx); err != nil {
		return fmt.Errorf("loki health check failed: %w", err)
	}
	return nil
}

// runLogBatcherToConsumer collects logs from logChan, batches them, and sends them to batchChan.
func (c *logCollector) runLogBatcherToConsumer() {
	slog.Debug("Starting pod runLogBatcheToConsumer")
	ticker := time.NewTicker(c.config.BatchDelay)
	defer ticker.Stop()
	defer close(c.batchChan) // CLose batchChan befor exit

	var batch []types.LogEntry

	for {
		select {
		case <-c.ctx.Done():
			// Send remaining batch
			if len(batch) > 0 {
				c.sendBatchToConsumer(batch)
			}
			return

		case entry := <-c.logChan:
			batch = append(batch, entry)

			// Send batch if it reaches the configured size
			if len(batch) >= c.config.BatchSize {
				c.sendBatchToConsumer(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Send batch on timeout
			if len(batch) > 0 {
				c.sendBatchToConsumer(batch)
				batch = batch[:0]
			}
		}
	}
}

// sendBatchToConsumer sends a batch of logs to the batchChan.
func (c *logCollector) sendBatchToConsumer(batch []types.LogEntry) {
	// Make a copy of the batch
	batchCopy := make([]types.LogEntry, len(batch))
	copy(batchCopy, batch)

	select {
	case c.batchChan <- batchCopy:
	case <-c.ctx.Done():
	case <-time.After(c.config.DropMsgTimeout * time.Second):
		// Batch channel is full, drop the batch
		slog.Warn("Batch channel full, dropping batch", slog.Int("batch_size", len(batch)))
		c.incrementErrorCount()
	}
}

// runBatchSender consumes batches from batchChan and sends them to the log backend.
func (c *logCollector) runBatchSender() {
	slog.Debug("Starting runBatchSender")
	for {
		select {
		case <-c.ctx.Done():
			slog.Info("Context cancelled for batch sender, processing remaining batches...")
			for batch := range c.batchChan {
				c.sendBatchToLogServer(batch)
			}
			slog.Info("All remaining batches processed by batch sender.")
			return
		case batch := <-c.batchChan:
			c.sendBatchToLogServer(batch)
		}
	}
}

// sendBatchToLogServer sends a single batch to the log backend with retry logic.
func (c *logCollector) sendBatchToLogServer(batch []types.LogEntry) {
	c.safeGoroutine("sendBatchToLogServer", func() {
		for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
			if attempt > 0 {
				// Wait before retry
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(c.config.RetryInterval * time.Duration(attempt)):
				}
			}

			err := c.logSender.SendLogs(c.ctx, batch)
			if err == nil {
				c.incrementSentCount(len(batch))
				return
			}
			slog.Warn("Failed to send batch to Loki",
				slog.Int("attempt", attempt+1),
				slog.Int("max_retries", c.config.MaxRetries),
				slog.Any("error", err))
		}
		// All retries failed
		slog.Error("Failed to send batch to Loki after all retries",
			slog.Int("batch_size", len(batch)))
		c.incrementErrorCount()
	})
}

// runHealthChecker periodically checks the health of the log backend.
func (c *logCollector) runHealthChecker() {
	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.HealthCheck(); err != nil {
				slog.Debug("Loki health check failed", slog.Any("error", err))
			} else {
				c.isSendReady = true
			}
		}
	}
}

// GetStats returns the current statistics of the collector.
func (c *logCollector) GetStats() ltypes.CollectorStats {
	c.statsMutex.RLock()
	defer c.statsMutex.RUnlock()
	// Merge with our stats
	stats := c.stats
	stats.LastUpdate = time.Now()
	return stats
}

// incrementSentCount increments the total number of logs successfully sent.
func (c *logCollector) incrementSentCount(count int) {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()

	c.stats.TotalLogsSent += int64(count)
}

// incrementErrorCount increments the count of failed batch sends.
func (c *logCollector) incrementErrorCount() {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()

	c.stats.ErrorCount++
}

// safeGoroutine is a generic safe goroutine wrapper to catch panics and log them.
func (c *logCollector) safeGoroutine(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Goroutine panic recovered",
				slog.String("goroutine", name),
				slog.Any("panic", r))
		}
	}()

	fn()
}

// startLogBatcherToConsumer starts the goroutine that batches logs from the input channel.
func (c *logCollector) startLogBatcherToConsumer() {
	c.safeGoroutine("log_batcher_to_consumer", func() {
		c.runLogBatcherToConsumer()
	})
}

// startBatchSender starts the goroutine that sends batched logs.
func (c *logCollector) startBatchSender() {
	c.safeGoroutine("batch_sender", func() {
		c.runBatchSender()
	})
}

// startHealthChecker starts the goroutine that periodically checks the health of the backend.
func (c *logCollector) startHealthChecker() {
	c.safeGoroutine("health_checker", func() {
		c.runHealthChecker()
	})
}
