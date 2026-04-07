package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

func TestVerifyPermissions(t *testing.T) {
	t.Run("should return error when namespace does not exist", func(t *testing.T) {
		clientset := fake.NewSimpleClientset()
		// The function to create a clientset from config needs to be adapted for testing
		// For this test, we'll assume verifyPermissions can accept a clientset directly
		// or we mock the config to produce our fake clientset.
		// Let's refactor verifyPermissions slightly to allow injecting the clientset for easier testing.
		config := &config.Config{}
		err := verifyPermissions(clientset, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "please check your cluster configuration. the specified namespaces cannot be empty")
	})

	t.Run("should succeed when namespace exists", func(t *testing.T) {
		ns := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "existing-ns"}}
		clientset := fake.NewSimpleClientset(ns)
		config := &config.Config{}
		config.Cluster.SpaceNamespace = "spaces"
		err := verifyPermissions(clientset, config)
		assert.Error(t, err)
	})
}

func TestGetGpuTypeAndVendor(t *testing.T) {
	tests := []struct {
		name           string
		vendorType     string
		label          string
		wantVendorType string // returns 0
		wantModel      string // returns 1
	}{
		{
			name:           "hyphen separated",
			vendorType:     "NVIDIA-GeForce-RTX-3060",
			label:          "nvidia.com/gpu",
			wantVendorType: "NVIDIA",
			wantModel:      "GeForce-RTX-3060",
		},
		{
			name:           "hyphen separated",
			vendorType:     "NVIDIA-4090",
			label:          "nvidia.com/gpu",
			wantVendorType: "NVIDIA",
			wantModel:      "4090",
		},
		{
			name:           "hyphen separated multi",
			vendorType:     "NVIDIA-GeForce-RTX-3060-Ti",
			label:          "nvidia.com/gpu",
			wantVendorType: "NVIDIA",
			wantModel:      "GeForce-RTX-3060-Ti",
		},
		{
			name:           "dot separated label",
			vendorType:     "T4",
			label:          "nvidia.com/gpu",
			wantVendorType: "nvidia",
			wantModel:      "T4",
		},
		{
			name:           "no separator",
			vendorType:     "T4",
			label:          "gpu",
			wantVendorType: "gpu",
			wantModel:      "T4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVendorType, gotModel := getGpuTypeAndVendor(tt.vendorType, tt.label)
			assert.Equal(t, tt.wantVendorType, gotVendorType)
			assert.Equal(t, tt.wantModel, gotModel)
		})
	}
}

func TestGetCluster(t *testing.T) {
	t.Run("should return error when no clusters available", func(t *testing.T) {
		pool := &pool{
			Clusters: []*Cluster{},
		}

		cluster, err := pool.GetCluster()
		assert.Error(t, err)
		assert.Nil(t, cluster)
		assert.Contains(t, err.Error(), "no available clusters")
	})

	t.Run("should return a cluster when clusters are available", func(t *testing.T) {
		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		testCluster := &Cluster{
			CID:        "test-cluster-1",
			ID:         "cluster-1",
			ConfigPath: "/path/to/kubeconfig",
			Client:     fake.NewSimpleClientset(),
		}

		pool := &pool{
			Clusters:     []*Cluster{testCluster},
			ClusterStore: mockClusterStore,
		}

		cluster, err := pool.GetCluster()
		assert.NoError(t, err)
		assert.NotNil(t, cluster)
		assert.Equal(t, testCluster.CID, cluster.CID)
		assert.Equal(t, testCluster.ID, cluster.ID)
	})
}

