package rpc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/google/uuid"
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
	ContentID     string
	Name          string
	Description   string
	Metadata      map[string]any
	ResourceState *LLMWikiKnowledgeBaseResourceState
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

const (
	llmWikiStorageProfileID    = "profile_default"
	llmWikiCreateSchemaVersion = "knowledge-base-create-request.v1"
	llmWikiDeleteSchemaVersion = "knowledge-base-delete-request.v1"
	llmWikiErrorBodyLimit      = 4096
	llmWikiErrorCodeKBDeleted  = "KB_DELETED"
	llmWikiDeleteReason        = "knowledge base deleted via csghub-server"
)

// llmWikiStatusError carries the upstream HTTP status and parsed error code so
// callers can apply idempotency rules (e.g. treat 404/409 KB_DELETED as
// already-gone on delete).
type llmWikiStatusError struct {
	status int
	code   string
}

func (e *llmWikiStatusError) Error() string {
	if e.code != "" {
		return fmt.Sprintf("llmwiki service returned status %d (%s)", e.status, e.code)
	}
	return fmt.Sprintf("llmwiki service returned status %d", e.status)
}

// llmWikiIdempotencyKey derives a per-attempt idempotency key (16-128 chars
// per upstream contract). The content-ID hash keeps keys traceable, while the
// random suffix makes every logical operation attempt unique: transport-level
// retries within one call still replay the original result, but a fresh
// attempt — in particular delete-then-recreate of the same content ID — can
// never replay a stale response from a previous resource lifecycle.
func llmWikiIdempotencyKey(op, contentID string) string {
	sum := sha256.Sum256([]byte(op + ":" + contentID))
	suffix := uuid.NewString()[:8]
	return "llmwiki-kb-" + op + "-" + hex.EncodeToString(sum[:])[:16] + "-" + suffix
}

func llmWikiMCPEndpointFromResourceState(state LLMWikiKnowledgeBaseResourceState) string {
	var probe struct {
		MCPEndpointURL string `json:"mcp_endpoint_url"`
	}
	if err := json.Unmarshal(state, &probe); err != nil {
		return ""
	}
	return probe.MCPEndpointURL
}

func llmWikiResourceVersionFromState(state LLMWikiKnowledgeBaseResourceState) (int64, error) {
	var probe struct {
		ResourceVersion json.Number `json:"resource_version"`
	}
	if err := json.Unmarshal(state, &probe); err != nil {
		return 0, fmt.Errorf("decode llmwiki resource state: %w", err)
	}
	version, err := probe.ResourceVersion.Int64()
	if err != nil {
		return 0, fmt.Errorf("llmwiki resource state missing resource_version")
	}
	return version, nil
}

func (c *LLMWikiSvcHttpClientImpl) CreateKnowledgeBase(ctx context.Context, req *LLMWikiCreateKnowledgeBaseRequest) (*LLMWikiCreateKnowledgeBaseResponse, error) {
	if req == nil || !types.AgentKnowledgeBaseIDPattern.MatchString(req.ContentID) {
		return nil, errorx.BadRequest(errors.New("invalid llmwiki create knowledge base request"), nil)
	}
	body := struct {
		SchemaVersion  string `json:"schema_version"`
		KBID           string `json:"kb_id"`
		StorageProfile struct {
			ProfileID string `json:"profile_id"`
		} `json:"storage_profile"`
	}{SchemaVersion: llmWikiCreateSchemaVersion, KBID: req.ContentID}
	body.StorageProfile.ProfileID = llmWikiStorageProfileID

	var state LLMWikiKnowledgeBaseResourceState
	headers := map[string]string{"Idempotency-Key": llmWikiIdempotencyKey("create", req.ContentID)}
	if err := c.doJSONEx(ctx, http.MethodPost, "/internal/v1/knowledge-bases", body, req.ActorID, "create knowledge base", headers, []int{http.StatusCreated}, &state); err != nil {
		return nil, err
	}

	resp := &LLMWikiCreateKnowledgeBaseResponse{
		ContentID:     req.ContentID,
		Name:          req.Name,
		Description:   req.Description,
		ResourceState: &state,
	}
	if endpoint := llmWikiMCPEndpointFromResourceState(state); endpoint != "" {
		resp.Metadata = map[string]any{types.AgentKnowledgeBaseMetadataMCPEndpointURLKey: endpoint}
	}
	return resp, nil
}

func (c *LLMWikiSvcHttpClientImpl) UpdateKnowledgeBase(_ context.Context, _ *LLMWikiUpdateKnowledgeBaseRequest) error {
	return nil
}

