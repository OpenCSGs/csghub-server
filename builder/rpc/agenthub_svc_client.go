package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
)

type AgentHubSvcClient interface {
	CreateAgentInstance(ctx context.Context, userUUID string, req *CreateAgentInstanceRequest) (*CreateAgentInstanceResponse, error)
	GetAgentInstances(ctx context.Context, req *GetAgentInstancesRequest) (GetAgentInstancesResponse, error)
}

type CreateAgentInstanceRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
}

type CreateAgentInstanceResponse AgentInstance

type GetAgentInstancesRequest struct {
	IDs      []string `json:"ids"`
	UserUUID string   `json:"user_uuid"`
}

type GetAgentInstancesResponse []*AgentInstance

/*
	{
	  "id": "new-flow-id",
	  "name": "New Flow Name",
	  "description": "Flow description",
	  "data": {
	    // Flow graph data
	  },
	  "hasIO": true,
	  "created_at": "2024-01-01T00:00:00Z",
	  "updated_at": "2024-01-01T00:00:00Z",
	  "user_id": "user-uuid",
	  "folder_id": "folder-uuid"
	}
*/
type AgentInstance struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	HasIO       bool            `json:"hasIO"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	UserUUID    string          `json:"user_id"` // user uuid
	FolderID    string          `json:"folder_id"`
}

type AgentHubSvcClientImpl struct {
	hc    *HttpClient
	token string
}

func NewAgentHubSvcClientImpl(endpoint string, token string, opts ...RequestOption) AgentHubSvcClient {
	return &AgentHubSvcClientImpl{
		hc:    NewHttpClient(strings.TrimSuffix(endpoint, "/"), opts...),
		token: token,
	}
}

// POST /api/v1/opencsg/flows/
func (c *AgentHubSvcClientImpl) CreateAgentInstance(ctx context.Context, userUUID string, req *CreateAgentInstanceRequest) (*CreateAgentInstanceResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("create agent instance request is nil"), nil)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/flows/",
	}
	var resp CreateAgentInstanceResponse
	var buf io.Reader

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/flows/?token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to create agent instance in agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		return nil, errorx.RemoteSvcFail(errors.New("failed to create agent instance in agenthub"), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return &resp, nil
}

// POST /api/v1/opencsg/flows/query
func (c *AgentHubSvcClientImpl) GetAgentInstances(ctx context.Context, req *GetAgentInstancesRequest) (GetAgentInstancesResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("get agent instances request is nil"), nil)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": req.UserUUID,
		"service":   "agenthub",
		"api":       "/api/v1/opencsg/flows/query",
	}

	var resp GetAgentInstancesResponse
	var buf io.Reader
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/opencsg/flows/query?token=" + c.token
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", req.UserUUID)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		return nil, errorx.RemoteSvcFail(errors.New("failed to get agent instance from agenthub"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		return nil, errorx.RemoteSvcFail(errors.New("failed to get agent instance from agenthub"), rpcErrorCtx)
	}
	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return resp, nil
}
