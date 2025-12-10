package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestGetNameSpaceResourcesQuota(t *testing.T) {
	testNamespace := "test-ns"
	testQuotaName := "compute-quota"
	customGPUConfigJSON := `[{"type_label": "my-custom.com/gpu-model", "capacity_label": "my-custom.com/gpu-count"}]`
	cfg := &config.Config{}
	cfg.Space.GPUModelLabel = customGPUConfigJSON
	testCases := []struct {
		name           string
		namespace      string
		quotaName      string
		config         *config.Config
		initialObjects []runtime.Object
		wantResult     map[string]types.NodeResourceInfo
		wantErr        bool
		errorContains  string
	}{
		{
			name:      "Success - with standard Aliyun/NVIDIA label",
			namespace: testNamespace,
			quotaName: testQuotaName,
			config:    &config.Config{},
			initialObjects: []runtime.Object{
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testQuotaName,
						Namespace: testNamespace,
						Labels:    map[string]string{"aliyun.accelerator/nvidia_name": "NVIDIA-V100"},
					},
					Status: v1.ResourceQuotaStatus{
						Hard: v1.ResourceList{
							"requests.cpu":            resource.MustParse("20"),
							"requests.memory":         resource.MustParse("64Gi"),
							"requests.nvidia.com/gpu": resource.MustParse("4"), // Key must match what getXPULabel returns
						},
						Used: v1.ResourceList{
							"requests.cpu":            resource.MustParse("5"),
							"requests.memory":         resource.MustParse("16Gi"),
							"requests.nvidia.com/gpu": resource.MustParse("1"),
						},
					},
				},
			},
			wantResult: map[string]types.NodeResourceInfo{
				"": {
					TotalCPU:         20.0,
					AvailableCPU:     15.0,
					TotalMem:         64.0,
					AvailableMem:     48.0,
					TotalXPU:         4,
					AvailableXPU:     3,
					XPUCapacityLabel: "nvidia.com/gpu",
					GPUVendor:        "NVIDIA",
					XPUModel:         "V100",
				},
			},
			wantErr: false,
		},
		{
			name:      "Success - with Hygon DCU label",
			namespace: testNamespace,
			quotaName: testQuotaName,
			config:    &config.Config{},
			initialObjects: []runtime.Object{
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testQuotaName,
						Namespace: testNamespace,
						Labels:    map[string]string{"hygon.com/dcu.name": "DTU-Z100"},
					},
					Status: v1.ResourceQuotaStatus{
						Hard: v1.ResourceList{
							"requests.cpu":           resource.MustParse("32"),
							"requests.memory":        resource.MustParse("128Gi"),
							"requests.hygon.com/dcu": resource.MustParse("8"), // Key must match what getXPULabel returns
						},
						Used: v1.ResourceList{
							"requests.cpu":           resource.MustParse("10"),
							"requests.memory":        resource.MustParse("32Gi"),
							"requests.hygon.com/dcu": resource.MustParse("2"),
						},
					},
				},
			},
			wantResult: map[string]types.NodeResourceInfo{
				"": {
					TotalCPU:         32.0,
					AvailableCPU:     22.0,
					TotalMem:         128.0,
					AvailableMem:     96.0,
					TotalXPU:         8,
					AvailableXPU:     6,
					XPUCapacityLabel: "hygon.com/dcu",
					GPUVendor:        "DTU",
					XPUModel:         "Z100",
				},
			},
			wantErr: false,
		},
		{
			name:      "Success - with custom GPU label from config",
			namespace: testNamespace,
			quotaName: testQuotaName,
			config:    cfg,
			initialObjects: []runtime.Object{
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testQuotaName,
						Namespace: testNamespace,
						Labels:    map[string]string{"my-custom.com/gpu-model": "SuperCard-X"},
					},
					Status: v1.ResourceQuotaStatus{
						Hard: v1.ResourceList{
							"requests.cpu":                     resource.MustParse("10"),
							"requests.memory":                  resource.MustParse("32Gi"),
							"requests.my-custom.com/gpu-count": resource.MustParse("2"), // Key must match what getXPULabel returns
						},
						Used: v1.ResourceList{
							"requests.my-custom.com/gpu-count": resource.MustParse("1"),
						},
					},
				},
			},
			wantResult: map[string]types.NodeResourceInfo{
				"": {
					TotalCPU:         10.0,
					AvailableCPU:     10.0,
					TotalMem:         32.0,
					AvailableMem:     32.0,
					TotalXPU:         2,
					AvailableXPU:     1,
					XPUCapacityLabel: "my-custom.com/gpu-count",
					GPUVendor:        "SuperCard",
					XPUModel:         "X",
				},
			},
			wantErr: false,
		},
		{
			name:      "Success - no XPU/GPU labels present",
			namespace: testNamespace,
			quotaName: testQuotaName,
			config:    &config.Config{},
			initialObjects: []runtime.Object{
				&v1.ResourceQuota{
					ObjectMeta: metav1.ObjectMeta{Name: testQuotaName, Namespace: testNamespace},
					Status: v1.ResourceQuotaStatus{
						Hard: v1.ResourceList{
							"requests.cpu":    resource.MustParse("8"),
							"requests.memory": resource.MustParse("16Gi"),
						},
						Used: v1.ResourceList{
							"requests.cpu": resource.MustParse("2500m"),
						},
					},
				},
			},
			wantResult: map[string]types.NodeResourceInfo{
				"": {
					TotalCPU:     8.0,
					AvailableCPU: 5.5,
					TotalMem:     16.0,
					AvailableMem: 16.0,
					TotalXPU:     0,
					AvailableXPU: 0,
				},
			},
			wantErr: false,
		},
		{
			name:           "Error - quota not found",
			namespace:      testNamespace,
			quotaName:      "non-existent-quota",
			config:         &config.Config{},
			initialObjects: []runtime.Object{},
			wantResult:     nil,
			wantErr:        true,
			errorContains:  `resourcequotas "non-existent-quota" not found`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			fakeClient := fake.NewSimpleClientset(tc.initialObjects...)
			cluster := &Cluster{Client: fakeClient}

			// --- Action ---
			result, err := cluster.GetResourceInNamespace(tc.namespace, tc.quotaName, tc.config)

			// --- Assert ---
			if tc.wantErr {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantResult, result, fmt.Sprintf("The result map did not match the expected value for case '%s'", tc.name))
			}
		})
	}
}

func TestGetXPUMem(t *testing.T) {
	node1 := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Labels: map[string]string{
				"nvidia.com/gpu":                 "true",
				"aliyun.accelerator/nvidia_name": "T4",
				"aliyun.accelerator/nvidia_mem":  "16GiB",
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
			Labels: map[string]string{
				"nvidia.com/gpu":                 "true",
				"aliyun.accelerator/nvidia_name": "NVIDIA-A10",
				"aliyun.accelerator/nvidia_mem":  "23028MiB",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionTrue},
			},
		},
	}

	clientset := fake.NewSimpleClientset(node1, node2)
	cluster := &Cluster{
		Client: clientset,
	}

	config := &config.Config{}

	resources, err := cluster.GetResourcesInCluster(config)
	assert.NoError(t, err)
	assert.Len(t, resources, 2)

	assert.Equal(t, resources["node1"].XPUMem, "16 GiB")
	assert.Equal(t, resources["node2"].XPUMem, "22 GiB")
}
