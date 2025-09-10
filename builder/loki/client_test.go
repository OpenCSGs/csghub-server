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

	logChan, err := client.Tail(ctx, `{app="test-app"}`, time.Now())
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
