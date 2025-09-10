package component

import (
	"context"
	"fmt"
	"log/slog"
	"opencsg.com/csghub-server/common/config"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"opencsg.com/csghub-server/common/types"
	ltypes "opencsg.com/csghub-server/logcollector/types"
	rtypes "opencsg.com/csghub-server/runner/types"
)

// PodMonitor monitors pods in specified namespaces and manages log streams
type PodMonitor struct {
	client          kubernetes.Interface
	namespaces      []string
	watchNSInterval time.Duration

	// Pod tracking
	pods     map[string]*types.PodInfo // key: namespace/podname
	podMutex sync.RWMutex

	// Stream management
	activeStreams map[string]*StreamInfo // key: namespace/podname/container
	streamMutex   sync.RWMutex

	// Configuration
	maxConcurrentStreams int

	// Channels
	podEvents chan PodEvent
	logChan   chan types.LogEntry

	// Statistics
	stats     ltypes.CollectorStats
	statMutex sync.RWMutex

	// Recovery
	lastReportedTime time.Time
}

// PodEvent represents a pod lifecycle event
type PodEvent struct {
	Type      watch.EventType
	Pod       *corev1.Pod
	Namespace string
}

// NewPodMonitor creates a new pod monitor
func NewPodMonitor(client kubernetes.Interface, namespaces []string, config *config.Config, logChan chan types.LogEntry, lastReportedTime time.Time) *PodMonitor {
	return &PodMonitor{
		client:               client,
		namespaces:           namespaces,
		watchNSInterval:      time.Duration(config.LogCollector.WatchNSInterval) * time.Second,
		pods:                 make(map[string]*types.PodInfo),
		activeStreams:        make(map[string]*StreamInfo),
		maxConcurrentStreams: config.LogCollector.MaxConcurrentStreams,
		podEvents:            make(chan PodEvent, 1000),
		logChan:              logChan,
		stats: ltypes.CollectorStats{
			NamespaceStats: make(map[string]ltypes.NamespaceStats),
			LastUpdate:     time.Now(),
		},
		lastReportedTime: lastReportedTime,
	}
}

// Start begins monitoring pods in all configured namespaces
func (pm *PodMonitor) Start(ctx context.Context) error {
	slog.Info("Starting log collector",
		slog.Any("namespaces", pm.namespaces),
		slog.Int("max_concurrent_streams", pm.maxConcurrentStreams))

	// Start pod event processor
	go pm.processPodEvents(ctx)

	// Start watchers for each namespace
	for _, namespace := range pm.namespaces {
		go func(ns string) {
			pm.watchNamespace(ctx, ns)
		}(namespace)
	}

	// Initial pod discovery
	go func() {
		pm.discoverExistingPods(ctx)
	}()

	return nil
}

// discoverExistingPods discovers all existing pods in monitored namespaces
func (pm *PodMonitor) discoverExistingPods(ctx context.Context) {
	for _, namespace := range pm.namespaces {
		pods, err := pm.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			slog.Error("Failed to list existing pods",
				slog.String("namespace", namespace),
				slog.Any("error", err))
			continue
		}

		for _, pod := range pods.Items {
			if pm.shouldMonitorPod(&pod) {
				pm.podEvents <- PodEvent{
					Type:      watch.Added,
					Pod:       &pod,
					Namespace: namespace,
				}
			}
		}
	}
}

// watchNamespace watches for pod events in a specific namespace
func (pm *PodMonitor) watchNamespace(ctx context.Context, namespace string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			pm.watchNamespaceOnce(ctx, namespace)
			// Wait before retrying
			time.Sleep(pm.watchNSInterval)
		}
	}
}

// watchNamespaceOnce performs a single watch operation for a namespace
func (pm *PodMonitor) watchNamespaceOnce(ctx context.Context, namespace string) {
	watcher, err := pm.client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		slog.Error("Failed to create pod watcher",
			slog.String("namespace", namespace),
			slog.Any("error", err))
		return
	}
	defer watcher.Stop()

	slog.Info("Started watching pods", slog.String("namespace", namespace))

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				slog.Warn("Pod watcher channel closed", slog.String("namespace", namespace), slog.Any("event", event))
				return
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			if pm.shouldMonitorPod(pod) {
				pm.podEvents <- PodEvent{
					Type:      event.Type,
					Pod:       pod,
					Namespace: namespace,
				}
			}
		}
	}
}

// processPodEvents processes pod lifecycle events
func (pm *PodMonitor) processPodEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-pm.podEvents:
			pm.handlePodEvent(ctx, event)
		}
	}
}

// handlePodEvent handles a single pod event
func (pm *PodMonitor) handlePodEvent(ctx context.Context, event PodEvent) {
	podKey := fmt.Sprintf("%s/%s", event.Namespace, event.Pod.Name)

	switch event.Type {
	case watch.Added, watch.Modified:
		pm.handlePodAddedOrModified(ctx, event.Pod)
	case watch.Deleted:
		pm.handlePodDeleted(podKey)
	}
}

