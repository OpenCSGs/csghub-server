//go:build ee || saas

package activity

import (
	"log/slog"

	"opencsg.com/csghub-server/aigateway/component/metrics"
	"opencsg.com/csghub-server/builder/deploy"
	deploycommon "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

// NewDeployerForReconcile builds a deploy.Deployer used by the temporal
// cron-based reconciler activity. Returns nil (and logs an error) if
// initialisation fails, so the worker can still start: the deploy activity
// becomes a no-op and logs the reason.
func NewDeployerForReconcile(cfg *config.Config) deploy.Deployer {
	dc := deploycommon.BuildDeployConfig(cfg)
	d, err := deploy.NewDeployerForReconcile(cfg, dc)
	if err != nil {
		slog.Error("failed to create deployer for reconcile", "error", err)
		return nil
	}
	return d
}

// NewMetricCollectorForReconcile builds a MetricCollector used by the
// temporal cron collector activity. Returns nil on failure (worker still
// starts); the collector activity will be a no-op when the pointer is nil.
func NewMetricCollectorForReconcile(cfg *config.Config) *metrics.MetricCollector {
	promClient := prometheus.NewPrometheusClient(cfg)
	metricStore := database.NewAIGatewayMetricMinuteStore()
	checkpointStore := database.NewAIGatewayMetricsCheckpointStore()
	return metrics.NewMetricCollector(cfg, promClient, metricStore, checkpointStore)
}
