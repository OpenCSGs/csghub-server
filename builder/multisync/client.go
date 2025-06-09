package multisync

import (
	"context"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/types"
)

type Client interface {
	Latest(ctx context.Context, currentVersion int64) (types.SyncVersionResponse, error)
	ModelInfo(ctx context.Context, v types.SyncVersion) (*types.Model, error)
	DatasetInfo(ctx context.Context, v types.SyncVersion) (*types.Dataset, error)
	CodeInfo(ctx context.Context, v types.SyncVersion) (*types.Code, error)
	PromptInfo(ctx context.Context, v types.SyncVersion) (*types.PromptRes, error)
	MCPServerInfo(ctx context.Context, v types.SyncVersion) (*types.MCPServer, error)
	ReadMeData(ctx context.Context, v types.SyncVersion) (string, error)
	FileList(ctx context.Context, v types.SyncVersion) ([]types.File, error)
	Diff(ctx context.Context, req types.RemoteDiffReq) ([]types.RemoteDiffs, error)
}

func FromOpenCSG(endpoint string, accessToken string) Client {
	if endpoint == "" {
		return &commonClient{}
	}
	return &commonClient{rpcClent: rpc.NewHttpClient(endpoint, rpc.AuthWithApiKey(accessToken))}
}

type commonClient struct {
	rpcClent *rpc.HttpClient
}

func (c *commonClient) Latest(ctx context.Context, currentVersion int64) (types.SyncVersionResponse, error) {
	path := fmt.Sprintf("/api/v1/sync/version/latest?cur=%d", currentVersion)

	var svc types.SyncVersionResponse
	err := c.rpcClent.Get(ctx, path, &svc)
	if err != nil {
		return svc, fmt.Errorf("failed to get latest version, cause: %w", err)
	}
	return svc, nil
}

func (c *commonClient) ModelInfo(ctx context.Context, v types.SyncVersion) (*types.Model, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/models/%s/%s?need_multi_sync=true", namespace, name)
	var res types.ModelResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) DatasetInfo(ctx context.Context, v types.SyncVersion) (*types.Dataset, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/datasets/%s/%s?need_multi_sync=true", namespace, name)
	var res types.DatasetResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) CodeInfo(ctx context.Context, v types.SyncVersion) (*types.Code, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/codes/%s/%s?need_multi_sync=true", namespace, name)
	var res types.CodeResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get codes info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) PromptInfo(ctx context.Context, v types.SyncVersion) (*types.PromptRes, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/prompts_info/%s/%s?need_multi_sync=true", namespace, name)
	var res types.PromptResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) MCPServerInfo(ctx context.Context, v types.SyncVersion) (*types.MCPServer, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/mcps/%s/%s?need_multi_sync=true", namespace, name)
	var res types.MCPServerResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get mcpserver info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) ReadMeData(ctx context.Context, v types.SyncVersion) (string, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/%ss/%s/%s/raw/README.md", v.RepoType, namespace, name)
	var res types.ReadMeResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return "", fmt.Errorf("failed to get dataset info, cause: %w", err)
	}
	return res.Data, nil
}

func (c *commonClient) FileList(ctx context.Context, v types.SyncVersion) ([]types.File, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/%ss/%s/%s/all_files", v.RepoType, namespace, name)
	var res types.AllFilesResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get file list, cause: %w", err)
	}
	return res.Data, nil
}

func (c *commonClient) Diff(ctx context.Context, req types.RemoteDiffReq) ([]types.RemoteDiffs, error) {
	url := fmt.Sprintf(
		"/api/v1/%ss/%s/%s/diff?left_commit_id=%s",
		req.RepoType,
		req.Namespace,
		req.Name,
		req.LeftCommitID)
	var res types.RemoteDiffRespones
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff list, cause: %w", err)
	}
	return res.Data, nil
}