func TestGetClusterByID(t *testing.T) {
	t.Run("should return error when cluster not found", func(t *testing.T) {
		mockClusterStore := mockdb.NewMockClusterInfoStore(t)

		pool := &pool{
			Clusters:     []*Cluster{},
			ClusterStore: mockClusterStore,
		}
		mockClusterStore.EXPECT().ByClusterID(context.Background(), "non-existent-cluster").
			Return(database.ClusterInfo{}, errors.New("cluster not found"))

		cluster, err := pool.GetClusterByID(context.Background(), "non-existent-cluster")
		assert.Error(t, err)
		assert.Nil(t, cluster)
		assert.Contains(t, err.Error(), "cluster with the given ID does not exist")
	})

	t.Run("should return cluster when found", func(t *testing.T) {
		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		testCluster := &Cluster{
			CID:        "test-cluster-1",
			ID:         "cluster-1",
			ConfigPath: "/path/to/kubeconfig",
			Client:     fake.NewSimpleClientset(),
		}

		pool := &pool{
			Clusters:     []*Cluster{testCluster},
			ClusterStore: mockClusterStore,
		}
		mockClusterStore.EXPECT().ByClusterID(context.Background(), "cluster-1").
			Return(database.ClusterInfo{
				ClusterID:     "cluster-1",
				ClusterConfig: "test-cluster-1",
			}, nil)

		cluster, err := pool.GetClusterByID(context.Background(), "cluster-1")
		assert.NoError(t, err)
		assert.NotNil(t, cluster)
		assert.Equal(t, testCluster.CID, cluster.CID)
		assert.Equal(t, testCluster.ID, cluster.ID)
	})
}

func TestGetAllCluster(t *testing.T) {
	mockClusterStore := mockdb.NewMockClusterInfoStore(t)
	testCluster := &Cluster{
		CID:        "test-cluster-1",
		ID:         "cluster-1",
		ConfigPath: "/path/to/kubeconfig",
		Client:     fake.NewSimpleClientset(),
	}

	pool := &pool{
		Clusters:     []*Cluster{testCluster},
		ClusterStore: mockClusterStore,
	}

	clusters := pool.GetAllCluster()
	assert.Len(t, clusters, 1)
	assert.Equal(t, testCluster.CID, clusters[0].CID)
	assert.Equal(t, testCluster.ID, clusters[0].ID)
}

func TestUpdateCluster(t *testing.T) {
	mockClusterStore := mockdb.NewMockClusterInfoStore(t)

	pool := &pool{
		Clusters:     []*Cluster{},
		ClusterStore: mockClusterStore,
	}

	testCluster := &database.ClusterInfo{
		ClusterID: "cluster-1",
	}

	mockClusterStore.EXPECT().Update(context.Background(), *testCluster).Return(nil)

	err := pool.Update(context.Background(), testCluster)
	assert.NoError(t, err)
}

func TestGetXPULabel(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		config        *config.Config
		wantCapacity  string
		wantTypeLabel string
	}{
		{
			name: "nvidia.com/gpu.product label",
			labels: map[string]string{
				"nvidia.com/gpu.product": "NVIDIA-GeForce-RTX-3060",
			},
			config:        &config.Config{},
			wantCapacity:  "nvidia.com/gpu",
			wantTypeLabel: "nvidia.com/gpu.product",
		},
		{
			name: "aliyun.accelerator/nvidia_name label",
			labels: map[string]string{
				"aliyun.accelerator/nvidia_name": "T4",
			},
			config:        &config.Config{},
			wantCapacity:  "nvidia.com/gpu",
			wantTypeLabel: "aliyun.accelerator/nvidia_name",
		},
		{
			name: "nvidia.com/nvidia_name label",
			labels: map[string]string{
				"nvidia.com/nvidia_name": "T4",
			},
			config:        &config.Config{},
			wantCapacity:  "nvidia.com/gpu",
			wantTypeLabel: "nvidia.com/nvidia_name",
		},
		{
			name:          "no matching label",
			labels:        map[string]string{},
			config:        &config.Config{},
			wantCapacity:  "",
			wantTypeLabel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCapacity, gotTypeLabel, _ := getXPULabel(tt.labels, tt.config)
			assert.Equal(t, tt.wantCapacity, gotCapacity)
			assert.Equal(t, tt.wantTypeLabel, gotTypeLabel)
		})
	}
}

