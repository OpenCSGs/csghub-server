package common

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// mockWebhookEndpointHandler is a mock implementation of the WebhookEndpointHandler interface for testing.
type mockWebhookEndpointHandler struct {
	mu         sync.Mutex
	callback   func(cm *corev1.ConfigMap)
	err        error
	calledWith []*corev1.ConfigMap
}

func (m *mockWebhookEndpointHandler) WatchCallback(cm *corev1.ConfigMap) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calledWith = append(m.calledWith, cm.DeepCopy())
	if m.callback != nil {
		m.callback(cm)
	}
	return m.err
}

func (m *mockWebhookEndpointHandler) getCalledWith() []*corev1.ConfigMap {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calledWith
}

func TestNewConfigmapWatcher(t *testing.T) {
	client := fake.NewSimpleClientset()
	handler := &mockWebhookEndpointHandler{}
	namespace := "test-ns"
	configmapName := "test-cm"

	t.Run("should return error when client is nil", func(t *testing.T) {
		_, err := NewConfigmapWatcher(nil, handler, namespace, configmapName)
		assert.Error(t, err)
		assert.Equal(t, "client cannot be nil", err.Error())
	})

	t.Run("should return error when handler is nil", func(t *testing.T) {
		_, err := NewConfigmapWatcher(client, nil, namespace, configmapName)
		assert.Error(t, err)
		assert.Equal(t, "handler cannot be nil", err.Error())
	})

	t.Run("should return error when namespace is empty", func(t *testing.T) {
		_, err := NewConfigmapWatcher(client, handler, "", configmapName)
		assert.Error(t, err)
		assert.Equal(t, "namespace cannot be empty", err.Error())
	})

	t.Run("should return error when configmapName is empty", func(t *testing.T) {
		_, err := NewConfigmapWatcher(client, handler, namespace, "")
		assert.Error(t, err)
		assert.Equal(t, "configmapName cannot be empty", err.Error())
	})

	t.Run("should create a new watcher successfully", func(t *testing.T) {
		watcher, err := NewConfigmapWatcher(client, handler, namespace, configmapName)
		assert.NoError(t, err)
		assert.NotNil(t, watcher)
	})
}

func TestConfigmapWatcher_Watch(t *testing.T) {
	namespace := "default"
	configmapName := "runner-cm"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientset := fake.NewSimpleClientset()
	var wg sync.WaitGroup
	handler := &mockWebhookEndpointHandler{
		callback: func(cm *corev1.ConfigMap) {
			wg.Done()
		},
	}

	watcher, err := NewConfigmapWatcher(clientset, handler, namespace, configmapName)
	require.NoError(t, err)

	go watcher.Watch(ctx)

	// Allow some time for the informer to start and sync
	time.Sleep(200 * time.Millisecond)

	t.Run("should trigger callback on add", func(t *testing.T) {
		wg.Add(1)
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configmapName,
				Namespace: namespace,
			},
			Data: map[string]string{"endpoint": "http://example.com"},
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		require.NoError(t, err)

		waitTimeout(t, &wg, 5*time.Second)

		calledCMs := handler.getCalledWith()
		require.NotEmpty(t, calledCMs)
		lastCall := calledCMs[len(calledCMs)-1]
		assert.Equal(t, configmapName, lastCall.Name)
		assert.Equal(t, "http://example.com", lastCall.Data["endpoint"])
	})

	t.Run("should trigger callback on update", func(t *testing.T) {
		wg.Add(1)
		updatedCm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configmapName, metav1.GetOptions{})
		require.NoError(t, err)

		updatedCm.Data["endpoint"] = "http://new-example.com"
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), updatedCm, metav1.UpdateOptions{})
		require.NoError(t, err)

		waitTimeout(t, &wg, 5*time.Second)

		calledCMs := handler.getCalledWith()
		require.NotEmpty(t, calledCMs)
		lastCall := calledCMs[len(calledCMs)-1]
		assert.Equal(t, "http://new-example.com", lastCall.Data["endpoint"])
	})

	t.Run("should trigger callback on delete", func(t *testing.T) {
		wg.Add(1)
		err := clientset.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), configmapName, metav1.DeleteOptions{})
		require.NoError(t, err)

		waitTimeout(t, &wg, 5*time.Second)

		calledCMs := handler.getCalledWith()
		require.NotEmpty(t, calledCMs)
		lastCall := calledCMs[len(calledCMs)-1]
		// The handler sets Data to nil on delete
		assert.Nil(t, lastCall.Data)
	})
}

// waitTimeout waits for the waitgroup for the specified duration.
func waitTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration) {
	t.Helper()
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		// completed
	case <-time.After(timeout):
		t.Fatal("timed out waiting for callback")
	}
}
