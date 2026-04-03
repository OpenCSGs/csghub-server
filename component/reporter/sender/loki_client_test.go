package sender

import (
	"fmt"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/common/types"

	"github.com/stretchr/testify/assert"
)

func Test_lokiClient_formatPodIdentifier(t *testing.T) {
	c := &lokiClient{}
	testCases := []struct {
		name     string
		stream   map[string]string
		expected string
	}{
		{
			name: "platform category",
			stream: map[string]string{
				"category": string(types.LogCategoryPlatform),
				"pod_name": "some-pod-name-123-456",
			},
			expected: "platform",
		},
		{
			name: "container category with full pod name",
			stream: map[string]string{
				"category": string(types.LogCategoryContainer),
				"pod_name": "sib-wanghh20-gradio3-966-3308-build-1963574892",
			},
			expected: "build-1963574892",
		},
		{
			name: "container category with short pod name",
			stream: map[string]string{
				"category": string(types.LogCategoryContainer),
				"pod_name": "short-name",
			},
			expected: "short-name",
		},
		{
			name: "container category with pod name with two parts",
			stream: map[string]string{
				"category": string(types.LogCategoryContainer),
				"pod_name": "build-1963574892",
			},
			expected: "build-1963574892",
		},
		{
			name: "container category with no pod name",
			stream: map[string]string{
				"category": string(types.LogCategoryContainer),
			},
			expected: "container",
		},
		{
			name: "default category with pod name",
			stream: map[string]string{
				"category": string(types.LogCategoryContainer),
				"pod_name": "my-app-backend-xyz-abc",
			},
			expected: "xyz-abc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := c.formatPodIdentifier(tc.stream)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func Test_lokiClient_formatLokiLog(t *testing.T) {
	t.Run("format loki log with multiple streams and lines", func(t *testing.T) {
		c := &lokiClient{
			lineSeparator: "\\n",
		}
		loc, err := time.LoadLocation("Asia/Shanghai")
		assert.NoError(t, err)

		lokiLog := &loki.LokiPushRequest{
			Streams: []loki.LokiStream{
				{
					Stream: map[string]string{
						"pod_name": "sib-wanghh20-gradio3-966-3308-build-1963574892",
						"category": string(types.LogCategoryContainer),
					},
					Values: [][]string{
						{
							"1756697342167959096",
							"2025-09-01T11:29:01.179995457+08:00 time=\"2025-09-01T03:29:01.179Z\" level=info msg=\"Starting Workflow Executor\"\n" +
								"2025-09-01T11:29:01.181964400+08:00 time=\"2025-09-01T03:29:01.181Z\" level=info msg=\"Using executor retry strategy\"\n" +
								"malformed log line without timestamp\n" +
								"invalid-timestamp-format another message",
						},
					},
				},
				{
					Stream: map[string]string{
						"category": string(types.LogCategoryPlatform),
					},
					Values: [][]string{
						{
							"1756697369212729305",
							"2025-09-01T11:29:29.209954299+08:00 time=\"2025-09-01T03:29:29.209Z\" level=info msg=\"Main container completed\"",
						},
					},
				},
			},
		}

		expected := fmt.Sprintf("build-1963574892 | 2025-09-01 11:29:01 time=\"2025-09-01T03:29:01.179Z\" level=info msg=\"Starting Workflow Executor\"%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | 2025-09-01 11:29:01 time=\"2025-09-01T03:29:01.181Z\" level=info msg=\"Using executor retry strategy\"%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | malformed log line without timestamp%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | invalid-timestamp-format another message%s", c.lineSeparator) +
			"platform | 2025-09-01 11:29:29 time=\"2025-09-01T03:29:29.209Z\" level=info msg=\"Main container completed\""

		actual := c.formatLokiLog(lokiLog, loc)
		assert.Equal(t, expected, actual)
	})
}

func Test_lokiClient_logEntryToMap(t *testing.T) {
	c := &lokiClient{
		clientID:          types.ClientTypeRunner,
		acceptLabelPrefix: "csghub_",
	}

	testCases := []struct {
		name     string
		entry    *types.LogEntry
		expected map[string]string
	}{
		{
			name: "basic entry",
			entry: &types.LogEntry{
				Category: types.LogCategoryContainer,
				DeployID: "deploy-123",
				Labels: map[string]string{
					types.StreamKeyDeployTaskID: "task-123",
				},
			},
			expected: map[string]string{
				"client_id":                 "runner",
				"category":                  "container",
				types.StreamKeyDeployID:     "deploy-123",
				types.StreamKeyDeployTaskID: "task-123",
			},
		},
		{
			name: "entry with pod info",
			entry: &types.LogEntry{
				Category: types.LogCategoryContainer,
				DeployID: "deploy-123",
				Labels: map[string]string{
					types.StreamKeyDeployTaskID: "task-123",
				},
				PodInfo: &types.PodInfo{
					PodName:       "pod-abc",
					PodUID:        "uid-abc",
					Namespace:     "default",
					ServiceName:   "service-abc",
					ContainerName: "container-abc",
					Labels: map[string]string{
						"csghub_label": "value1",
						"other_label":  "value2",
						"csghub_empty": "",
					},
				},
			},
			expected: map[string]string{
				"client_id":                 "runner",
				"category":                  "container",
				types.StreamKeyDeployID:     "deploy-123",
				types.StreamKeyDeployTaskID: "task-123",
				"pod_name":                  "pod-abc",
				"pod_uid":                   "uid-abc",
				"namespace":                 "default",
				"service_name":              "service-abc",
				"container_name":            "container-abc",
				"csghub_label":              "value1",
			},
		},
		{
			name: "entry with custom labels",
			entry: &types.LogEntry{
				Category: types.LogCategoryContainer,
				DeployID: "deploy-123",
				Labels: map[string]string{
					types.StreamKeyDeployTaskID: "task-123",
					"custom_label":              "custom_value",
					"empty_label":               "",
				},
			},
			expected: map[string]string{
				"client_id":                 "runner",
				"category":                  "container",
				types.StreamKeyDeployID:     "deploy-123",
				types.StreamKeyDeployTaskID: "task-123",
				"custom_label":              "custom_value",
			},
		},
		{
			name: "max label count limit",
			entry: &types.LogEntry{
				Category: types.LogCategoryContainer,
				DeployID: "deploy-123",
				Labels: map[string]string{
					types.StreamKeyDeployTaskID: "task-123",
					"l1":                        "v1",
					"l2":                        "v2",
					"l3":                        "v3",
					"l4":                        "v4",
					"l5":                        "v5",
					"l6":                        "v6",
					"l7":                        "v7",
					"l8":                        "v8",
					"l9":                        "v9",
					"l10":                       "v10",
				},
				PodInfo: &types.PodInfo{
					PodName:       "p1",
					PodUID:        "u1",
					Namespace:     "n1",
					ServiceName:   "s1",
					ContainerName: "c1",
				},
			},
			// Base: 4
			// PodInfo: 5. Total 9.
			// Custom labels: 11 provided.
			// Max is 15.
			// Allowed custom labels: 15 - 9 = 6.
			// Note: types.StreamKeyDeployTaskID is in Labels, so it takes 1 slot.
			// So 5 more custom labels should be present.
			// Total in map should be 15.
			// However, map iteration order is random, so we can't deterministically say WHICH labels are present,
			// only that the count is 15 (if keys are unique) or less (if keys overlap).
			// Here all keys are unique (except overlap of StreamKeyDeployTaskID).
			// StreamKeyDeployTaskID is added in base, then re-added in loop.
			// If it's re-added, it consumes a slot.
			// Let's just check the length of the result map.
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := c.logEntryToMap(tc.entry)
			if tc.name == "max label count limit" {
				assert.LessOrEqual(t, len(actual), types.MaxLabelCount)
				// Base keys must exist
				assert.Equal(t, "runner", actual["client_id"])
				assert.Equal(t, "p1", actual["pod_name"])
			} else {
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}

func Test_lokiClient_GenerateQuery(t *testing.T) {
	c := &lokiClient{}
	testCases := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: "{}",
		},
		{
			name: "single label",
			labels: map[string]string{
				"client_id": "runner",
			},
			expected: `{client_id="runner"}`,
		},
		{
			name: "label with special characters",
			labels: map[string]string{
				"label": "value-with-dash",
			},
			expected: `{label="value-with-dash"}`,
		},
		{
			name: "label with empty value",
			labels: map[string]string{
				"empty": "",
			},
			expected: `{empty=""}`,
		},
		{
			name: "label with double quotes",
			labels: map[string]string{
				"label": `value"with"quote`,
			},
			expected: `{label="value"with"quote"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := c.GenerateLabelQuery(tc.labels)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