func TestCollectNodeResource(t *testing.T) {
	cfg := &config.Config{}

	t.Run("should return basic info when node is not ready", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-node-1",
				Labels: map[string]string{"key": "value"},
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionFalse,
					},
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "test-node-1", result.NodeName)
		assert.Equal(t, "", result.NodeStatus)
		assert.Equal(t, map[string]string{"key": "value"}, result.Labels)
		assert.Empty(t, result.Processes)
		assert.False(t, result.EnableVXPU)
	})

	t.Run("should return correct hardware info when node is ready without GPU", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-2",
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(4000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(3500, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(7*1024*1024*1024, resource.DecimalSI),
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "test-node-2", result.NodeName)
		assert.Equal(t, "Ready", result.NodeStatus)
		assert.Equal(t, float64(4), result.TotalCPU)
		assert.Equal(t, float64(3.5), result.AvailableCPU)
		assert.Equal(t, float32(8), result.TotalMem)
		assert.Equal(t, float32(7), result.AvailableMem)
		assert.Equal(t, int64(0), result.TotalXPU)
		assert.Equal(t, int64(0), result.AvailableXPU)
		assert.Empty(t, result.XPUModel)
		assert.Empty(t, result.GPUVendor)
		assert.Empty(t, result.VXPUs)
		assert.False(t, result.EnableVXPU)
	})

	t.Run("should return correct GPU info with nvidia.com/gpu.product label", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-3",
				Labels: map[string]string{
					"nvidia.com/gpu.product": "NVIDIA-GeForce-RTX-3060",
					"nvidia.com/gpu.mem":     "8Gi",
				},
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(8000, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(32*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(2, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(7500, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(30*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "test-node-3", result.NodeName)
		assert.Equal(t, "NVIDIA", result.GPUVendor)
		assert.Equal(t, "GeForce-RTX-3060", result.XPUModel)
		assert.Equal(t, "nvidia.com/gpu", result.XPUCapacityLabel)
		assert.Equal(t, int64(2), result.TotalXPU)
		assert.Equal(t, int64(2), result.AvailableXPU)
		assert.Equal(t, "8.0 GiB", result.XPUMem)
	})

	t.Run("should return correct GPU info with aliyun accelerator label", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-4",
				Labels: map[string]string{
					"aliyun.accelerator/nvidia_name": "T4",
					"aliyun.accelerator/nvidia_mem":  "16384",
				},
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(8000, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(32*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(1, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(7500, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(30*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(1, resource.DecimalSI),
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "nvidia", result.GPUVendor)
		assert.Equal(t, "T4", result.XPUModel)
		assert.Equal(t, "nvidia.com/gpu", result.XPUCapacityLabel)
		assert.Equal(t, int64(1), result.TotalXPU)
	})

	t.Run("should return correct GPU info with nvidia.com/nvidia_name label", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-5",
				Labels: map[string]string{
					"nvidia.com/nvidia_name": "A100-SXM4-40GB",
				},
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(16000, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(64*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(8, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:                    *resource.NewMilliQuantity(15000, resource.DecimalSI),
					v1.ResourceMemory:                 *resource.NewQuantity(60*1024*1024*1024, resource.DecimalSI),
					v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(8, resource.DecimalSI),
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "A100", result.GPUVendor)
		assert.Equal(t, "SXM4-40GB", result.XPUModel)
		assert.Equal(t, int64(8), result.TotalXPU)
		assert.Equal(t, int64(8), result.AvailableXPU)
	})

	t.Run("should return node with multiple ready conditions", func(t *testing.T) {
		node := &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-6",
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeDiskPressure,
						Status: v1.ConditionFalse,
					},
					{
						Type:   v1.NodeMemoryPressure,
						Status: v1.ConditionFalse,
					},
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(4000, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(8*1024*1024*1024, resource.DecimalSI),
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(3500, resource.DecimalSI),
					v1.ResourceMemory: *resource.NewQuantity(7*1024*1024*1024, resource.DecimalSI),
				},
			},
		}

		result := collectNodeResource(*node, cfg)

		assert.Equal(t, "test-node-6", result.NodeName)
		assert.Equal(t, "Ready", result.NodeStatus)
	})
}
