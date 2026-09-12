package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMWikiSvcHttpClientPlaceholder(t *testing.T) {
	client := NewLLMWikiSvcHttpClient("http://127.0.0.1:1")
	req := &LLMWikiCreateKnowledgeBaseRequest{
		ContentID:   "kb-existing",
		Name:        "Existing KB",
		Description: "managed by llmwiki",
		ActorID:     "user-uuid",
	}

	resp, err := client.CreateKnowledgeBase(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, req.ContentID, resp.ContentID)
	require.Equal(t, req.Name, resp.Name)
	require.Equal(t, req.Description, resp.Description)
	require.NoError(t, client.UpdateKnowledgeBase(context.Background(), &LLMWikiUpdateKnowledgeBaseRequest{}))
	require.NoError(t, client.DeleteKnowledgeBase(context.Background(), &LLMWikiDeleteKnowledgeBaseRequest{}))

	resp, err = client.CreateKnowledgeBase(context.Background(), nil)
	require.Nil(t, resp)
	require.Error(t, err)
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
