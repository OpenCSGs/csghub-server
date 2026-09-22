package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMWikiSvcHttpClientCreateKnowledgeBase(t *testing.T) {
	endpoint := "http://llmwiki-create"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/internal/v1/knowledge-bases", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/internal/v1/knowledge-bases", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "user-uuid", r.Header.Get(types.HeaderCSGHubActorID))
		key := r.Header.Get("Idempotency-Key")
		require.True(t, strings.HasPrefix(key, "llmwiki-kb-create-"))
		require.GreaterOrEqual(t, len(key), 16)
		require.LessOrEqual(t, len(key), 128)
		var body struct {
			SchemaVersion  string `json:"schema_version"`
			KBID           string `json:"kb_id"`
			StorageProfile struct {
				ProfileID string `json:"profile_id"`
			} `json:"storage_profile"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "knowledge-base-create-request.v1", body.SchemaVersion)
		require.Equal(t, "kb-new", body.KBID)
		require.Equal(t, "profile_default", body.StorageProfile.ProfileID)
		return httpmock.NewStringResponse(http.StatusCreated, `{
			"schema_version":"resource-state.v2","kb_id":"kb-new","resource_version":1,
			"mcp_endpoint_url":"http://llmwiki.internal/mcp","provisioning_status":"pending"
		}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	resp, err := client.CreateKnowledgeBase(context.Background(), &LLMWikiCreateKnowledgeBaseRequest{
		ContentID:   "kb-new",
		Name:        "New KB",
		Description: "auto-provisioned",
		ActorID:     "user-uuid",
	})
	require.NoError(t, err)
	require.Equal(t, "kb-new", resp.ContentID)
	require.Equal(t, "New KB", resp.Name)
	require.Equal(t, "auto-provisioned", resp.Description)
	require.NotNil(t, resp.ResourceState)
	require.Equal(t, "http://llmwiki.internal/mcp", resp.Metadata[types.AgentKnowledgeBaseMetadataMCPEndpointURLKey])
}

func TestLLMWikiSvcHttpClientCreateKnowledgeBaseEmptyBody(t *testing.T) {
	endpoint := "http://llmwiki-create-empty"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/internal/v1/knowledge-bases", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusCreated, ""), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	resp, err := client.CreateKnowledgeBase(context.Background(), &LLMWikiCreateKnowledgeBaseRequest{
		ContentID: "kb-empty",
		Name:      "Empty body KB",
		ActorID:   "user-uuid",
	})
	require.NoError(t, err)
	require.Equal(t, "kb-empty", resp.ContentID)
	require.Empty(t, resp.Metadata)
}

func TestLLMWikiIdempotencyKey(t *testing.T) {
	key1 := llmWikiIdempotencyKey("create", "kb-one")
	key2 := llmWikiIdempotencyKey("create", "kb-one")
	require.NotEqual(t, key1, key2, "each attempt must get a unique key")
	require.True(t, strings.HasPrefix(key1, "llmwiki-kb-create-"))
	require.GreaterOrEqual(t, len(key1), 16)
	require.LessOrEqual(t, len(key1), 128)
}

func TestLLMWikiSvcHttpClientCreateKnowledgeBaseConflict(t *testing.T) {
	endpoint := "http://llmwiki-create-conflict"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/internal/v1/knowledge-bases", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusConflict, `{"code":"RESOURCE_ALREADY_EXISTS","message":"kb exists"}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	resp, err := client.CreateKnowledgeBase(context.Background(), &LLMWikiCreateKnowledgeBaseRequest{
		ContentID: "kb-existing",
		Name:      "Existing KB",
		ActorID:   "user-uuid",
	})
	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "409")
	require.Contains(t, err.Error(), "RESOURCE_ALREADY_EXISTS")
}

