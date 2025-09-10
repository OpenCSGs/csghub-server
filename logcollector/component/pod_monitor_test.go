package component

import (
	"context"
	"opencsg.com/csghub-server/common/config"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	testcore "k8s.io/client-go/testing"

	"opencsg.com/csghub-server/common/types"
	ltypes "opencsg.com/csghub-server/logcollector/types"
	rtypes "opencsg.com/csghub-server/runner/types"
)

func TestNewPodMonitor(t *testing.T) {
	client := fake.NewSimpleClientset()
	logChan := make(chan types.LogEntry, 100)
	namespaces := []string{"default"}
	lastReportedTime := time.Now()

	config := &config.Config{}
	config.LogCollector.MaxConcurrentStreams = 10
	config.LogCollector.WatchNSInterval = 60

	pm := NewPodMonitor(client, namespaces, config, logChan, lastReportedTime)

	assert.NotNil(t, pm)
	assert.Equal(t, client, pm.client)
	assert.Equal(t, namespaces, pm.namespaces)
	assert.Equal(t, 10, pm.maxConcurrentStreams)
	assert.Equal(t, time.Duration(60)*time.Second, pm.watchNSInterval)
	assert.NotNil(t, pm.pods)
	assert.NotNil(t, pm.activeStreams)
	assert.NotNil(t, pm.podEvents)
	assert.Equal(t, logChan, pm.logChan)
	assert.Equal(t, lastReportedTime, pm.lastReportedTime)
}

