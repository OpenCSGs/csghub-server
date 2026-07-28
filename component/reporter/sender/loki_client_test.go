package sender

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	mock_loki "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/loki"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

// --- GetLastReportedTimestamp timeout tests ---

func Test_lokiClient_GetLastReportedTimestamp_SetTimeout(t *testing.T) {
	testCases := []struct {
		name                   string
		queryLastReportTimeout int
		expectedSetTimeout     time.Duration
	}{
		{
			name:                   "zero timeout",
			queryLastReportTimeout: 0,
			expectedSetTimeout:     0,
		},
		{
			name:                   "5s timeout - less than default 10s",
			queryLastReportTimeout: 5,
			expectedSetTimeout:     5 * time.Second,
		},
		{
			name:                   "15s timeout - greater than default 10s (bug scenario)",
			queryLastReportTimeout: 15,
			expectedSetTimeout:     15 * time.Second,
		},
		{
			name:                   "300s timeout - default value (bug scenario)",
			queryLastReportTimeout: 300,
			expectedSetTimeout:     300 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := mock_loki.NewMockClient(t)

			// SetTimeout should be called twice: once with the query timeout,
			// once with DefaultTimeout to restore.
			mockClient.EXPECT().SetTimeout(tc.expectedSetTimeout).Once()
			mockClient.EXPECT().SetTimeout(loki.DefaultTimeout).Once()

			mockClient.EXPECT().QueryRange(
				mock.Anything,
				mock.Anything,
			).RunAndReturn(func(ctx context.Context, _ loki.QueryRangeParams) (*loki.LokiQueryResponse, error) {
				return &loki.LokiQueryResponse{}, nil
			}).Maybe()

			c := &lokiClient{
				clientID:               types.ClientType("test-client"),
				queryLastReportTimeout: tc.queryLastReportTimeout,
				maxStoreTimeDay:        7,
				lokiClient:             mockClient,
			}

			_, _ = c.GetLastReportedTimestamp(context.Background())
		})
	}
}

func Test_lokiClient_GetLastReportedTimestamp_EmptyClientID(t *testing.T) {
	mockClient := mock_loki.NewMockClient(t)

	// SetTimeout should NOT be called when clientID is empty
	c := &lokiClient{
		clientID:               "",
		queryLastReportTimeout: 300,
		lokiClient:             mockClient,
	}

	_, err := c.GetLastReportedTimestamp(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no client ID")
}

func Test_lokiClient_GetLastReportedTimestamp_SuccessFromCache(t *testing.T) {
	mockClient := mock_loki.NewMockClient(t)

	// First query (cache) returns a valid timestamp
	ts := time.Now().Add(-time.Hour).UnixNano()
	cacheResponse := &loki.LokiQueryResponse{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     []loki.LokiStream `json:"result"`
		}{
			ResultType: "streams",
			Result: []loki.LokiStream{
				{
					Stream: map[string]string{"client_id": "test-client"},
					Values: [][]string{
						{fmt.Sprintf("%d", ts), "log message"},
					},
				},
			},
		},
	}

	mockClient.EXPECT().SetTimeout(300 * time.Second).Once()
	mockClient.EXPECT().SetTimeout(loki.DefaultTimeout).Once()
	mockClient.EXPECT().QueryRange(
		mock.Anything,
		mock.Anything,
	).RunAndReturn(func(ctx context.Context, _ loki.QueryRangeParams) (*loki.LokiQueryResponse, error) {
		return cacheResponse, nil
	}).Once()

	c := &lokiClient{
		clientID:               types.ClientType("test-client"),
		queryLastReportTimeout: 300,
		maxStoreTimeDay:        7,
		lokiClient:             mockClient,
	}

	result, err := c.GetLastReportedTimestamp(context.Background())
	require.NoError(t, err)
	assert.False(t, result.IsZero())
	// The result should be the timestamp + 1 nanosecond
	expected := time.Unix(0, ts).Add(time.Nanosecond)
	assert.Equal(t, expected, result)
}

