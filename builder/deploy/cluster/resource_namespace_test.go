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
			config: &config.Config{Space: struct {
				BuilderEndpoint           string "env:\"STARHUB_SERVER_SPACE_BUILDER_ENDPOINT\" default:\"http://localhost:8082\""
				RunnerEndpoint            string "env:\"STARHUB_SERVER_SPACE_RUNNER_ENDPOINT\" default:\"http://localhost:8082\""
				RunnerServerPort          int    "env:\"STARHUB_SERVER_SPACE_RUNNER_SERVER_PORT\" default:\"8082\""
				InternalRootDomain        string "env:\"STARHUB_SERVER_INTERNAL_ROOT_DOMAIN\" default:\"internal.example.com\""
				PublicRootDomain          string "env:\"STARHUB_SERVER_PUBLIC_ROOT_DOMAIN\""
				DockerRegBase             string "env:\"STARHUB_SERVER_DOCKER_REG_BASE\" default:\"registry.cn-beijing.aliyuncs.com/opencsg_public/\""
				ImagePullSecret           string "env:\"STARHUB_SERVER_DOCKER_IMAGE_PULL_SECRET\" default:\"opencsg-pull-secret\""
				RProxyServerPort          int    "env:\"STARHUB_SERVER_SPACE_RPROXY_SERVER_PORT\" default:\"8083\""
				SessionSecretKey          string "env:\"STARHUB_SERVER_SPACE_SESSION_SECRET_KEY\" default:\"secret\""
				DeployTimeoutInMin        int    "env:\"STARHUB_SERVER_SPACE_DEPLOY_TIMEOUT_IN_MINUTES\" default:\"30\""
				BuildTimeoutInMin         int    "env:\"STARHUB_SERVER_SPACE_BUILD_TIMEOUT_IN_MINUTES\" default:\"30\""
				GPUModelLabel             string "env:\"STARHUB_SERVER_GPU_MODEL_LABEL\""
				ReadinessDelaySeconds     int    "env:\"STARHUB_SERVER_READINESS_DELAY_SECONDS\" default:\"120\""
				ReadinessPeriodSeconds    int    "env:\"STARHUB_SERVER_READINESS_PERIOD_SECONDS\" default:\"10\""
				ReadinessFailureThreshold int    "env:\"STARHUB_SERVER_READINESS_FAILURE_THRESHOLD\" default:\"3\""
				PYPIIndexURL              string "env:\"STARHUB_SERVER_SPACE_PYPI_INDEX_URL\" default:\"\""
				InformerSyncPeriodInMin   int    "env:\"STARHUB_SERVER_SPACE_INFORMER_SYNC_PERIOD_IN_MINUTES\" default:\"2\""
			}{
				GPUModelLabel: customGPUConfigJSON,
			}},
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