func TestLLMWikiSvcHttpClientCreateKnowledgeBaseInvalidRequest(t *testing.T) {
	endpoint := "http://llmwiki-create-invalid"
	client := NewLLMWikiSvcHttpClient(endpoint)

	resp, err := client.CreateKnowledgeBase(context.Background(), nil)
	require.Nil(t, resp)
	require.Error(t, err)

	resp, err = client.CreateKnowledgeBase(context.Background(), &LLMWikiCreateKnowledgeBaseRequest{
		ContentID: "-bad-id",
		ActorID:   "user-uuid",
	})
	require.Nil(t, resp)
	require.Error(t, err)

	info := httpmock.GetCallCountInfo()
	require.Zero(t, info[http.MethodPost+" "+endpoint+"/internal/v1/knowledge-bases"])
}

func TestLLMWikiSvcHttpClientUpdateKnowledgeBase(t *testing.T) {
	client := NewLLMWikiSvcHttpClient("http://127.0.0.1:1")
	require.NoError(t, client.UpdateKnowledgeBase(context.Background(), &LLMWikiUpdateKnowledgeBaseRequest{}))
}

func TestLLMWikiSvcHttpClientDeleteKnowledgeBase(t *testing.T) {
	endpoint := "http://llmwiki-delete"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/internal/v1/knowledge-bases/kb-gone/resource-state", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "user-uuid", r.Header.Get(types.HeaderCSGHubActorID))
		return httpmock.NewStringResponse(http.StatusOK, `{
			"schema_version":"resource-state.v2","kb_id":"kb-gone","resource_version":7
		}`), nil
	})
	httpmock.RegisterResponder(http.MethodDelete, endpoint+"/internal/v1/knowledge-bases/kb-gone", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "user-uuid", r.Header.Get(types.HeaderCSGHubActorID))
		require.True(t, strings.HasPrefix(r.Header.Get("Idempotency-Key"), "llmwiki-kb-delete-"))
		var body struct {
			SchemaVersion   string `json:"schema_version"`
			KBID            string `json:"kb_id"`
			ResourceVersion int64  `json:"resource_version"`
			Reason          string `json:"reason"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "knowledge-base-delete-request.v1", body.SchemaVersion)
		require.Equal(t, "kb-gone", body.KBID)
		require.Equal(t, int64(7), body.ResourceVersion)
		require.NotEmpty(t, body.Reason)
		return httpmock.NewStringResponse(http.StatusOK, `{}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	err := client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{
		ContentID: "kb-gone",
		ActorID:   "user-uuid",
	})
	require.NoError(t, err)
}

func TestLLMWikiSvcHttpClientDeleteKnowledgeBaseAlreadyGone(t *testing.T) {
	endpoint := "http://llmwiki-delete-gone"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/internal/v1/knowledge-bases/kb-gone/resource-state", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusNotFound, `{"code":"NOT_FOUND","message":"missing"}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	err := client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{
		ContentID: "kb-gone",
		ActorID:   "user-uuid",
	})
	require.NoError(t, err)
	info := httpmock.GetCallCountInfo()
	require.Zero(t, info[http.MethodDelete+" "+endpoint+"/internal/v1/knowledge-bases/kb-gone"])
}

func TestLLMWikiSvcHttpClientDeleteKnowledgeBaseAlreadyDeleted(t *testing.T) {
	endpoint := "http://llmwiki-delete-deleted"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/internal/v1/knowledge-bases/kb-gone/resource-state", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusOK, `{
			"schema_version":"resource-state.v2","kb_id":"kb-gone","resource_version":3
		}`), nil
	})
	httpmock.RegisterResponder(http.MethodDelete, endpoint+"/internal/v1/knowledge-bases/kb-gone", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusConflict, `{"code":"KB_DELETED","message":"already deleted"}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	err := client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{
		ContentID: "kb-gone",
		ActorID:   "user-uuid",
	})
	require.NoError(t, err)
}

func TestLLMWikiSvcHttpClientDeleteKnowledgeBaseResourceStateFailure(t *testing.T) {
	endpoint := "http://llmwiki-delete-fail"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/internal/v1/knowledge-bases/kb-gone/resource-state", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusInternalServerError, `{"code":"INTERNAL","message":"boom"}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	err := client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{
		ContentID: "kb-gone",
		ActorID:   "user-uuid",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
	info := httpmock.GetCallCountInfo()
	require.Zero(t, info[http.MethodDelete+" "+endpoint+"/internal/v1/knowledge-bases/kb-gone"])
}

