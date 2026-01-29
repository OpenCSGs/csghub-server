package memmachine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type Client struct {
	hc       *rpc.HttpClient
	endpoint string
	basePath string
	authOpts []rpc.RequestOption
	logger   *slog.Logger
}

func NewClient(endpoint, basePath string, opts ...rpc.RequestOption) *Client {
	client := &Client{
		hc:       rpc.NewHttpClient(endpoint, opts...),
		endpoint: endpoint,
		basePath: normalizeBasePath(basePath),
		authOpts: opts,
		logger:   slog.Default(),
	}
	client.ensureDefaultProject()
	return client
}

func (c *Client) SetTimeout(timeout time.Duration) {
	c.hc.SetTimeout(timeout)
}

func (c *Client) WithRetry(attempts uint) {
	c.hc.WithRetry(attempts)
}

func (c *Client) WithDelay(delay time.Duration) {
	c.hc.WithDelay(delay)
}

func normalizeBasePath(basePath string) string {
	if strings.TrimSpace(basePath) == "" {
		return "/api/v2"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return strings.TrimRight(basePath, "/")
}

func (c *Client) ensureDefaultProject() {
	req := &types.GetMemoryProjectRequest{OrgID: "_global", ProjectID: "_public"}
	status, _, err := c.postWithStatus(context.Background(), "/projects/get", req)
	if err != nil {
		c.logger.Error("memory default project check failed", "error", err)
		return
	}
	if status != http.StatusNotFound {
		return
	}
	createReq := &types.CreateMemoryProjectRequest{OrgID: "_global", ProjectID: "_public"}
	if _, err := c.CreateProject(context.Background(), createReq); err != nil {
		c.logger.Error("memory default project creation failed", "error", err)
	}
}

func (c *Client) CreateProject(ctx context.Context, req *types.CreateMemoryProjectRequest) (*types.MemoryProjectResponse, error) {
	var resp types.MemoryProjectResponse
	err := c.post(ctx, "/projects", req, &resp, http.StatusCreated)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "create project"))
	}
	return &resp, nil
}

func (c *Client) GetProject(ctx context.Context, req *types.GetMemoryProjectRequest) (*types.MemoryProjectResponse, error) {
	var resp types.MemoryProjectResponse
	err := c.post(ctx, "/projects/get", req, &resp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "get project"))
	}
	return &resp, nil
}

func (c *Client) ListProjects(ctx context.Context) ([]*types.MemoryProjectRef, error) {
	var resp []*types.MemoryProjectRef
	err := c.post(ctx, "/projects/list", map[string]any{}, &resp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "list projects"))
	}
	return resp, nil
}

func (c *Client) DeleteProject(ctx context.Context, req *types.DeleteMemoryProjectRequest) error {
	err := c.post(ctx, "/projects/delete", req, nil, http.StatusOK, http.StatusNoContent)
	if err != nil {
		return errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "delete project"))
	}
	return nil
}

func (c *Client) AddMemories(ctx context.Context, req *types.AddMemoriesRequest) (*types.AddMemoriesResponse, error) {
	var addResp struct {
		Results []types.MemoryAddResult `json:"results"`
	}
	err := c.post(ctx, "/memories", mapAddRequest(req), &addResp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "add memories"))
	}
	if len(addResp.Results) == 0 {
		return &types.AddMemoriesResponse{Created: []types.MemoryMessage{}}, nil
	}

	uids := make([]string, 0, len(addResp.Results))
	for _, result := range addResp.Results {
		if result.UID != "" {
			uids = append(uids, result.UID)
		}
	}
	if len(uids) == 0 {
		return &types.AddMemoriesResponse{Created: []types.MemoryMessage{}}, nil
	}

	orgID, projectID := resolveOrgProject(req)
	filter := fmt.Sprintf("uid in (%s)", joinQuoted(uids))
	listReq := &memmachineListByUIDRequest{
		OrgID:     orgID,
		ProjectID: projectID,
		Type:      string(types.MemoryTypeEpisodic),
		Filter:    filter,
	}

	var rawResp memmachineSearchResponse
	err = c.post(ctx, "/memories/list", listReq, &rawResp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "list memories"))
	}
	created := mapMemmachineContent(rawResp.Content, buildScopeRequest(req))
	return &types.AddMemoriesResponse{Created: created}, nil
}

func (c *Client) SearchMemories(ctx context.Context, req *types.SearchMemoriesRequest) (*types.SearchMemoriesResponse, error) {
	mappedReq := mapSearchRequest(req)
	if mappedReq != nil && mappedReq.PageNum > 0 {
		copied := *mappedReq
		copied.PageNum--
		mappedReq = &copied
	}
	var rawResp memmachineSearchResponse
	err := c.post(ctx, "/memories/search", mappedReq, &rawResp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "search memories"))
	}
	resp := mapSearchResponse(rawResp, req)
	return resp, nil
}

func (c *Client) ListMemories(ctx context.Context, req *types.ListMemoriesRequest) (*types.ListMemoriesResponse, error) {
	mappedReq := mapListRequest(req)
	if mappedReq != nil && mappedReq.PageNum > 0 {
		// MemMachine expects zero-based page_num; csghub API uses one-based paging.
		copied := *mappedReq
		copied.PageNum--
		mappedReq = &copied
	}
	var rawResp memmachineSearchResponse
	err := c.post(ctx, "/memories/list", mappedReq, &rawResp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "list memories"))
	}
	resp := mapListResponse(rawResp, req)
	return resp, nil
}

