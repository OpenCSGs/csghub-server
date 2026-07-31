package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{}

func TestNewClient(t *testing.T) {
	_, err := NewClient("http://localhost:3100")
	require.NoError(t, err)
}

func TestClient_Push(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/push", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		var req LokiPushRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Len(t, req.Streams, 1)
		assert.Equal(t, "test-app", req.Streams[0].Stream["app"])
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	req := &LokiPushRequest{
		Streams: []LokiStream{
			{
				Stream: map[string]string{"app": "test-app"},
				Values: [][]string{{fmt.Sprintf("%d", time.Now().UnixNano()), "log message"}},
			},
		},
	}

	err = client.Push(context.Background(), req)
	require.NoError(t, err)
}

func TestClient_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/query", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, `{app="test-app"}`, r.URL.Query().Get("query"))
		resp := &LokiQueryResponse{
			Status: "success",
			Data: struct {
				ResultType string       `json:"resultType"`
				Result     []LokiStream `json:"result"`
			}{
				ResultType: "streams",
				Result: []LokiStream{
					{
						Stream: map[string]string{"app": "test-app"},
						Values: [][]string{{fmt.Sprintf("%d", time.Now().UnixNano()), "log message"}},
					},
				},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	resp, err := client.Query(context.Background(), `{app="test-app"}`, 1, time.Now(), "forward")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Result, 1)
}

func TestClient_QueryRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/query_range", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, `{app="test-app"}`, r.URL.Query().Get("query"))
		assert.Equal(t, "100", r.URL.Query().Get("limit"))
		assert.Equal(t, "forward", r.URL.Query().Get("direction"))

		resp := &LokiQueryResponse{
			Status: "success",
			Data: struct {
				ResultType string       `json:"resultType"`
				Result     []LokiStream `json:"result"`
			}{
				ResultType: "streams",
				Result: []LokiStream{
					{
						Stream: map[string]string{"app": "test-app"},
						Values: [][]string{{fmt.Sprintf("%d", time.Now().UnixNano()), "log message"}},
					},
				},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	params := QueryRangeParams{
		Query:     `{app="test-app"}`,
		Limit:     100,
		Start:     time.Now().Add(-time.Hour),
		End:       time.Now(),
		Direction: "forward",
	}

	resp, err := client.QueryRange(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Result, 1)
}

func TestClient_QueryLast(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/query_range", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, `{app="test-app"}`, r.URL.Query().Get("query"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, "168h0m0s", r.URL.Query().Get("since"))
		assert.Equal(t, "backward", r.URL.Query().Get("direction"))

		resp := &LokiQueryResponse{
			Status: "success",
			Data: struct {
				ResultType string       `json:"resultType"`
				Result     []LokiStream `json:"result"`
			}{
				ResultType: "streams",
				Result: []LokiStream{
					{
						Stream: map[string]string{"app": "test-app"},
						Values: [][]string{{fmt.Sprintf("%d", time.Now().UnixNano()), "log message"}},
					},
				},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	params := QueryLastParams{
		Query:     `{app="test-app"}`,
		Limit:     50,
		Since:     7 * 24 * time.Hour,
		Direction: "backward",
	}

	resp, err := client.QueryLast(context.Background(), params)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "success", resp.Status)
	assert.Len(t, resp.Data.Result, 1)
}

func TestClient_Ready(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ready", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	err = client.Ready(context.Background())
	require.NoError(t, err)
}

func TestClient_Tail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/loki/api/v1/tail") {
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()

			// Simulate Loki sending a log message
			logEntry := LokiPushRequest{
				Streams: []LokiStream{
					{
						Stream: map[string]string{"app": "test-app"},
						Values: [][]string{{fmt.Sprintf("%d", time.Now().UnixNano()), "streamed log message"}},
					},
				},
			}
			msg, err := json.Marshal(logEntry)
			require.NoError(t, err)
			err = conn.WriteMessage(websocket.TextMessage, msg)
			require.NoError(t, err)
			// Keep the connection open for a short time to allow the client to read
			time.Sleep(100 * time.Millisecond)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logChan, err := client.Tail(ctx, `{app="test-app"}`, time.Now(), 100)
	require.NoError(t, err)
	require.NotNil(t, logChan)

	select {
	case log, ok := <-logChan:
		require.True(t, ok)
		require.NotNil(t, log)
		assert.Len(t, log.Streams, 1)
		assert.Equal(t, "streamed log message", log.Streams[0].Values[0][1])
	case <-ctx.Done():
		t.Fatal("timed out waiting for log message")
	}
}

// newDelayedReadyServer creates an httptest server whose /ready endpoint
// sleeps for the given delay before responding. This is used to test
// HTTP client timeout behavior.
func newDelayedReadyServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	// Server delays 12s, which exceeds the default 10s HTTP client timeout.
	// The request should fail with a timeout error.
	server := newDelayedReadyServer(12 * time.Second)
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	err = client.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestClient_SetTimeout_LargerThanDefault(t *testing.T) {
	// Server delays 12s, which exceeds the default 10s but is within the
	// dynamically set 30s timeout. The request should succeed.
	server := newDelayedReadyServer(12 * time.Second)
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)
	client.SetTimeout(30 * time.Second)

	err = client.Ready(context.Background())
	require.NoError(t, err)
}

func TestClient_SetTimeout_SmallerThanDefault(t *testing.T) {
	// Server delays 3s, which exceeds the dynamically set 1s timeout.
	// The request should fail with a timeout error.
	server := newDelayedReadyServer(3 * time.Second)
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)
	client.SetTimeout(1 * time.Second)

	err = client.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}

func TestClient_SetTimeout_Zero(t *testing.T) {
	// A timeout of 0 means no timeout. The server delays 3s, which should
	// succeed because there is no HTTP client timeout.
	server := newDelayedReadyServer(3 * time.Second)
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)
	client.SetTimeout(0)

	err = client.Ready(context.Background())
	require.NoError(t, err)
}

func TestClient_SetTimeout_RestoreDefault(t *testing.T) {
	// After extending the timeout and restoring the default, subsequent
	// requests should be governed by the default 10s timeout again.
	server := newDelayedReadyServer(12 * time.Second)
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	// Extend timeout — 12s delay should succeed
	client.SetTimeout(30 * time.Second)
	err = client.Ready(context.Background())
	require.NoError(t, err)

	// Restore default — 12s delay should now fail
	client.SetTimeout(DefaultTimeout)
	err = client.Ready(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadline exceeded")
}