func (c *LLMWikiSvcHttpClientImpl) DeleteKnowledgeBase(ctx context.Context, req *LLMWikiDeleteKnowledgeBaseRequest) error {
	if req == nil || !types.AgentKnowledgeBaseIDPattern.MatchString(req.ContentID) {
		return errorx.BadRequest(errors.New("invalid llmwiki delete knowledge base request"), nil)
	}

	kbPath := "/internal/v1/knowledge-bases/" + url.PathEscape(req.ContentID)
	var state LLMWikiKnowledgeBaseResourceState
	err := c.doJSONEx(ctx, http.MethodGet, kbPath+"/resource-state", nil, req.ActorID, "get knowledge base resource state", nil, []int{http.StatusOK}, &state)
	if err != nil {
		var statusErr *llmWikiStatusError
		if errors.As(err, &statusErr) && statusErr.status == http.StatusNotFound {
			return nil
		}
		return err
	}
	resourceVersion, err := llmWikiResourceVersionFromState(state)
	if err != nil {
		return errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "llmwiki service").Set("action", "delete knowledge base"))
	}

	body := struct {
		SchemaVersion   string `json:"schema_version"`
		KBID            string `json:"kb_id"`
		ResourceVersion int64  `json:"resource_version"`
		Reason          string `json:"reason"`
	}{SchemaVersion: llmWikiDeleteSchemaVersion, KBID: req.ContentID, ResourceVersion: resourceVersion, Reason: llmWikiDeleteReason}
	headers := map[string]string{"Idempotency-Key": llmWikiIdempotencyKey("delete", req.ContentID)}
	err = c.doJSONEx(ctx, http.MethodDelete, kbPath, body, req.ActorID, "delete knowledge base", headers, []int{http.StatusOK}, nil)
	if err != nil {
		var statusErr *llmWikiStatusError
		if errors.As(err, &statusErr) && statusErr.status == http.StatusConflict && statusErr.code == llmWikiErrorCodeKBDeleted {
			return nil
		}
		return err
	}
	return nil
}

func (c *LLMWikiSvcHttpClientImpl) GetKnowledgeBaseResourceState(ctx context.Context, req *LLMWikiGetKnowledgeBaseResourceStateRequest) (*LLMWikiKnowledgeBaseResourceState, error) {
	if req == nil || !types.AgentKnowledgeBaseIDPattern.MatchString(req.ContentID) {
		return nil, errorx.BadRequest(errors.New("invalid llmwiki knowledge base resource state request"), nil)
	}

	path := "/internal/v1/knowledge-bases/" + url.PathEscape(req.ContentID) + "/resource-state"
	var state LLMWikiKnowledgeBaseResourceState
	if err := c.doJSON(ctx, http.MethodGet, path, nil, req.ActorID, "get knowledge base resource state", &state); err != nil {
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
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/knowledge-bases:batchGetResourceStatus", body, req.ActorID, "batch get knowledge base resource states", &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *LLMWikiSvcHttpClientImpl) doJSON(ctx context.Context, method, path string, body any, actorID, action string, out any) error {
	return c.doJSONEx(ctx, method, path, body, actorID, action, nil, []int{http.StatusOK}, out)
}

func (c *LLMWikiSvcHttpClientImpl) doJSONEx(ctx context.Context, method, path string, body any, actorID, action string, headers map[string]string, expected []int, out any) error {
	errCtx := errorx.Ctx().Set("service", "llmwiki service").Set("action", action)
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
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for _, opt := range c.hc.authOpts {
		opt.Set(req)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return errorx.RemoteSvcFail(err, errCtx)
	}
	defer resp.Body.Close()
	if !slices.Contains(expected, resp.StatusCode) {
		respBody, _ := readLimitedResponseBody(resp.Body, llmWikiErrorBodyLimit)
		statusErr := &llmWikiStatusError{status: resp.StatusCode}
		var errBody struct {
			Code string `json:"code"`
		}
		if json.Unmarshal([]byte(respBody), &errBody) == nil {
			statusErr.code = errBody.Code
		}
		// The upstream body can contain internal addresses; keep it in logs only.
		slog.WarnContext(ctx, "llmwiki service request failed", "action", action, "http_status", resp.StatusCode, "upstream_code", statusErr.code, "response_body", respBody)
		return errorx.RemoteSvcFail(statusErr, errCtx.Set("http_status", resp.StatusCode))
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		// A success status with an empty body leaves out at its zero value.
		if errors.Is(err, io.EOF) {
			return nil
		}
		return errorx.RemoteSvcFail(fmt.Errorf("decode llmwiki response: %w", err), errCtx)
	}
	return nil
}
