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

		expected := fmt.Sprintf("build-1963574892 | 2025-09-01 time=\"2025-09-01T03:29:01.179Z\" level=info msg=\"Starting Workflow Executor\"%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | 2025-09-01 time=\"2025-09-01T03:29:01.181Z\" level=info msg=\"Using executor retry strategy\"%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | malformed log line without timestamp%s", c.lineSeparator) +
			fmt.Sprintf("build-1963574892 | invalid-timestamp-format another message%s", c.lineSeparator) +
			"platform | 2025-09-01 time=\"2025-09-01T03:29:29.209Z\" level=info msg=\"Main container completed\""

		actual := c.formatLokiLog(lokiLog, loc)
		assert.Equal(t, expected, actual)
	})
}
