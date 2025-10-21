package component

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	webHookEndpoint := cm.Data[rtypes.KeyHubServerWebhookEndpoint]
	// Delete WebHookEndpoint or delete entire configmap
	if len(webHookEndpoint) == 0 {
		w.SetWebhookEndpoint("")
		slog.Warn("webhook endpoint is empty and skip update webhook endpoint", slog.String("cluster", w.cluster.CID))
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
		Mode:             w.cluster.ConnectMode,
		StorageClass:     w.cluster.StorageClass,
		NetworkInterface: w.cluster.NetworkInterface,
		Status:           types.ClusterStatusRunning,
		Region:           configmapData[rtypes.KeyRunnerClusterRegion],
		Endpoint:         configmapData[rtypes.KeyRunnerExposedEndpont],
		AppEndpoint:      w.getClusterAppEndpoint(configmapData),
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
	slog.Info("report_event_configmap_update", slog.Any("event", event))
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

func (w *clusterWatcher) getClusterAppEndpoint(configmapData map[string]string) string {
	inputVal := configmapData[rtypes.KeyApplicationEndpoint]
	if len(inputVal) < 1 {
		slog.Warn("no application endpoint provided in configmap", slog.Any(rtypes.KeyApplicationEndpoint, inputVal))
	}

	if inputVal != "auto" {
		return inputVal
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc, err := w.cluster.Client.CoreV1().Services("kourier-system").Get(ctx, "kourier", metav1.GetOptions{})
	if err != nil {
		slog.Warn("failed to get kourier-system/kourier service and use app endpoint input value", slog.Any("error", err))
		return inputVal
	}

	ingress := svc.Status.LoadBalancer.Ingress

	if len(ingress) < 1 {
		slog.Warn("kourier-system/kourier service does not have external IP and try to read clusterIP", slog.Any("ingress", ingress))
		clusterIP := svc.Spec.ClusterIP
		if len(clusterIP) > 0 {
			slog.Info("kourier-system/kourier service does not have external IP and use clusterIP", slog.Any("clusterIP", clusterIP))
			inputVal = fmt.Sprintf("http://%s", clusterIP)
		}
		return inputVal
	}

	inputVal = fmt.Sprintf("http://%s", ingress[0].IP)
	return inputVal
}
