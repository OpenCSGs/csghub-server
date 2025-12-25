package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type CsgbotSvcClient interface {
	DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error
	CreateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, req *CreateKnowledgeBaseRequest) (*CreateKnowledgeBaseResponse, error)
	DeleteKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string) error
	UpdateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string, req *types.UpdateAgentKnowledgeBaseRequest) error
}

type CreateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CreateKnowledgeBaseResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	IsComponent bool            `json:"is_component"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Webhook     bool            `json:"webhook"`
	Tags        []string        `json:"tags"`
	Locked      bool            `json:"locked"`
	McpEnabled  bool            `json:"mcp_enabled"`
	AccessType  string          `json:"access_type"`
	UserUUID    string          `json:"user_id"` // user uuid
	FolderID    string          `json:"folder_id"`
	Model       string          `json:"model"`
}

type UpdateKnowledgeBaseRequest struct {
	Name              string          `json:"name,omitempty"`
	Description       string          `json:"description,omitempty"`
	Data              json.RawMessage `json:"data,omitempty"`
	FolderID          string          `json:"folder_id,omitempty"`
	EndpointName      string          `json:"endpoint_name,omitempty"`
	MCPEnabled        *bool           `json:"mcp_enabled,omitempty"`
	Locked            *bool           `json:"locked,omitempty"`
	ActionName        string          `json:"action_name,omitempty"`
	ActionDescription string          `json:"action_description,omitempty"`
	AccessType        string          `json:"access_type,omitempty"`
	FSPath            string          `json:"fs_path,omitempty"`
}

type DeleteKnowledgeBaseRequest struct {
	IDs []string `json:"ids"`
}

type DeleteKnowledgeBaseResponse struct {
	IDs   []string `json:"ids"`
	Total int      `json:"total"`
}

type CsgbotSvcHttpClientImpl struct {
	hc *HttpClient
}

func NewCsgbotSvcHttpClient(endpoint string, opts ...RequestOption) CsgbotSvcClient {
	return &CsgbotSvcHttpClientImpl{
		hc: NewHttpClient(endpoint, opts...),
	}
}

// Delete workspace files for a code agent
// DELETE /api/v1/csgbot/codeAgent/{agent_name}
func (c *CsgbotSvcHttpClientImpl) DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"api":       "DELETE /api/v1/csgbot/codeAgent/{agent_name}",
	}

	path := c.hc.endpoint + "/api/v1/csgbot/codeAgent/" + agentName
	hreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)
	hreq.Header.Set("user_name", username)
	hreq.Header.Set("user_token", token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete workspace files for code agent", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to delete workspace files for code agent"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to delete workspace files for code agent", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to delete workspace files for code agent"), rpcErrorCtx)
	}

	return nil
}

// Create knowledge base
// POST /api/v1/csgbot/langflow/flows/rag
func (c *CsgbotSvcHttpClientImpl) CreateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, req *CreateKnowledgeBaseRequest) (*CreateKnowledgeBaseResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("create knowledge base request is nil"), nil)
	}

	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"api":       "POST /api/v1/csgbot/langflow/flows/rag",
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/csgbot/langflow/flows/rag"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)
	hreq.Header.Set("user_name", username)
	hreq.Header.Set("user_token", token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create knowledge base in csgbot service", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.RemoteSvcFail(errors.New("failed to create knowledge base in csgbot service"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to create knowledge base in csgbot service", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.RemoteSvcFail(errors.New("failed to create knowledge base in csgbot service"), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create knowledge base in csgbot service", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	var resp CreateKnowledgeBaseResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	return &resp, nil
}

// Delete knowledge base
// POST /api/v1/csgbot/langflow/flows/rag/delete
func (c *CsgbotSvcHttpClientImpl) DeleteKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid":  userUUID,
		"content_id": contentID,
		"service":    "csgbot",
		"api":        "POST /api/v1/csgbot/langflow/flows/rag/delete",
	}
	var resp DeleteKnowledgeBaseResponse

	req := DeleteKnowledgeBaseRequest{
		IDs: []string{contentID},
	}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/csgbot/langflow/flows/rag/delete"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}

	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)
	hreq.Header.Set("user_name", username)
	hreq.Header.Set("user_token", token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete knowledge base in csgbot service", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service, status code: "+strconv.Itoa(hresp.StatusCode)), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service, unmarshal response error: "+err.Error()), rpcErrorCtx)
	}

	if resp.Total != 1 {
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service, total: "+strconv.Itoa(resp.Total)), rpcErrorCtx)
	}

	if len(resp.IDs) == 0 {
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service, response IDs is empty"), rpcErrorCtx)
	}

	if resp.IDs[0] != contentID {
		return errorx.RemoteSvcFail(errors.New("failed to delete knowledge base in csgbot service, content ID mismatch: "+contentID+" != "+resp.IDs[0]), rpcErrorCtx)
	}
	return nil
}

// Update knowledge base
// PATCH /api/v1/csgbot/langflow/flows/rag/{content_id}
func (c *CsgbotSvcHttpClientImpl) UpdateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string, req *types.UpdateAgentKnowledgeBaseRequest) error {
	if req == nil {
		return errorx.BadRequest(errors.New("update knowledge base request is nil"), nil)
	}

	var updateKnowledgeBaseReq UpdateKnowledgeBaseRequest
	if req.Name != nil {
		updateKnowledgeBaseReq.Name = *req.Name
	}
	if req.Description != nil {
		updateKnowledgeBaseReq.Description = *req.Description
	}

	rpcErrorCtx := map[string]any{
		"user_uuid":  userUUID,
		"content_id": contentID,
		"service":    "csgbot",
		"api":        "PATCH /api/v1/csgbot/langflow/flows/rag/:content_id",
	}
	var buf io.Reader
	jsonData, err := json.Marshal(updateKnowledgeBaseReq)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf = bytes.NewBuffer(jsonData)
	path := c.hc.endpoint + "/api/v1/csgbot/langflow/flows/rag/" + contentID
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPatch, path, buf)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("user_uuid", userUUID)
	hreq.Header.Set("user_name", username)
	hreq.Header.Set("user_token", token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update knowledge base in csgbot service", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to update knowledge base in csgbot service"), rpcErrorCtx)
	}
	defer hresp.Body.Close()
	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to update knowledge base in csgbot service", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to update knowledge base in csgbot service"), rpcErrorCtx)
	}

	return nil
}

func NewCsgbotSvcHttpClientBuilder(endpoint string, opts ...RequestOption) CsgbotSvcClientBuilder {
	return &CsgbotSvcHttpClientImpl{
		hc: NewHttpClient(endpoint, opts...),
	}
}

type CsgbotSvcClientBuilder interface {
	WithRetry(attempts uint) CsgbotSvcClientBuilder
	WithDelay(delay time.Duration) CsgbotSvcClientBuilder
	Build() CsgbotSvcClient
}

func (c *CsgbotSvcHttpClientImpl) WithRetry(attempts uint) CsgbotSvcClientBuilder {
	c.hc = c.hc.WithRetry(attempts)
	return c
}

func (c *CsgbotSvcHttpClientImpl) WithDelay(delay time.Duration) CsgbotSvcClientBuilder {
	c.hc = c.hc.WithDelay(delay)
	return c
}

func (c *CsgbotSvcHttpClientImpl) Build() CsgbotSvcClient {
	return c
}
