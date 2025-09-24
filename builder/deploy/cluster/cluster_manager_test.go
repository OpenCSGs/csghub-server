package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func TestGetResourcesInCluster(t *testing.T) {
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"nvidia.com/gpu":                 "true",
				"aliyun.accelerator/nvidia_name": "T4",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionTrue},
			},
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
				"nvidia.com/gpu":  resource.MustParse("2"),
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("3"),
				v1.ResourceMemory: resource.MustParse("14Gi"),
				"nvidia.com/gpu":  resource.MustParse("2"),
			},
		},
	}

	node2 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionFalse},
			},
		},
	}

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("2Gi"),
							"nvidia.com/gpu":  resource.MustParse("1"),
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod2",
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
		},
		Status: v1.PodStatus{
			Phase: v1.PodSucceeded,
		},
	}

	clientset := fake.NewSimpleClientset(node1, node2, pod1, pod2)

	cluster := &Cluster{
		Client: clientset,
	}

	config := &config.Config{}

	resources, err := cluster.GetResourcesInCluster(config)
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Contains(t, resources, "node1")
	assert.NotContains(t, resources, "node2")

	node1Resources := resources["node1"]
	assert.Equal(t, 4.0, node1Resources.TotalCPU)
	assert.Equal(t, 2.0, node1Resources.AvailableCPU)
	assert.InDelta(t, 16.0, node1Resources.TotalMem, 0.1)
	assert.InDelta(t, 12.0, node1Resources.AvailableMem, 0.1)
	assert.Equal(t, int64(2), node1Resources.TotalXPU)
	assert.Equal(t, int64(1), node1Resources.AvailableXPU)
	assert.Equal(t, "T4", node1Resources.XPUModel)
	assert.Equal(t, "nvidia", node1Resources.GPUVendor)
}