func TestLLMWikiSvcHttpClientDeleteKnowledgeBaseInvalidRequest(t *testing.T) {
	client := NewLLMWikiSvcHttpClient("http://127.0.0.1:1")
	require.Error(t, client.DeleteKnowledgeBase(context.Background(), nil))
	require.Error(t, client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{ContentID: "-bad-id"}))
}

func TestLLMWikiSvcHttpClientGetKnowledgeBaseResourceState(t *testing.T) {
	observedAt := "2026-08-24T00:10:00Z"
	endpoint := "http://llmwiki-get"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/internal/v1/knowledge-bases/kb-east/resource-state", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/internal/v1/knowledge-bases/kb-east/resource-state", r.URL.Path)
		require.Equal(t, "user-uuid", r.Header.Get(types.HeaderCSGHubActorID))
		return httpmock.NewStringResponse(http.StatusOK, `{
			"schema_version":"resource-state.v1","kb_id":"kb-east","resource_version":7,
			"storage_profile_id":"storage-primary","desired_state":"present",
			"provisioning_status":"ready","readiness":"ready","runtime_status":"ready","index_status":"ready",
			"current_generation_id":"gen-active","candidate_generation_ids":["gen-candidate"],
			"mcp_endpoint_id":"mcp-private","mcp_endpoint_url":"http://llmwiki.internal/mcp","mcp_status":"ready",
			"last_error":null,"observed_at":"`+observedAt+`",
			"content_status":"available","health_status":"healthy",
			"workspace_task":{"kind":"compile","progress":{"percent":50}}
		}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	state, err := client.GetKnowledgeBaseResourceState(context.Background(), &LLMWikiGetKnowledgeBaseResourceStateRequest{
		ContentID: "kb-east",
		ActorID:   "user-uuid",
	})
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(*state, &fields))
	require.JSONEq(t, `"kb-east"`, string(fields["kb_id"]))
	require.JSONEq(t, `7`, string(fields["resource_version"]))
	require.JSONEq(t, `"http://llmwiki.internal/mcp"`, string(fields["mcp_endpoint_url"]))
	require.Contains(t, fields, "workspace_task")
}

func TestLLMWikiSvcHttpClientBatchGetKnowledgeBaseResourceStates(t *testing.T) {
	endpoint := "http://llmwiki-batch"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/internal/v1/knowledge-bases:batchGetResourceStatus", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/internal/v1/knowledge-bases:batchGetResourceStatus", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body struct {
			SchemaVersion string   `json:"schema_version"`
			KBIDs         []string `json:"kb_ids"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "resource-status-batch-request.v1", body.SchemaVersion)
		require.Equal(t, []string{"kb-one", "kb-missing"}, body.KBIDs)
		return httpmock.NewStringResponse(http.StatusOK, `{
			"schema_version":"resource-status-batch-response.v1","results":[
				{"kb_id":"kb-one","found":true,"resource_state":{
					"schema_version":"resource-state.v1","kb_id":"kb-one","resource_version":1,
					"storage_profile_id":"storage","desired_state":"present","provisioning_status":"pending",
					"readiness":"unknown","runtime_status":"pending","index_status":"absent",
					"current_generation_id":null,"candidate_generation_ids":[],"mcp_endpoint_id":null,
					"mcp_endpoint_url":null,"mcp_status":"unprovisioned","last_error":null,
					"observed_at":"2026-08-24T00:10:00Z"}},
				{"kb_id":"kb-missing","found":false,"error":{"code":"NOT_FOUND","message":"missing"}}
			]
		}`), nil
	})

	client := NewLLMWikiSvcHttpClient(endpoint)
	response, err := client.BatchGetKnowledgeBaseResourceStates(context.Background(), &LLMWikiBatchGetKnowledgeBaseResourceStatesRequest{
		ContentIDs: []string{"kb-one", "kb-missing"},
		ActorID:    "user-uuid",
	})
	require.NoError(t, err)
	require.Len(t, response.Results, 2)
	require.True(t, response.Results[0].Found)
	require.False(t, response.Results[1].Found)
	require.Equal(t, "NOT_FOUND", response.Results[1].Error.Code)
}
