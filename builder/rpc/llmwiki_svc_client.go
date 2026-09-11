package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type LLMWikiSvcClient interface {
	CreateKnowledgeBase(ctx context.Context, req *LLMWikiCreateKnowledgeBaseRequest) (*LLMWikiCreateKnowledgeBaseResponse, error)
	UpdateKnowledgeBase(ctx context.Context, req *LLMWikiUpdateKnowledgeBaseRequest) error
	DeleteKnowledgeBase(ctx context.Context, req *LLMWikiDeleteKnowledgeBaseRequest) error
	GetKnowledgeBaseResourceState(ctx context.Context, req *LLMWikiGetKnowledgeBaseResourceStateRequest) (*LLMWikiKnowledgeBaseResourceState, error)
	BatchGetKnowledgeBaseResourceStates(ctx context.Context, req *LLMWikiBatchGetKnowledgeBaseResourceStatesRequest) (*LLMWikiBatchGetKnowledgeBaseResourceStatesResponse, error)
}

type LLMWikiCreateKnowledgeBaseRequest struct {
	ContentID   string
	Name        string
	Description string
	ActorID     string
}

type LLMWikiCreateKnowledgeBaseResponse struct {
	ContentID   string
	Name        string
	Description string
	Metadata    map[string]any
}

type LLMWikiUpdateKnowledgeBaseRequest struct {
	ContentID   string
	Name        *string
	Description *string
	ActorID     string
}

type LLMWikiDeleteKnowledgeBaseRequest struct {
	ContentID string
	ActorID   string
}

type LLMWikiGetKnowledgeBaseResourceStateRequest struct {
	ContentID string
	ActorID   string
}

type LLMWikiBatchGetKnowledgeBaseResourceStatesRequest struct {
	ContentIDs []string
	ActorID    string
}

type LLMWikiKnowledgeBaseStateError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// LLMWikiKnowledgeBaseResourceState is intentionally opaque because the
// upstream ResourceStateV2 schema is still evolving.
type LLMWikiKnowledgeBaseResourceState = json.RawMessage

type LLMWikiBatchGetKnowledgeBaseResourceStateResult struct {
	KBID          string                             `json:"kb_id"`
	Found         bool                               `json:"found"`
	ResourceState *LLMWikiKnowledgeBaseResourceState `json:"resource_state,omitempty"`
	Error         *LLMWikiKnowledgeBaseStateError    `json:"error,omitempty"`
}

type LLMWikiBatchGetKnowledgeBaseResourceStatesResponse struct {
	SchemaVersion string                                            `json:"schema_version"`
	Results       []LLMWikiBatchGetKnowledgeBaseResourceStateResult `json:"results"`
}

type LLMWikiSvcHttpClientImpl struct {
	hc *HttpClient
}

var _ LLMWikiSvcClient = (*LLMWikiSvcHttpClientImpl)(nil)

func NewLLMWikiSvcHttpClient(endpoint string, opts ...RequestOption) LLMWikiSvcClient {
	return &LLMWikiSvcHttpClientImpl{hc: NewHttpClient(endpoint, opts...)}
}

func (c *LLMWikiSvcHttpClientImpl) CreateKnowledgeBase(_ context.Context, req *LLMWikiCreateKnowledgeBaseRequest) (*LLMWikiCreateKnowledgeBaseResponse, error) {
	if req == nil {
		return nil, errorx.BadRequest(errors.New("llmwiki create knowledge base request is nil"), nil)
	}
	return &LLMWikiCreateKnowledgeBaseResponse{
		ContentID:   req.ContentID,
		Name:        req.Name,
		Description: req.Description,
	}, nil
}

func (c *LLMWikiSvcHttpClientImpl) UpdateKnowledgeBase(_ context.Context, _ *LLMWikiUpdateKnowledgeBaseRequest) error {
	return nil
}

func (c *LLMWikiSvcHttpClientImpl) DeleteKnowledgeBase(_ context.Context, _ *LLMWikiDeleteKnowledgeBaseRequest) error {
	return nil
}

func (c *LLMWikiSvcHttpClientImpl) GetKnowledgeBaseResourceState(ctx context.Context, req *LLMWikiGetKnowledgeBaseResourceStateRequest) (*LLMWikiKnowledgeBaseResourceState, error) {
	if req == nil || !types.AgentKnowledgeBaseIDPattern.MatchString(req.ContentID) {
		return nil, errorx.BadRequest(errors.New("invalid llmwiki knowledge base resource state request"), nil)
	}

	path := "/internal/v1/knowledge-bases/" + url.PathEscape(req.ContentID) + "/resource-state"
	var state LLMWikiKnowledgeBaseResourceState
	if err := c.doJSON(ctx, http.MethodGet, path, nil, req.ActorID, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *LLMWikiSvcHttpClientImpl) BatchGetKnowledgeBaseResourceStates(ctx context.Context, req *LLMWikiBatchGetKnowledgeBaseResourceStatesRequest) (*LLMWikiBatchGetKnowledgeBaseResourceStatesResponse, error) {
	if req == nil || len(req.ContentIDs) == 0 || len(req.ContentIDs) > 100 {
		return nil, errorx.BadRequest(errors.New("invalid llmwiki batch resource state request"), nil)
	}
	seen := make(map[string]struct{}, len(req.ContentIDs))
	for _, contentID := range req.ContentIDs {
		if !types.AgentKnowledgeBaseIDPattern.MatchString(contentID) {
			return nil, errorx.BadRequest(errors.New("invalid llmwiki knowledge base identifier"), nil)
		}
		if _, ok := seen[contentID]; ok {
			return nil, errorx.BadRequest(errors.New("duplicate llmwiki knowledge base identifier"), nil)
		}
		seen[contentID] = struct{}{}
	}

	body := struct {
		SchemaVersion string   `json:"schema_version"`
		KBIDs         []string `json:"kb_ids"`
	}{SchemaVersion: "resource-status-batch-request.v1", KBIDs: req.ContentIDs}
	var response LLMWikiBatchGetKnowledgeBaseResourceStatesResponse
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/knowledge-bases:batchGetResourceStatus", body, req.ActorID, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *LLMWikiSvcHttpClientImpl) doJSON(ctx context.Context, method, path string, body any, actorID string, out any) error {
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal llmwiki request: %w", err)
		}
		requestBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.hc.endpoint+path, requestBody)
	if err != nil {
		return fmt.Errorf("create llmwiki request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actorID != "" {
		req.Header.Set(types.HeaderCSGHubActorID, actorID)
	}
	for _, opt := range c.hc.authOpts {
		opt.Set(req)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "llmwiki service").Set("action", "get knowledge base resource state"))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(fmt.Errorf("llmwiki service returned status %d", resp.StatusCode), errorx.Ctx().Set("service", "llmwiki service").Set("action", "get knowledge base resource state"))
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return errorx.RemoteSvcFail(fmt.Errorf("decode llmwiki response: %w", err), errorx.Ctx().Set("service", "llmwiki service").Set("action", "get knowledge base resource state"))
	}
	return nil
}
