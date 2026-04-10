package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type CsgbotSvcClient interface {
	DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error
	UpdateWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error
	CreateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, req *CreateKnowledgeBaseRequest) (*CreateKnowledgeBaseResponse, error)
	DeleteKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string) error
	UpdateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, contentID string, req *types.UpdateAgentKnowledgeBaseRequest) error

	Chat(ctx context.Context, userUUID, username, token, agentName, sessionID string, req *CsgbotChatRequest) (io.ReadCloser, error)
	ChatLangflow(ctx context.Context, userUUID, username, token, flowID, sessionID string, req *LangflowSchedulerChatRequest) (io.ReadCloser, error)
	ChatCodeAgent(ctx context.Context, userUUID, username, token string, req *CodeAgentSchedulerChatRequest) (io.ReadCloser, error)
	CreateOpenClaw(ctx context.Context, userUUID, username, token string, req CreateOpenClawRequest) (*CreateOpenClawResponse, error)
	DeleteOpenClaw(ctx context.Context, userUUID, username, token, contentID string) error
}

type CsgbotChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // can be a string or []map[string]any for Claude content blocks
}

type CsgbotChatRequest struct {
	Model    string              `json:"model,omitempty"`
	Messages []CsgbotChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// LangflowSchedulerChatRequest represents a chat request to langflow for scheduler
type LangflowSchedulerChatRequest struct {
	InputValue string `json:"input_value"`
	InputType  string `json:"input_type"`
	OutputType string `json:"output_type"`
	SessionID  string `json:"session_id,omitempty"`
}

// CodeAgentSchedulerChatRequest represents a chat request to a custom code agent for scheduler
type CodeAgentSchedulerChatRequest struct {
	RequestID     string                     `json:"request_id"`
	Query         string                     `json:"query"`
	AgentName     string                     `json:"agent_name"`
	Stream        bool                       `json:"stream"`
	StreamMode    map[string]any             `json:"streamMode,omitempty"`
	History       []CodeAgentChatHistoryItem `json:"history,omitempty"`
	MaxLoop       int                        `json:"maxLoop,omitempty"`
	SearchEngines []string                   `json:"search_engines,omitempty"`
}

// CodeAgentChatHistoryItem represents a history item for code agent chat
type CodeAgentChatHistoryItem struct {
	Content string `json:"content"`
	Role    string `json:"role"`
	File    any    `json:"file"`
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

// CreateOpenClawRequest represents a request to create an OpenClaw sandbox.
// Supports dynamic fields driven by instance.metadata["source_request"].
type CreateOpenClawRequest map[string]any

// CreateOpenClawResponse represents the response from creating an OpenClaw sandbox.
type CreateOpenClawResponse struct {
	ID       string         `json:"id"`       // CSGHub content ID
	Metadata map[string]any `json:"metadata"` // Metadata including endpoint URL and other fields
}

// DeleteOpenClawResponse represents the response from deleting an OpenClaw sandbox.
type DeleteOpenClawResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type UpdateWorkspaceFilesRequest struct {
	AgentName string `json:"agent_name"`
}

type CsgbotSvcHttpClientImpl struct {
	hc *HttpClient
}

func NewCsgbotSvcHttpClient(endpoint string, opts ...RequestOption) CsgbotSvcClient {
	return &CsgbotSvcHttpClientImpl{
		hc: NewHttpClient(endpoint, opts...),
	}
}

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

func (c *CsgbotSvcHttpClientImpl) UpdateWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"api":       "POST /api/v1/csgbot/codeAgent/updateCode",
	}

	path := c.hc.endpoint + "/api/v1/csgbot/codeAgent/updateCode"
	req := UpdateWorkspaceFilesRequest{
		AgentName: agentName,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)
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
		slog.ErrorContext(ctx, "failed to update workspace files for code agent", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to update workspace files for code agent"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to update workspace files for code agent", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to update workspace files for code agent"), rpcErrorCtx)
	}

	return nil
}

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

func (c *CsgbotSvcHttpClientImpl) Chat(ctx context.Context, userUUID, username, token, agentName, sessionID string, req *CsgbotChatRequest) (io.ReadCloser, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("chat request is nil"), nil)
	}

	headers := map[string]string{
		types.CSGBotHeaderAgentName: agentName,
		types.CSGBotHeaderSessionID: sessionID,
	}
	return c.doStreamChatRequest(ctx, "/api/v1/csgbot/"+agentName+"/chat", userUUID, username, token, req, req.Stream, headers)
}

func (c *CsgbotSvcHttpClientImpl) ChatLangflow(ctx context.Context, userUUID, username, token, flowID, sessionID string, req *LangflowSchedulerChatRequest) (io.ReadCloser, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("langflow chat request is nil"), nil)
	}

	headers := map[string]string{
		types.CSGBotHeaderSessionID: sessionID,
	}
	return c.doStreamChatRequest(ctx, "/api/v1/csgbot/langflow/flows/run/"+flowID+"?stream=true", userUUID, username, token, req, true, headers)
}

