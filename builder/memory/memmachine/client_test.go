package memmachine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/types"
)

func TestClient_CreateProject(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/projects", r.URL.Path)

		var req types.CreateMemoryProjectRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "org", req.OrgID)
		assert.Equal(t, "proj", req.ProjectID)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(types.MemoryProjectResponse{
			OrgID:     "org",
			ProjectID: "proj",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/api/v2")
	client.logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	resp, err := client.CreateProject(context.Background(), &types.CreateMemoryProjectRequest{
		OrgID:     "org",
		ProjectID: "proj",
	})
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, "org", resp.OrgID)
		assert.Equal(t, "proj", resp.ProjectID)
	}
}

func TestClient_AddMemories(t *testing.T) {
	callCount := 0
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 2 {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v2/memories/list", r.URL.Path)
			var req memmachineListByUIDRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, "org", req.OrgID)
			assert.Equal(t, "proj", req.ProjectID)
			assert.Equal(t, string(types.MemoryTypeEpisodic), req.Type)
			assert.Contains(t, req.Filter, "uid in")
			episodicRaw, rawErr := json.Marshal([]memmachineEpisodic{{UID: "1", Content: "hello"}})
			assert.NoError(t, rawErr)
			_ = json.NewEncoder(w).Encode(memmachineSearchResponse{
				Status: 0,
				Content: memmachineSearchContent{
					EpisodicMemory: episodicRaw,
				},
			})
			return
		}
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/memories", r.URL.Path)

		var req memmachineAddRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "org", req.OrgID)
		assert.Equal(t, "proj", req.ProjectID)
		if assert.Len(t, req.Messages, 1) {
			assert.Equal(t, "hello", req.Messages[0].Content)
			assert.Equal(t, "user", req.Messages[0].ProducerRole)
			assert.Equal(t, map[string]any{
				"user_id":    "u1",
				"agent_id":   "agent",
				"session_id": "sess",
			}, req.Messages[0].Metadata)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]string{{"uid": "1"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/api/v2")
	resp, err := client.AddMemories(context.Background(), &types.AddMemoriesRequest{
		AgentID:   "agent",
		SessionID: "sess",
		OrgID:     "org",
		ProjectID: "proj",
		Types:     []types.MemoryType{types.MemoryTypeEpisodic},
		Messages: []types.MemoryMessage{
			{Content: "hello", Role: "user", UserID: "u1"},
		},
	})
	assert.NoError(t, err)
	if assert.Len(t, resp.Created, 1) {
		assert.Equal(t, "e_1", resp.Created[0].UID)
	}
}

func TestClient_SearchMemories(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/memories/search", r.URL.Path)

		var req memmachineSearchRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "org", req.OrgID)
		assert.Equal(t, "proj", req.ProjectID)
		assert.Equal(t, "hello", req.Query)
		assert.Equal(t, 1, req.PageNum)
		if assert.NotNil(t, req.ScoreThreshold) {
			assert.InDelta(t, 0.5, *req.ScoreThreshold, 0.0001)
		}

		episodicRaw, rawErr := json.Marshal([]memmachineEpisodic{{UID: "1", Content: "hello"}})
		assert.NoError(t, rawErr)
		_ = json.NewEncoder(w).Encode(memmachineSearchResponse{
			Status: 0,
			Content: memmachineSearchContent{
				EpisodicMemory: episodicRaw,
			},
		})
	}))
	defer server.Close()

	minSim := 0.5
	client := NewClient(server.URL, "/api/v2")
	resp, err := client.SearchMemories(context.Background(), &types.SearchMemoriesRequest{
		OrgID:         "org",
		ProjectID:     "proj",
		ContentQuery:  "hello",
		PageNum:       2,
		MinSimilarity: &minSim,
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Content, 1)
}

func TestClient_ListMemories(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v2/memories/list", r.URL.Path)

		var req memmachineSearchRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "org", req.OrgID)
		assert.Equal(t, "proj", req.ProjectID)
		assert.Equal(t, 0, req.PageNum)

		episodicRaw, rawErr := json.Marshal([]memmachineEpisodic{{UID: "1", Content: "hello"}})
		assert.NoError(t, rawErr)
		_ = json.NewEncoder(w).Encode(memmachineSearchResponse{
			Status: 0,
			Content: memmachineSearchContent{
				EpisodicMemory: episodicRaw,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/api/v2")
	resp, err := client.ListMemories(context.Background(), &types.ListMemoriesRequest{
		OrgID:     "org",
		ProjectID: "proj",
		PageNum:   1,
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Content, 1)
}

func TestClient_DeleteMemories(t *testing.T) {
	callCount := 0
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		assert.Equal(t, http.MethodPost, r.Method)
		switch r.URL.Path {
		case "/api/v2/memories/episodic/delete":
			var req memmachineDeleteEpisodicRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, "_global", req.OrgID)
			assert.Equal(t, "_public", req.ProjectID)
			assert.Equal(t, []string{"93"}, req.EpisodicIDs)
			w.WriteHeader(http.StatusNoContent)
		case "/api/v2/memories/semantic/delete":
			var req memmachineDeleteSemanticRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)
			assert.Equal(t, "_global", req.OrgID)
			assert.Equal(t, "_public", req.ProjectID)
			assert.Equal(t, []string{"20"}, req.SemanticIDs)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "/api/v2")
	err := client.DeleteMemories(context.Background(), &types.DeleteMemoriesRequest{UIDs: []string{"e_93", "s_20"}})
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestClient_Health(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v2/health", r.URL.Path)
		_ = json.NewEncoder(w).Encode(types.MemoryHealthResponse{Status: "healthy", Service: "memmachine"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "/api/v2")
	resp, err := client.Health(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
}

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to open listener in this environment: %v", err)
	}
	defaultHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/projects/get":
			body, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			var req types.GetMemoryProjectRequest
			if json.Unmarshal(body, &req) == nil && req.OrgID == "_global" && req.ProjectID == "_public" {
				_ = json.NewEncoder(w).Encode(types.MemoryProjectResponse{
					OrgID:     "_global",
					ProjectID: "_public",
				})
				return
			}
			handler.ServeHTTP(w, r)
		case "/api/v2/projects":
			body, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			var req types.CreateMemoryProjectRequest
			if json.Unmarshal(body, &req) == nil && req.OrgID == "_global" && req.ProjectID == "_public" {
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(types.MemoryProjectResponse{
					OrgID:     "_global",
					ProjectID: "_public",
				})
				return
			}
			handler.ServeHTTP(w, r)
		default:
			handler.ServeHTTP(w, r)
		}
	})
	server := httptest.NewUnstartedServer(defaultHandler)
	server.Listener = listener
	server.Start()
	return server
}
