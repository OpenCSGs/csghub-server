package component

import (
	"fmt"
	"log/slog"
	"time"

	v1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rcommon "opencsg.com/csghub-server/runner/common"
	rtypes "opencsg.com/csghub-server/runner/types"
	"opencsg.com/csghub-server/runner/utils"
)

type ClusterWatcher interface {
	WatchCallback(cm *v1.ConfigMap) error
}

// clusterWatcher implements rcommon.ConfigmapWatcherCallback
type clusterWatcher struct {
	cluster *cluster.Cluster
	env     *config.Config
}

func (w *clusterWatcher) WatchCallback(cm *v1.ConfigMap) error {
	webHookEndpoint := cm.Data[w.env.Runner.WatchConfigmapKey]
	// Delete WebHookEndpoint or delete entire configmap
	if len(webHookEndpoint) == 0 {
		w.SetWebhookEndpoint("")
		slog.Info("webhook endpoint is cleared", slog.String("cluster", w.cluster.CID))
		return nil
	}
	// check endpoint format
	if !utils.ValidUrl(webHookEndpoint) {
		return fmt.Errorf("invalid endpoint: %s", webHookEndpoint)
	}
	// update config.runner.WebHookEndpoint
	w.SetWebhookEndpoint(webHookEndpoint)
	slog.Info("webhook endpoint is updated", slog.String("cluster", w.cluster.CID), slog.String("endpoint", webHookEndpoint))

	// entire subscribeKey all check pass
	checkPass := true
	for configMapKey, function := range rtypes.SubscribeKeyWithEventPush {
		if !function(cm.Data[configMapKey]) {
			checkPass = false
			slog.Warn("The event push not be triggered. The key value check failed.", slog.Any("key", configMapKey))
			break
		}
	}
	if checkPass {
		_ = w.pushClusterChangeEvent(cm.Data)
	}
	return nil
}

func (w *clusterWatcher) pushClusterChangeEvent(configmapData map[string]string) error {
	data := types.ClusterEvent{
		ClusterID:        w.cluster.ID,
		ClusterConfig:    types.DefaultClusterCongfig,
		Region:           w.cluster.Region,
		Mode:             w.cluster.ConnectMode,
		StorageClass:     w.cluster.StorageClass,
		NetworkInterface: w.cluster.NetworkInterface,
		Status:           types.ClusterStatusRunning,
		Endpoint:         configmapData["STARHUB_SERVER_RUNNER_PUBLIC_DOMAIN"],
	}
	event := &types.WebHookSendEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: types.RunnerClusterUpdate,
			EventTime: time.Now().Unix(),
			ClusterID: w.cluster.ID,
			DataType:  types.WebHookDataTypeObject,
		},
		Data: data,
	}
	go func() {
		err := rcommon.Push(w.env.Runner.WebHookEndpoint, w.env.APIToken, event)
		if err != nil {
			slog.Error("failed to push RunnerClusterUpdate status event", slog.Any("error", err))
		}
	}()
	return nil
}

func (w *clusterWatcher) SetWebhookEndpoint(endpoint string) {
	w.env.Runner.WebHookEndpoint = endpoint
}

func (w *clusterWatcher) GetWebhookEndpoint() string {
	return w.env.Runner.WebHookEndpoint
}
