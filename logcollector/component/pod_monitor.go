package component

import (
	"context"
	"fmt"
	"io"
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

// StreamInfo stores information about an active log stream
type StreamInfo struct {
	key              string
	cancel           context.CancelFunc
	lastStreamedTime *time.Time
	Status           ltypes.StreamStatus
}

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
	logStreamCD      time.Duration
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
		logStreamCD:      time.Duration(config.LogCollector.StreamCD) * time.Second,
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
			Status:           ltypes.StreamStatusPending,
		}
		pm.streamMutex.Lock()
		// Cancel previous stream if it exists and update last streamed time
		if oldStream, exists := pm.activeStreams[streamKey]; exists {
			slog.Debug("Stream already running", slog.String("stream", streamKey),
				slog.Any("pods staues", pod.Status.Phase), slog.Any("stream status", oldStream.Status))
			if oldStream.Status == ltypes.StreamStatusRunning {
				pm.streamMutex.Unlock()
				continue
			}

			if oldStream.lastStreamedTime != nil {
				var addDuration time.Duration
				lastDuration := time.Since(*oldStream.lastStreamedTime)
				if lastDuration < pm.logStreamCD {
					addDuration = pm.logStreamCD - lastDuration
				}
				newTime := oldStream.lastStreamedTime.Add(addDuration)
				streamInfo.lastStreamedTime = &newTime
			}
			// Cancel previous stream if it exists...
			oldStream.cancel()
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
			slog.String("container", containerName),
			slog.String("status", string(streamInfo.Status)))
	}()

	// Determine the start time for fetching logs
	var sinceTime *metav1.Time
	if streamInfo.lastStreamedTime != nil {
		slog.Info("Resuming log stream from last streamed time", "pod", pod.Name,
			"container", containerName, "since", *streamInfo.lastStreamedTime, "status", pod.Status.Phase)
		sinceTime = &metav1.Time{Time: *streamInfo.lastStreamedTime}
	} else if !pm.lastReportedTime.IsZero() && pod.CreationTimestamp.Time.Before(pm.lastReportedTime) {
		slog.Info("Resuming log stream from last reported time", "pod", pod.Name,
			"container", containerName, "since", pm.lastReportedTime, "status", pod.Status.Phase)
		sinceTime = &metav1.Time{Time: pm.lastReportedTime}
	} else {
		slog.Info("Starting new log stream from pod creation time", "pod", pod.Name,
			"container", containerName, "since", pod.CreationTimestamp.Time, "status", pod.Status.Phase)
		sinceTime = &pod.CreationTimestamp
	}

	// Get log stream using the existing function with correct client
	logChan, message, err := pm.getPodLogStream(ctx, pm.client, pod, containerName, sinceTime)
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
	streamInfo.Status = ltypes.StreamStatusRunning
	for {
		select {
		case <-ctx.Done():
			// The context is canceled either when the pod is deleted or the stream is intentionally stopped.
			streamInfo.Status = ltypes.StreamStatusFailed
			return
		case logData, ok := <-logChan:
			if !ok {
				streamInfo.Status = ltypes.StreamStatusCompleted
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
				now := time.Now()
				streamInfo.lastStreamedTime = &now
				pm.incrementLogCount(pod.Namespace)
			case <-ctx.Done():
				streamInfo.Status = ltypes.StreamStatusFailed
				return
			}
		}
	}
}

// getPodLogStream gets a streaming channel of pod logs
func (pm *PodMonitor) getPodLogStream(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod, container string, sinceTime *metav1.Time) (chan []byte, string, error) {
	logOptions := &corev1.PodLogOptions{
		Container:  container,
		Follow:     true,
		Timestamps: true, // Timestamps are useful for debugging and potential future logic
	}

	if sinceTime != nil {
		logOptions.SinceTime = sinceTime
	}

	ch := make(chan []byte, 1000)
	buf := make([]byte, 32*1024)

	logs := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)

	// For streaming log requests, we must disable the client-side timeout.
	// The underlying http request will be kept alive until the context is cancelled.
	stream, err := logs.Timeout(0).Stream(ctx)
	if err != nil {
		return nil, "", err
	}

	go func() {
		defer close(ch)
		defer func() { _ = stream.Close() }()
		for {
			select {
			case <-ctx.Done():
				slog.Debug("logs request context done", slog.Any("error", ctx.Err()))
				return
			default:
				n, err := stream.Read(buf)
				if err != nil {
					if err == io.EOF {
						slog.Debug("read pod logs finished normally", slog.Any("error", err), slog.String("pod", pod.Name))
					} else {
						slog.Warn("read pod logs finished with error", slog.Any("error", err), slog.String("pod", pod.Name))
					}
					return
				}
				if n == 0 {
					time.Sleep(1000 * time.Millisecond)
					continue
				}

				if n > 0 {
					// Make a copy of the buffer to avoid data races
					data := make([]byte, n)
					copy(data, buf[:n])
					ch <- data
				}
			}
		}
	}()

	return ch, "", nil
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
