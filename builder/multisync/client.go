package multisync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	return &commonClient{
		endpoint:  endpoint,
		hc:        http.DefaultClient,
		authToken: accessToken,
	}
}

type commonClient struct {
	endpoint  string
	hc        *http.Client
	authToken string
}

func (c *commonClient) Latest(ctx context.Context, currentVersion int64) (types.SyncVersionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/sync/version/latest?cur=%d", c.endpoint, currentVersion)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return types.SyncVersionResponse{}, fmt.Errorf("failed to get latest version from endpoint %s, param cur:%d, cause: %w",
			c.endpoint, currentVersion, err)
	}

	if resp.StatusCode != http.StatusOK {
		var data bytes.Buffer
		data.ReadFrom(resp.Body)
		return types.SyncVersionResponse{}, fmt.Errorf("failed to get latest version from endpoint %s, param cur:%d, status code: %d, body: %s",
			c.endpoint, currentVersion, resp.StatusCode, data.String())
	}
	var svc types.SyncVersionResponse
	err = json.NewDecoder(resp.Body).Decode(&svc)
	if err != nil {
		return types.SyncVersionResponse{}, fmt.Errorf("failed to decode response body as types.SyncVersionResponse, cause: %w", err)
	}
	return svc, nil
}

func (c *commonClient) ModelInfo(ctx context.Context, v types.SyncVersion) (*types.Model, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("%s/api/v1/models/%s/%s", c.endpoint, namespace, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info from endpoint %s, repo path:%s, cause: %w",
			c.endpoint, v.RepoPath, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get model info from endpoint %s, repo path:%s, status code: %d",
			c.endpoint, v.RepoPath, resp.StatusCode)
	}
	var res types.ModelResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body as types.Model, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) DatasetInfo(ctx context.Context, v types.SyncVersion) (*types.Dataset, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("%s/api/v1/datasets/%s/%s", c.endpoint, namespace, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset info from endpoint %s, repo path:%s, cause: %w",
			c.endpoint, v.RepoPath, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get dataset info from endpoint %s, repo path:%s, status code: %d",
			c.endpoint, v.RepoPath, resp.StatusCode)
	}
	var res types.DatasetResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body as types.Dataset, cause: %w", err)
	}
	return &res.Data, nil
}

func (c *commonClient) ReadMeData(ctx context.Context, v types.SyncVersion) (string, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("%s/api/v1/%ss/%s/%s/raw/README.md", c.endpoint, v.RepoType, namespace, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get readme data endpoint %s, repo path:%s, cause: %w",
			c.endpoint, v.RepoPath, err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to get readme data from endpoint %s, repo path:%s, status code: %d",
			c.endpoint, v.RepoPath, resp.StatusCode)
	}
	var res types.ReadMeResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return "", fmt.Errorf("failed to decode response body as types.Dataset, cause: %w", err)
	}
	return res.Data, nil
}

func (c *commonClient) FileList(ctx context.Context, v types.SyncVersion) ([]types.File, error) {
	namespace, name, _ := strings.Cut(v.RepoPath, "/")
	url := fmt.Sprintf("%s/api/v1/%ss/%s/%s/all_files", c.endpoint, v.RepoType, namespace, name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("Authorization", "Bearer "+c.authToken)
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get readme data endpoint %s, repo path:%s, cause: %w",
			c.endpoint, v.RepoPath, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get readme data from endpoint %s, repo path:%s, status code: %d",
			c.endpoint, v.RepoPath, resp.StatusCode)
	}
	var res types.AllFilesResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response body as types.Dataset, cause: %w", err)
	}
	return res.Data, nil
}
