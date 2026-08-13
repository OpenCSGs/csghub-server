package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestQueryRange_URLAndParams(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		resp := `{"status":"success","data":{"resultType":"matrix","result":[]}}`
		_, _ = w.Write([]byte(resp))
	}))
	defer ts.Close()

	cfg := &config.Config{}
	cfg.Prometheus.ApiAddress = ts.URL + "/api/v1/query"
	client := NewPrometheusClient(cfg)

	start := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)

	_, err := client.QueryRange(context.Background(), "up", start, end, time.Minute)
	require.NoError(t, err)

	// Verify the URL contains query_range (not query) and the required params.
	require.Contains(t, capturedURL, "/api/v1/query_range")
	require.Contains(t, capturedURL, "query=up")
	require.Contains(t, capturedURL, "start=")
	require.Contains(t, capturedURL, "end=")
	require.Contains(t, capturedURL, "step=60")
}

func TestQueryRange_ParsesResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "matrix",
				"result": []map[string]any{
					{
						"metric": map[string]string{"model": "qwen-72b"},
						"values": [][]any{
							{float64(1753972800), "42"},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	cfg := &config.Config{}
	cfg.Prometheus.ApiAddress = ts.URL + "/api/v1/query"
	client := NewPrometheusClient(cfg)

	resp, err := client.QueryRange(context.Background(), "up", time.Now(), time.Now().Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.Equal(t, "success", resp.Status)
	require.Len(t, resp.Data.Result, 1)
	require.Equal(t, "qwen-72b", resp.Data.Result[0].Metric["model"])
	require.Len(t, resp.Data.Result[0].Values, 1)
}

func TestQueryRange_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad query"}`))
	}))
	defer ts.Close()

	cfg := &config.Config{}
	cfg.Prometheus.ApiAddress = ts.URL + "/api/v1/query"
	client := NewPrometheusClient(cfg)

	_, err := client.QueryRange(context.Background(), "up", time.Now(), time.Now().Add(time.Minute), time.Minute)
	require.Error(t, err)
}

func TestQueryRangeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://prom:9090/api/v1/query", "http://prom:9090/api/v1/query_range"},
		{"http://prom:9090/api/v1/query_range", "http://prom:9090/api/v1/query_range"},
		{"http://prom:9090/api/v1/query/", "http://prom:9090/api/v1/query_range"},
		{"http://prom:9090", "http://prom:9090/api/v1/query_range"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, queryRangeURL(tt.input))
	}
}