func TestPodMonitor_Start(t *testing.T) {
	client := fake.NewSimpleClientset()
	logChan := make(chan types.LogEntry, 100)
	config := &config.Config{}
	pm := NewPodMonitor(client, []string{"default"}, config, logChan, time.Time{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := pm.Start(ctx)
	assert.NoError(t, err)

	// allow some time for goroutines to start
	time.Sleep(100 * time.Millisecond)
}

func TestPodMonitor_discoverExistingPods(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := fake.NewSimpleClientset(pod1)
	logChan := make(chan types.LogEntry, 100)
	config := &config.Config{}
	pm := NewPodMonitor(client, []string{"default"}, config, logChan, time.Time{})
	ctx := context.Background()

	pm.discoverExistingPods(ctx)

	select {
	case event := <-pm.podEvents:
		assert.Equal(t, watch.Added, event.Type)
		assert.Equal(t, "pod1", event.Pod.Name)
	case <-time.After(1 * time.Second):
		t.Fatal("expected pod event, but none received")
	}
}

func TestPodMonitor_watchNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	watcher := watch.NewFake()
	client.PrependWatchReactor("pods", testcore.DefaultWatchReactor(watcher, nil))

	config := &config.Config{}
	logChan := make(chan types.LogEntry, 100)
	pm := NewPodMonitor(client, []string{"default"}, config, logChan, time.Time{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pm.watchNamespace(ctx, "default")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	watcher.Add(pod)

	select {
	case event := <-pm.podEvents:
		assert.Equal(t, watch.Added, event.Type)
		assert.Equal(t, "pod1", event.Pod.Name)
	case <-time.After(1 * time.Second):
		t.Fatal("expected pod event, but none received")
	}
}

func TestPodMonitor_processPodEvents(t *testing.T) {
	client := fake.NewSimpleClientset()
	logChan := make(chan types.LogEntry, 100)
	config := &config.Config{}
	pm := NewPodMonitor(client, []string{"default"}, config, logChan, time.Time{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pm.processPodEvents(ctx)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default", UID: "uid1"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "container1"}},
		},
	}
	rtypes.LogTargetContainersMap = map[string]struct{}{"container1": {}}

	pm.podEvents <- PodEvent{Type: watch.Added, Pod: pod, Namespace: "default"}

	time.Sleep(100 * time.Millisecond)

	pm.podMutex.RLock()
	_, exists := pm.pods["default/pod1"]
	pm.podMutex.RUnlock()
	assert.True(t, exists)
}

func TestPodMonitor_handlePodEvent(t *testing.T) {
	client := fake.NewSimpleClientset()
	logChan := make(chan types.LogEntry, 100)
	config := &config.Config{}
	pm := NewPodMonitor(client, []string{"default"}, config, logChan, time.Time{})
	ctx := context.Background()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default", UID: "uid1"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "container1"}},
		},
	}
	rtypes.LogTargetContainersMap = map[string]struct{}{"container1": {}}

	// Test Added
	pm.handlePodEvent(ctx, PodEvent{Type: watch.Added, Pod: pod, Namespace: "default"})
	pm.podMutex.RLock()
	assert.Contains(t, pm.pods, "default/pod1")
	pm.podMutex.RUnlock()

	// Test Deleted
	pm.handlePodEvent(ctx, PodEvent{Type: watch.Deleted, Pod: pod, Namespace: "default"})
	pm.podMutex.RLock()
	assert.NotContains(t, pm.pods, "default/pod1")
	pm.podMutex.RUnlock()
}

func TestPodMonitor_GetStats(t *testing.T) {
	config := &config.Config{}
	pm := NewPodMonitor(nil, []string{"default"}, config, nil, time.Time{})
	pm.stats = ltypes.CollectorStats{
		TotalLogsCollected: 10,
		ActiveStreams:      1,
		NamespaceStats: map[string]ltypes.NamespaceStats{
			"default": {PodCount: 1, ActiveStreams: 1, LogsCollected: 10},
		},
	}

	stats := pm.GetStats()
	assert.Equal(t, int64(10), stats.TotalLogsCollected)
	assert.Equal(t, 1, stats.ActiveStreams)
	assert.Equal(t, 1, stats.NamespaceStats["default"].PodCount)
}

func TestPodMonitor_StopAllStreams(t *testing.T) {
	config := &config.Config{}
	pm := NewPodMonitor(nil, nil, config, nil, time.Time{})
	var wg sync.WaitGroup
	wg.Add(2)

	ctx1, cancel1 := context.WithCancel(context.Background())
	pm.activeStreams["stream1"] = &StreamInfo{cancel: func() {
		cancel1()
		wg.Done()
	},
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	pm.activeStreams["stream2"] = &StreamInfo{cancel: func() {
		cancel2()
		wg.Done()
	},
	}

	pm.StopAllStreams()

	wg.Wait()
	assert.Empty(t, pm.activeStreams)
	<-ctx1.Done()
	<-ctx2.Done()
}

func TestPodMonitor_shouldMonitorPod(t *testing.T) {
	pm := &PodMonitor{}

	testCases := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "regular pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			expected: true,
		},
		{
			name: "kube-system pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			expected: false,
		},
		{
			name: "succeeded pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status:     corev1.PodStatus{Phase: corev1.PodSucceeded},
			},
			expected: true,
		},
		{
			name: "failed pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Status:     corev1.PodStatus{Phase: corev1.PodFailed},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, pm.shouldMonitorPod(tc.pod))
		})
	}
}

func TestPodMonitor_extractServiceName(t *testing.T) {
	pm := &PodMonitor{}

	testCases := []struct {
		name     string
		pod      *corev1.Pod
		expected string
	}{
		{
			name: "app label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-app-pod-xyz",
					Labels: map[string]string{"app": "my-app"},
				},
			},
			expected: "my-app",
		},
		{
			name: "kubernetes.io/name label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-service-pod-xyz",
					Labels: map[string]string{"app.kubernetes.io/name": "my-service"},
				},
			},
			expected: "my-service",
		},
		{
			name: "fallback from pod name",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-fallback-app-deployment-abc-123",
				},
			},
			expected: "my-fallback-app-deployment",
		},
		{
			name: "fallback with short name",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "short-name",
				},
			},
			expected: "short-name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, pm.extractServiceName(tc.pod))
		})
	}
}

func init() {
	// Necessary for fake client to work with Pod objects
	scheme.Codecs.WithoutConversion()
}