func (pm *PodMonitor) formatPodKey(pod *corev1.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

// handlePodAddedOrModified handles pod addition or modification
func (pm *PodMonitor) handlePodAddedOrModified(ctx context.Context, pod *corev1.Pod) {
	podKey := pm.formatPodKey(pod)

	// Update pod info
	podInfo := &types.PodInfo{
		PodName:     pod.Name,
		Namespace:   pod.Namespace,
		PodUID:      string(pod.UID),
		ServiceName: pm.extractServiceName(pod),
		Labels:      pod.Labels,
		Phase:       pod.Status.Phase,
	}

	pm.podMutex.Lock()
	pm.pods[podKey] = podInfo
	pm.podMutex.Unlock()

	if pod.Status.Phase == corev1.PodRunning ||
		pod.Status.Phase == corev1.PodSucceeded ||
		pod.Status.Phase == corev1.PodFailed {
		pm.startLogStreams(ctx, pod)
	}

	pm.updateStats()
}

// handlePodDeleted handles pod deletion
func (pm *PodMonitor) handlePodDeleted(podKey string) {
	pm.podMutex.Lock()
	delete(pm.pods, podKey)
	pm.podMutex.Unlock()

	pm.stopPodStreams(podKey)
	pm.updateStats()
}

// startLogStreams starts log streams for all containers in a pod
func (pm *PodMonitor) startLogStreams(_ context.Context, pod *corev1.Pod) {
	containerList := append(pod.Spec.InitContainers, pod.Spec.Containers...)
	for _, container := range containerList {
		containerName := container.Name
		if _, ok := rtypes.LogTargetContainersMap[containerName]; !ok {
			continue
		}

		streamKey := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, containerName)
		streamCtx, cancel := context.WithCancel(context.Background())
		streamInfo := &StreamInfo{
			key:              streamKey,
			cancel:           cancel,
			lastStreamedTime: nil,
		}
		pm.streamMutex.Lock()
		// Cancel previous stream if it exists and update last streamed time
		if oldStream, exists := pm.activeStreams[streamKey]; exists {
			// Cancel previous stream if it exists...
			oldStream.cancel()
			// ...and carry over its last streamed time to the new stream.
			if oldStream.lastStreamedTime != nil {
				newTime := oldStream.lastStreamedTime.Add(1 * time.Nanosecond)
				streamInfo.lastStreamedTime = &newTime
			}

		}
		pm.activeStreams[streamKey] = streamInfo
		pm.streamMutex.Unlock()

		// Check concurrent stream limit
		if len(pm.activeStreams) >= pm.maxConcurrentStreams {
			slog.Warn("Maximum concurrent streams reached",
				slog.Int("max_streams", pm.maxConcurrentStreams),
				slog.String("pod", pod.Name))
		}

		// Start log stream
		go pm.streamPodLogs(streamCtx, pod, containerName, streamInfo)

		slog.Debug("Started log stream",
			slog.String("pod", pod.Name),
			slog.String("container", containerName),
			slog.String("namespace", pod.Namespace))
	}
}

// ClusterAdapter adapts kubernetes.Interface to the cluster.Cluster interface
type ClusterAdapter struct {
	client kubernetes.Interface
}

func (c *ClusterAdapter) Client() kubernetes.Interface {
	return c.client
}

// streamPodLogs streams logs from a specific container
func (pm *PodMonitor) streamPodLogs(ctx context.Context, pod *corev1.Pod, containerName string, streamInfo *StreamInfo) {
	defer func() {
		slog.Debug("Stopped log stream",
			slog.String("pod", pod.Name),
			slog.String("container", containerName))
	}()

	// Determine the start time for fetching logs
	var sinceTime *metav1.Time
	podStartTime := pod.Status.StartTime

	if streamInfo.lastStreamedTime != nil {
		slog.Info("Resuming log stream from last streamed time", "pod", pod.Name,
			"container", containerName, "since", *streamInfo.lastStreamedTime, "status", pod.Status.Phase)
		sinceTime = &metav1.Time{Time: *streamInfo.lastStreamedTime}
	} else if !pm.lastReportedTime.IsZero() && podStartTime != nil && podStartTime.Time.Before(pm.lastReportedTime) {
		slog.Info("Resuming log stream from last reported time", "pod", pod.Name,
			"container", containerName, "since", pm.lastReportedTime, "status", pod.Status.Phase)
		sinceTime = &metav1.Time{Time: pm.lastReportedTime}
	} else {
		slog.Info("Starting new log stream from pod creation time", "pod", pod.Name,
			"container", containerName, "since", pod.CreationTimestamp.Time, "status", pod.Status.Phase)
		sinceTime = &pod.CreationTimestamp
	}

	// Get log stream using the existing function with correct client
	logChan, message, err := GetPodLogStream(ctx, pm.client, pod, containerName, sinceTime)
	if err != nil {
		slog.Warn("Failed to get pod log stream",
			slog.String("pod", pod.Name),
			slog.String("container", containerName),
			slog.Any("error", err))
		return
	}

	if message != "" {
		slog.Warn("Pod log stream warning",
			slog.String("pod", pod.Name),
			slog.String("message", message))
		return
	}

	basePodInfo := &types.PodInfo{
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		PodUID:        string(pod.UID),
		Labels:        pod.Labels,
		ServiceName:   pm.extractServiceName(pod),
		Phase:         pod.Status.Phase,
		ContainerName: containerName,
	}
	// Process log entries
	for {
		select {
		case <-ctx.Done():
			return
		case logData, ok := <-logChan:

			if !ok {
				return
			}

			// Parse and send log entry
			entry := types.LogEntry{
				Timestamp: time.Now(),
				Message:   string(logData),
				Category:  types.LogCategoryContainer,
				PodInfo:   basePodInfo,
			}

			select {
			case pm.logChan <- entry:
				pm.incrementLogCount(pod.Namespace)
				now := time.Now()
				streamInfo.lastStreamedTime = &now
			case <-ctx.Done():
				return
			}
		}
	}
}

