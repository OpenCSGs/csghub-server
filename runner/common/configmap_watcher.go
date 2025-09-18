package common

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"log/slog"
	"opencsg.com/csghub-server/common/config"
	"time"
)

// WebhookEndpointHandler defines the interface for handling updates to the webhook endpoint.
type WebhookEndpointHandler interface {
	WatchCallback(cm *corev1.ConfigMap) error
}

// ConfigmapWatcher defines the interface for watching the runner's ConfigMap.
type ConfigmapWatcher interface {
	Watch(ctx context.Context)
}

// configmapWatcher implements the ConfigmapWatcher interface using an informer.
type configmapWatcher struct {
	informer cache.SharedIndexInformer
	handler  WebhookEndpointHandler
}

// NewConfigmapWatcher creates a new watcher for the runner's ConfigMap using an informer.
// It takes the Kubernetes cluster connection and a handler for endpoint updates.
func NewConfigmapWatcher(
	client kubernetes.Interface,
	handler WebhookEndpointHandler,
	config *config.Config) (ConfigmapWatcher, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}
	if config.Runner.RunnerNamespace == "" {
		return nil, fmt.Errorf("namespace cannot be empty")
	}
	if config.Runner.WatchConfigmapName == "" {
		return nil, fmt.Errorf("configmapName cannot be empty")
	}

	// Create an informer factory, scoped to the specific namespace and ConfigMap name.
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		time.Duration(config.Runner.WatchConfigmapIntervalInSec)*time.Second,
		informers.WithNamespace(config.Runner.RunnerNamespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + config.Runner.WatchConfigmapName
		}),
	)

	configMapInformer := factory.Core().V1().ConfigMaps().Informer()

	cw := &configmapWatcher{
		informer: configMapInformer,
		handler:  handler,
	}

	// Add event handlers to the informer to process ConfigMap changes.
	if _, err := configMapInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cw.handleUpdate,
		UpdateFunc: func(old, new interface{}) { cw.handleUpdate(new) },
		DeleteFunc: cw.handleDelete,
	}); err != nil {
		return nil, err
	}

	return cw, nil
}

// Watch starts the informer to monitor changes to the runner's ConfigMap.
// It blocks until the context is cancelled.
func (cw *configmapWatcher) Watch(ctx context.Context) {
	slog.Info("Starting ConfigMap informer for runner configuration")

	// Run the informer in a background goroutine.
	go cw.informer.Run(ctx.Done())

	// Wait for the informer's cache to be synced before proceeding.
	if !cache.WaitForCacheSync(ctx.Done(), cw.informer.HasSynced) {
		slog.Error("Failed to sync informer cache")
		return
	}

	slog.Info("Informer cache synced successfully, watching for changes...")
	<-ctx.Done()
	slog.Info("Stopping ConfigMap informer.")
}

// handleChange is called when a ConfigMap is added or updated.
func (cw *configmapWatcher) handleUpdate(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		slog.Warn("Received unexpected object type in informer event", "type", obj)
		return
	}
	if nil == cm {
		slog.Warn("Received nil ConfigMap in informer event")
		return
	}
	if err := cw.handler.WatchCallback(cm); err != nil {
		slog.Error("Failed to handle watch callback update", "error", err)
	}
}

// handleDelete is called when the monitored ConfigMap is deleted.
func (cw *configmapWatcher) handleDelete(obj interface{}) {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		// Handle cases where the object is a DeletedFinalStateUnknown tombstone.
		slog.Warn("Received unexpected object type in informer event", "type", obj)
	}
	if nil == cm {
		slog.Warn("Received nil ConfigMap in informer event")
		return
	}
	cm.Data = nil
	if err := cw.handler.WatchCallback(cm); err != nil {
		slog.Error("Failed to handle watch callback update", "error", err)
	}
}
