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
	ReadMeData(ctx context.Context, v types.SyncVersion) (string, error)
	FileList(ctx context.Context, v types.SyncVersion) ([]types.File, error)
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
	var svc types.SyncVersionResponse
	path := fmt.Sprintf("/api/v1/sync/version/latest?cur=%d", currentVersion)

	err := c.rpcClent.Get(ctx, path, &svc)
	if err != nil {
		return svc, fmt.Errorf("failed to get latest version, cause: %w", err)
	}
	return svc, nil
}

func (c *commonClient) ModelInfo(ctx context.Context, v types.SyncVersion) (*types.Model, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/models/%s/%s", namespace, name)
	var res types.ModelResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) DatasetInfo(ctx context.Context, v types.SyncVersion) (*types.Dataset, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("/api/v1/datasets/%s/%s", namespace, name)
	var res types.DatasetResponse
	err := c.rpcClent.Get(ctx, url, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset info, cause: %w", err)
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