func (c *Client) DeleteMemories(ctx context.Context, req *types.DeleteMemoriesRequest) error {
	if req == nil {
		return nil
	}
	episodicIDs, semanticIDs := splitDeleteUIDs(req)
	if len(episodicIDs) == 0 && len(semanticIDs) == 0 {
		return nil
	}
	orgID, projectID := resolveQueryOrgProject(req.OrgID, req.ProjectID)
	if len(episodicIDs) > 0 {
		payload := memmachineDeleteEpisodicRequest{
			OrgID:       orgID,
			ProjectID:   projectID,
			EpisodicIDs: episodicIDs,
		}
		err := c.post(ctx, "/memories/episodic/delete", payload, nil, http.StatusOK, http.StatusNoContent)
		if err != nil {
			return errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "delete episodic memories"))
		}
	}
	if len(semanticIDs) > 0 {
		payload := memmachineDeleteSemanticRequest{
			OrgID:       orgID,
			ProjectID:   projectID,
			SemanticIDs: semanticIDs,
		}
		err := c.post(ctx, "/memories/semantic/delete", payload, nil, http.StatusOK, http.StatusNoContent)
		if err != nil {
			return errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "delete semantic memories"))
		}
	}
	return nil
}

func (c *Client) Health(ctx context.Context) (*types.MemoryHealthResponse, error) {
	var resp types.MemoryHealthResponse
	err := c.get(ctx, "/health", &resp, http.StatusOK)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, errorx.Ctx().Set("service", "memory service").Set("action", "health check"))
	}
	return &resp, nil
}

func (c *Client) post(ctx context.Context, path string, req any, out any, okCodes ...int) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.do(ctx, http.MethodPost, path, bytes.NewReader(body), out, okCodes...)
}

func (c *Client) postWithStatus(ctx context.Context, path string, req any) (int, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.doRaw(ctx, http.MethodPost, path, bytes.NewReader(body))
}

func (c *Client) get(ctx context.Context, path string, out any, okCodes ...int) error {
	return c.do(ctx, http.MethodGet, path, nil, out, okCodes...)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any, okCodes ...int) error {
	fullPath := fmt.Sprintf("%s%s%s", c.endpoint, c.basePath, path)
	req, err := http.NewRequestWithContext(ctx, method, fullPath, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range c.authOpts {
		opt.Set(req)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusAllowed(resp.StatusCode, okCodes...) {
		respBody, readErr := io.ReadAll(resp.Body)
		bodyText := string(respBody)
		if len(bodyText) > 2048 {
			bodyText = bodyText[:2048] + "...(truncated)"
		}
		c.logger.ErrorContext(ctx, "memory service request failed",
			"method", method,
			"url", fullPath,
			"status", resp.StatusCode,
			"body", bodyText,
		)
		if readErr != nil {
			return fmt.Errorf("unexpected response status: %d (failed to read body: %w)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("unexpected response status: %d, body: %s", resp.StatusCode, bodyText)
	}

	if out == nil {
		return nil
	}
	if buf, ok := out.(*bytes.Buffer); ok {
		_, err = buf.ReadFrom(resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		c.logger.ErrorContext(ctx, "memory service response decode failed",
			"method", method,
			"url", fullPath,
			"error", err.Error(),
		)
		return err
	}
	return nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, body io.Reader) (int, []byte, error) {
	fullPath := fmt.Sprintf("%s%s%s", c.endpoint, c.basePath, path)
	req, err := http.NewRequestWithContext(ctx, method, fullPath, body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range c.authOpts {
		opt.Set(req)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response: %w", readErr)
	}
	return resp.StatusCode, respBody, nil
}

func statusAllowed(status int, okCodes ...int) bool {
	if len(okCodes) == 0 {
		return status == http.StatusOK
	}
	for _, code := range okCodes {
		if status == code {
			return true
		}
	}
	return false
}

func buildScopeRequest(req *types.AddMemoriesRequest) *types.SearchMemoriesRequest {
	if req == nil {
		return nil
	}
	orgID, projectID := resolveOrgProject(req)
	return &types.SearchMemoriesRequest{
		AgentID:   req.AgentID,
		OrgID:     orgID,
		ProjectID: projectID,
		SessionID: req.SessionID,
	}
}

func joinQuoted(values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%q", value))
	}
	return strings.Join(parts, ", ")
}

func splitDeleteUIDs(req *types.DeleteMemoriesRequest) ([]string, []string) {
	if req == nil {
		return nil, nil
	}
	var episodicIDs []string
	var semanticIDs []string
	all := make([]string, 0, len(req.UIDs)+1)
	if req.UID != "" {
		all = append(all, req.UID)
	}
	all = append(all, req.UIDs...)
	for _, uid := range all {
		if uid == "" {
			continue
		}
		switch {
		case strings.HasPrefix(uid, "e_"):
			episodicIDs = append(episodicIDs, strings.TrimPrefix(uid, "e_"))
		case strings.HasPrefix(uid, "s_"):
			semanticIDs = append(semanticIDs, strings.TrimPrefix(uid, "s_"))
		default:
			episodicIDs = append(episodicIDs, uid)
		}
	}
	return episodicIDs, semanticIDs
}