func Test_lokiClient_GetLastReportedTimestamp_FallbackToTimeRangeQuery(t *testing.T) {
	mockClient := mock_loki.NewMockClient(t)

	// First query (cache) returns empty/no result
	emptyResponse := &loki.LokiQueryResponse{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     []loki.LokiStream `json:"result"`
		}{
			ResultType: "streams",
			Result:     []loki.LokiStream{},
		},
	}

	// Second query (with start time) returns a valid timestamp
	ts := time.Now().Add(-2 * time.Hour).UnixNano()
	fallbackResponse := &loki.LokiQueryResponse{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     []loki.LokiStream `json:"result"`
		}{
			ResultType: "streams",
			Result: []loki.LokiStream{
				{
					Stream: map[string]string{"client_id": "test-client"},
					Values: [][]string{
						{fmt.Sprintf("%d", ts), "log message"},
					},
				},
			},
		},
	}

	mockClient.EXPECT().SetTimeout(300 * time.Second).Once()
	mockClient.EXPECT().SetTimeout(loki.DefaultTimeout).Once()

	callCount := 0
	mockClient.EXPECT().QueryRange(
		mock.Anything,
		mock.Anything,
	).RunAndReturn(func(ctx context.Context, params loki.QueryRangeParams) (*loki.LokiQueryResponse, error) {
		callCount++
		if callCount == 1 {
			// First call: cache query, no start time
			assert.True(t, params.Start.IsZero(), "first query should have no start time")
			return emptyResponse, nil
		}
		// Second call: fallback with start time
		assert.False(t, params.Start.IsZero(), "second query should have a start time")
		return fallbackResponse, nil
	}).Times(2)

	c := &lokiClient{
		clientID:               types.ClientType("test-client"),
		queryLastReportTimeout: 300,
		maxStoreTimeDay:        7,
		lokiClient:             mockClient,
	}

	result, err := c.GetLastReportedTimestamp(context.Background())
	require.NoError(t, err)
	assert.False(t, result.IsZero())
	expected := time.Unix(0, ts).Add(time.Nanosecond)
	assert.Equal(t, expected, result)
	assert.Equal(t, 2, callCount, "QueryRange should be called twice")
}

// --- GetLastReportedTimestamp end-to-end timeout tests ---
//
// These tests verify the bug fix: GetLastReportedTimestamp extends the HTTP
// client timeout via SetTimeout for the duration of the query, then restores
// the default. Other operations (Health, Push, etc.) are unaffected and use
// the default 10s timeout.

func Test_lokiClient_GetLastReportedTimestamp_ExtendsTimeoutForQuery(t *testing.T) {
	// Server delays 12s on query_range, which exceeds the default 10s timeout.
	// GetLastReportedTimestamp should extend the timeout so the query succeeds.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/loki/api/v1/query_range" {
			time.Sleep(12 * time.Second)
			// Return empty result so both cache and fallback queries are exercised
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"streams","result":[]}}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.LogCollector.QueryLastReportTimeout = 15

	sender, err := NewLokiClient(server.URL, types.ClientType("test-client"), cfg)
	require.NoError(t, err)

	// GetLastReportedTimestamp should succeed because timeout is extended to 15s
	result, err := sender.GetLastReportedTimestamp(context.Background())
	require.NoError(t, err)
	assert.True(t, result.IsZero(), "no previous logs, should return zero time")
}

func TestNewLokiClient_HealthUsesDefaultTimeout(t *testing.T) {
	// Health check should use the default 10s timeout, not queryLastReportTimeout.
	// Server delays 12s, which exceeds the default 10s but is within 300s.
	// Health should fail because it uses the default timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(12 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.LogCollector.QueryLastReportTimeout = 300

	sender, err := NewLokiClient(server.URL, types.ClientType("test-client"), cfg)
	require.NoError(t, err)

	err = sender.Health(context.Background())
	require.Error(t, err, "health check should fail with default 10s timeout")
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestNewLokiClient_DefaultConfigUsesDefaultTimeout(t *testing.T) {
	// With default config, a quick server response should succeed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}

	sender, err := NewLokiClient(server.URL, types.ClientType("test-client"), cfg)
	require.NoError(t, err)

	err = sender.Health(context.Background())
	assert.NoError(t, err)
}

// TestNewLokiClient_ReturnsLogSender verifies that NewLokiClient returns a
// valid LogSender that can be used for all operations.
func TestNewLokiClient_ReturnsLogSender(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.LogCollector.QueryLastReportTimeout = 300
	cfg.LogCollector.AcceptLabelPrefix = "csghub_"
	cfg.LogCollector.LineSeparator = "\\n"
	cfg.LogCollector.MaxStoreTimeDay = 7
	cfg.Database.TimeZone = "UTC"

	sender, err := NewLokiClient(server.URL, types.ClientType("test-client"), cfg)
	require.NoError(t, err)
	assert.NotNil(t, sender)
}
