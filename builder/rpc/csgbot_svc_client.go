package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type CsgbotSvcClient interface {
	DeleteWorkspaceFiles(ctx context.Context, userUUID string, username string, token string, agentName string) error
	CreateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, req *CreateKnowledgeBaseRequest) (*CreateKnowledgeBaseResponse, error)
	DeleteKnowledgeBase(ctx context.Context, userUUID string, contentID string) error
	UpdateKnowledgeBase(ctx context.Context, userUUID string, contentID string, req *types.UpdateAgentKnowledgeBaseRequest) error
}

type CreateKnowledgeBaseRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ContentID   string `json:"content_id"`
	Public      *bool  `json:"public,omitempty"`
}

type CreateKnowledgeBaseResponse struct {
	ContentID string `json:"content_id"`
	Name      string `json:"name"`
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
// POST /api/v1/csgbot/knowledgeBase
func (c *CsgbotSvcHttpClientImpl) CreateKnowledgeBase(ctx context.Context, userUUID string, username string, token string, req *CreateKnowledgeBaseRequest) (*CreateKnowledgeBaseResponse, error) {
	// TODO: implement
	return &CreateKnowledgeBaseResponse{
		ContentID: uuid.New().String(),
		Name:      req.Name,
	}, nil
}

// Delete knowledge base
// DELETE /api/v1/csgbot/knowledgeBase/{content_id}
func (c *CsgbotSvcHttpClientImpl) DeleteKnowledgeBase(ctx context.Context, userUUID string, contentID string) error {
	// TODO: implement
	return nil
}

// Update knowledge base
// PUT /api/v1/csgbot/knowledgeBase/{content_id}
func (c *CsgbotSvcHttpClientImpl) UpdateKnowledgeBase(ctx context.Context, userUUID string, contentID string, req *types.UpdateAgentKnowledgeBaseRequest) error {
	// TODO: implement
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