func (c *CsgbotSvcHttpClientImpl) ChatCodeAgent(ctx context.Context, userUUID, username, token string, req *CodeAgentSchedulerChatRequest) (io.ReadCloser, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("code agent chat request is nil"), nil)
	}

	return c.doStreamChatRequest(ctx, "/api/v1/csgbot/codeAgent", userUUID, username, token, req, true, nil)
}

func (c *CsgbotSvcHttpClientImpl) doStreamChatRequest(ctx context.Context, path, userUUID, username, token string, req any, stream bool, extraHeaders map[string]string) (io.ReadCloser, error) {
	const errMessage = "failed to call csgbot chat"
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"path":      path,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)

	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.hc.endpoint+path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}

	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set(types.CSGBotHeaderRequestID, uuid.New().String())
	hreq.Header.Set(types.CSGBotHeaderUserUUID, userUUID)
	hreq.Header.Set(types.CSGBotHeaderUserName, username)
	hreq.Header.Set(types.CSGBotHeaderUserToken, token)
	if stream {
		hreq.Header.Set("Accept", "text/event-stream")
	}
	for key, value := range extraHeaders {
		hreq.Header.Set(key, value)
	}

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, errMessage, "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.RemoteSvcFail(errors.New(errMessage), rpcErrorCtx)
	}

	if hresp.StatusCode != http.StatusOK {
		defer hresp.Body.Close()
		body, _ := io.ReadAll(hresp.Body)
		return nil, errorx.RemoteSvcFail(fmt.Errorf("%s with status %d: %s", errMessage, hresp.StatusCode, string(body)), rpcErrorCtx)
	}

	return hresp.Body, nil
}

func (c *CsgbotSvcHttpClientImpl) CreateOpenClaw(ctx context.Context, userUUID, username, token string, req CreateOpenClawRequest) (*CreateOpenClawResponse, error) {
	// Ensure req is not nil to avoid null JSON body
	if req == nil {
		req = make(CreateOpenClawRequest)
	}
	rpcErrorCtx := map[string]any{
		"user_uuid": userUUID,
		"service":   "csgbot",
		"api":       "POST /api/v1/csgbot/openclaw/create",
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}
	buf := bytes.NewBuffer(jsonData)

	path := c.hc.endpoint + "/api/v1/csgbot/openclaw/create"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, path, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}

	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set(types.CSGBotHeaderUserUUID, userUUID)
	hreq.Header.Set(types.CSGBotHeaderUserName, username)
	hreq.Header.Set(types.CSGBotHeaderUserToken, token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create openclaw sandbox", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.RemoteSvcFail(errors.New("failed to create openclaw sandbox"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to create openclaw sandbox", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.RemoteSvcFail(errors.New("failed to create openclaw sandbox"), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read openclaw create response", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}

	var resp CreateOpenClawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal openclaw create response", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return nil, errorx.InternalServerError(err, rpcErrorCtx)
	}

	return &resp, nil
}

func (c *CsgbotSvcHttpClientImpl) DeleteOpenClaw(ctx context.Context, userUUID, username, token, contentID string) error {
	rpcErrorCtx := map[string]any{
		"user_uuid":  userUUID,
		"content_id": contentID,
		"service":    "csgbot",
		"api":        "DELETE /api/v1/csgbot/openclaw/{id}",
	}

	path := c.hc.endpoint + "/api/v1/csgbot/openclaw/" + contentID
	hreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return errorx.InternalServerError(err, rpcErrorCtx)
	}

	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set(types.CSGBotHeaderUserUUID, userUUID)
	hreq.Header.Set(types.CSGBotHeaderUserName, username)
	hreq.Header.Set(types.CSGBotHeaderUserToken, token)

	hresp, err := c.hc.Do(hreq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete openclaw sandbox", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to delete openclaw sandbox"), rpcErrorCtx)
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "failed to delete openclaw sandbox", "status_code", hresp.StatusCode, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(errors.New("failed to delete openclaw sandbox"), rpcErrorCtx)
	}

	body, err := io.ReadAll(hresp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read openclaw delete response", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.InternalServerError(err, rpcErrorCtx)
	}

	var resp DeleteOpenClawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal openclaw delete response", "error", err, "rpc_error_ctx", rpcErrorCtx)
		return errorx.InternalServerError(err, rpcErrorCtx)
	}

	if !resp.Success {
		errMsg := resp.Error
		if errMsg == "" {
			errMsg = "unknown error"
		}
		slog.ErrorContext(ctx, "openclaw delete returned failure", "error", errMsg, "rpc_error_ctx", rpcErrorCtx)
		return errorx.RemoteSvcFail(fmt.Errorf("openclaw delete failed: %s", errMsg), rpcErrorCtx)
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
