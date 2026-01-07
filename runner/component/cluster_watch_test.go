package component

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rtypes "opencsg.com/csghub-server/runner/types"
)

// Mock for rtypes.SubscribeKeyWithEventPush
func mockSubscribeCheck(val string) bool {
	return val == "true"
}

func TestClusterWatcher_WatchCallback(t *testing.T) {
	// mock rtypes.SubscribeKeyWithEventPush
	originalSubscribeKeyWithEventPush := rtypes.SubscribeKeyWithEventPush
	rtypes.SubscribeKeyWithEventPush = map[string]rtypes.Validator{
		"check_key": mockSubscribeCheck,
	}
	defer func() { rtypes.SubscribeKeyWithEventPush = originalSubscribeKeyWithEventPush }()

	// mock http server to receive webhook
	var receivedEvent types.WebHookSendEvent
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedEvent)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// test cases
	tests := []struct {
		name                string
		configMapData       map[string]string
		initialEndpoint     string
		expectError         bool
		expectedEndpoint    string
		expectEventPush     bool
		expectedClusterID   string
		expectedClusterMode types.ClusterMode
	}{
		{
			name: "should update endpoint and push event on valid config",
			configMapData: map[string]string{
				rtypes.KeyHubServerWebhookEndpoint: server.URL,
				"check_key":                        "true",
				rtypes.KeyRunnerExposedEndpont:     server.URL,
			},
			initialEndpoint:     server.URL,
			expectError:         false,
			expectedEndpoint:    server.URL,
			expectEventPush:     true,
			expectedClusterID:   "test-cluster",
			expectedClusterMode: types.ConnectModeInCluster,
		},
		{
			name: "should clear endpoint when configmap value is empty",
			configMapData: map[string]string{
				"STARHUB_SERVER_RUNNER_WATCH_CONFIGMAP_KEY": "",
			},
			initialEndpoint:  server.URL,
			expectError:      false,
			expectedEndpoint: "",
			expectEventPush:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset mock server state
			mu.Lock()
			receivedEvent = types.WebHookSendEvent{}
			mu.Unlock()

			// setup
			mockCluster := &cluster.Cluster{
				ID:          "test-cluster",
				CID:         "test-cluster",
				ConnectMode: types.ConnectModeInCluster,
				Region:      "test-region",
			}
			// Note: This line will still cause a compilation error if SetWebhookEndpoint is not defined in cluster.Cluster

			cfg := &config.Config{}
			cfg.Cluster.SpaceNamespace = "spaces"
			watcher := &clusterWatcher{
				cluster: mockCluster,
				env:     cfg,
			}
			watcher.SetWebhookEndpoint(tt.initialEndpoint)

			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cm"},
				Data:       tt.configMapData,
			}

			// execute
			err := watcher.WatchCallback(cm)

			// assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Note: This line will still cause a compilation error if GetWebhookEndpoint is not defined in cluster.Cluster
			assert.Equal(t, tt.expectedEndpoint, watcher.GetWebhookEndpoint())
		})
	}
}

func TestClusterWatcher_PushClusterChangeEvent(t *testing.T) {
	// mock http server to receive webhook
	var receivedEvent types.WebHookSendEvent
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		err = json.Unmarshal(body, &receivedEvent)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name                     string
		configmapData            map[string]string
		initialStorageClass      string
		initialNetworkInterface  string
		expectedStorageClass     string
		expectedNetworkInterface string
		expectedEventData        types.ClusterEvent
	}{
		{
			name: "should use ConfigMap values for StorageClass and NetworkInterface",
			configmapData: map[string]string{
				rtypes.KeyStorageClass:         "fast-ssd",
				rtypes.KeyNetworkInterface:     "eth1",
				rtypes.KeyRunnerClusterRegion:  "region-1",
				rtypes.KeyRunnerExposedEndpont: server.URL,
			},
			initialStorageClass:      "slow-ssd",
			initialNetworkInterface:  "eth0",
			expectedStorageClass:     "fast-ssd",
			expectedNetworkInterface: "eth1",
			expectedEventData: types.ClusterEvent{
				StorageClass:     "fast-ssd",
				NetworkInterface: "eth1",
			},
		},
		{
			name: "should use cluster values when ConfigMap is empty",
			configmapData: map[string]string{
				rtypes.KeyRunnerClusterRegion:  "region-1",
				rtypes.KeyRunnerExposedEndpont: server.URL,
			},
			initialStorageClass:      "fast-ssd",
			initialNetworkInterface:  "eth0",
			expectedStorageClass:     "fast-ssd",
			expectedNetworkInterface: "eth0",
			expectedEventData: types.ClusterEvent{
				StorageClass:     "fast-ssd",
				NetworkInterface: "eth0",
			},
		},
		{
			name: "should update cluster NetworkInterface when ConfigMap has value",
			configmapData: map[string]string{
				rtypes.KeyNetworkInterface:     "eth2",
				rtypes.KeyRunnerClusterRegion:  "region-1",
				rtypes.KeyRunnerExposedEndpont: server.URL,
			},
			initialStorageClass:      "fast-ssd",
			initialNetworkInterface:  "eth0",
			expectedStorageClass:     "fast-ssd",
			expectedNetworkInterface: "eth2",
			expectedEventData: types.ClusterEvent{
				StorageClass:     "fast-ssd",
				NetworkInterface: "eth2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset mock server state
			mu.Lock()
			receivedEvent = types.WebHookSendEvent{}
			mu.Unlock()

			// setup
			mockCluster := &cluster.Cluster{
				ID:               "test-cluster",
				CID:              "test-cluster",
				ConnectMode:      types.ConnectModeInCluster,
				Region:           "test-region",
				StorageClass:     tt.initialStorageClass,
				NetworkInterface: tt.initialNetworkInterface,
			}

			cfg := &config.Config{}
			cfg.Runner.WebHookEndpoint = server.URL
			cfg.APIToken = "test-token"

			watcher := &clusterWatcher{
				cluster: mockCluster,
				env:     cfg,
			}

			// execute
			err := watcher.pushClusterChangeEvent(tt.configmapData)

			// assert
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStorageClass, mockCluster.StorageClass)
			assert.Equal(t, tt.expectedNetworkInterface, mockCluster.NetworkInterface)

			// wait for async webhook call
			time.Sleep(100 * time.Millisecond)

			// verify event data
			mu.Lock()
			if receivedEvent.Data != nil {
				eventData, ok := receivedEvent.Data.(types.ClusterEvent)
				if ok {
					assert.Equal(t, tt.expectedEventData.StorageClass, eventData.StorageClass)
					assert.Equal(t, tt.expectedEventData.NetworkInterface, eventData.NetworkInterface)
				}
			}
			mu.Unlock()
		})
	}
}
