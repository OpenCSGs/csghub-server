package cluster

import (
	"fmt"
	"log/slog"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func (cluster *Cluster) GetResourceAvailability(config *config.Config) (types.ResourceStatus, map[string]types.NodeResourceInfo, error) {

	// --- Step 1: Attempt to query cluster-wide resources (preferred method) ---
	nodeResourcesMap, err := cluster.GetResourcesInCluster(config)
	if err == nil {
		// Success with cluster-wide permissions.
		return types.StatusClusterWide, nodeResourcesMap, nil
	}

	if !k8s_errors.IsForbidden(err) {
		return "", nil, fmt.Errorf("failed to get node resources with a non-permission error: %w", err)
	}
	namespace := config.Cluster.SpaceNamespace
	quotaName := config.Cluster.QuotaName

	slog.Info("Cluster-wide access denied without ClusterRole. Falling back to namespace resource quota",
		slog.Any("error", err),
		slog.String("namespace", namespace),
		slog.String("quota", quotaName))

	quotaResult, err := cluster.GetResourceInNamespace(namespace, quotaName, config)
	if err == nil {
		slog.Info("Successfully retrieved resource availability", slog.String("quota", quotaName))
		return types.StatusNamespaceQuota, quotaResult, nil
	}
	slog.Warn("ResourceQuota not found, skipping",
		slog.String("namespace", namespace),
		slog.String("quota", quotaName),
		slog.Any("error", err))
	return types.StatusUncertain, nil, nil
}