// stopPodStreams stops all log streams for a pod
func (pm *PodMonitor) stopPodStreams(podKey string) {
	slog.Debug("Stopping log streams for pod", "pod", podKey)
	pm.streamMutex.Lock()
	defer pm.streamMutex.Unlock()

	for streamKey, streamInfo := range pm.activeStreams {
		if strings.HasPrefix(streamKey, podKey+"/") {
			streamInfo.cancel()
			delete(pm.activeStreams, streamKey)
		}
	}
}

// StopAllStreams stops all active log streams
func (pm *PodMonitor) StopAllStreams() {
	slog.Debug("Stopping all log streams")
	pm.streamMutex.Lock()
	defer pm.streamMutex.Unlock()

	for _, streamInfo := range pm.activeStreams {
		streamInfo.cancel()
	}
	pm.activeStreams = nil
}

// shouldMonitorPod determines if a pod should be monitored
func (pm *PodMonitor) shouldMonitorPod(pod *corev1.Pod) bool {
	// Skip system pods
	if strings.HasPrefix(pod.Namespace, "kube-") {
		return false
	}

	return true
}

// extractServiceName extracts service name from pod labels
func (pm *PodMonitor) extractServiceName(pod *corev1.Pod) string {
	if app, ok := pod.Labels["app"]; ok {
		return app
	}
	if service, ok := pod.Labels["app.kubernetes.io/name"]; ok {
		return service
	}
	// Fallback: derive from pod name
	parts := strings.Split(pod.Name, "-")
	if len(parts) > 2 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	return pod.Name
}

// updateStats updates collector statistics
func (pm *PodMonitor) updateStats() {
	pm.statMutex.Lock()
	defer pm.statMutex.Unlock()

	pm.streamMutex.RLock()
	pm.stats.ActiveStreams = len(pm.activeStreams)
	pm.streamMutex.RUnlock()

	// Update namespace stats
	namespacePodCount := make(map[string]int)
	namespaceStreamCount := make(map[string]int)

	pm.podMutex.RLock()
	for _, pod := range pm.pods {
		namespacePodCount[pod.Namespace]++
	}
	pm.podMutex.RUnlock()

	pm.streamMutex.RLock()
	for streamKey := range pm.activeStreams {
		parts := strings.Split(streamKey, "/")
		if len(parts) >= 2 {
			namespace := parts[0]
			namespaceStreamCount[namespace]++
		}
	}
	pm.streamMutex.RUnlock()

	for _, namespace := range pm.namespaces {
		if _, exists := pm.stats.NamespaceStats[namespace]; !exists {
			pm.stats.NamespaceStats[namespace] = ltypes.NamespaceStats{}
		}

		stats := pm.stats.NamespaceStats[namespace]
		stats.PodCount = namespacePodCount[namespace]
		stats.ActiveStreams = namespaceStreamCount[namespace]
		pm.stats.NamespaceStats[namespace] = stats
	}

	pm.stats.LastUpdate = time.Now()
}

// incrementLogCount increments the log count for a namespace
func (pm *PodMonitor) incrementLogCount(namespace string) {
	pm.statMutex.Lock()
	defer pm.statMutex.Unlock()

	pm.stats.TotalLogsCollected++

	if stats, exists := pm.stats.NamespaceStats[namespace]; exists {
		stats.LogsCollected++
		pm.stats.NamespaceStats[namespace] = stats
	}
}

// GetStats returns current collector statistics
func (pm *PodMonitor) GetStats() ltypes.CollectorStats {
	pm.statMutex.RLock()
	defer pm.statMutex.RUnlock()

	// Deep copy stats
	stats := pm.stats
	stats.NamespaceStats = make(map[string]ltypes.NamespaceStats)
	for k, v := range pm.stats.NamespaceStats {
		stats.NamespaceStats[k] = v
	}

	return stats
}

// StreamInfo stores information about an active log stream
type StreamInfo struct {
	key              string
	cancel           context.CancelFunc
	lastStreamedTime *time.Time
}
