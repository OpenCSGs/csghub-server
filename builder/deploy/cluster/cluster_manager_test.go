package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
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
