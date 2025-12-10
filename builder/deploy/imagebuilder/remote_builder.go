package imagebuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var _ Builder = (*RemoteBuilder)(nil)

type RemoteBuilder struct {
	remote       *url.URL
	client       rpc.HttpDoer
	config       common.DeployConfig
	clusterStore database.ClusterInfoStore
}

func NewRemoteBuilder(remoteURL string, c common.DeployConfig) (*RemoteBuilder, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	clusterStore := database.NewClusterInfoStore()
	return &RemoteBuilder{
		remote: parsedURL,
		//client:       http.DefaultClient,
		client:       rpc.NewHttpClient(""),
		config:       c,
		clusterStore: clusterStore,
	}, nil
}

func (h *RemoteBuilder) Build(ctx context.Context, req *types.ImageBuilderRequest) error {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/api/v1/imagebuilder/builder", remote)
	response, err := h.doRequest(ctx, http.MethodPost, u, req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(fmt.Errorf("failed to build image, status code: %d", response.StatusCode), errorx.Ctx().Set("cluster_id", req.ClusterID).Set("code", response.StatusCode))
	}

	return nil
}

func (h *RemoteBuilder) Stop(ctx context.Context, req types.ImageBuildStopReq) error {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/api/v1/imagebuilder/stop", remote)
	response, err := h.doRequest(ctx, http.MethodPut, u, req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errorx.RemoteSvcFail(fmt.Errorf("failed to stop image, status code: %d", response.StatusCode), errorx.Ctx().Set("cluster_id", req.ClusterID).Set("code", response.StatusCode))
	}

	return nil
}

// Helper method to execute the actual HTTP request and read the response.
func (h *RemoteBuilder) doRequest(ctx context.Context, method, url string, data interface{}) (*http.Response, error) {
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.config.APIKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData interface{}
		err := json.NewDecoder(resp.Body).Decode(&errData)
		if err != nil {
			return nil, fmt.Errorf("unexpected http status: %d, error: %w", resp.StatusCode, err)
		} else {
			return nil, fmt.Errorf("unexpected http status: %d, error: %v", resp.StatusCode, errData)
		}
	}

	return resp, nil
}

// get remote runner host
func (h *RemoteBuilder) GetRemoteRunnerHost(ctx context.Context, clusterID string) (string, error) {
	if clusterID == "" {
		// use default remote
		return h.remote.Scheme + "://" + h.remote.Host, nil
	}
	cluster, err := h.clusterStore.ByClusterID(ctx, clusterID)
	if err != nil {
		slog.Error("failed to get cluster info from db in image builder", slog.String("cluster_id", clusterID), slog.Any("error", err))
		return "", err
	}
	if cluster.Mode == types.ConnectModeInCluster {
		return cluster.RunnerEndpoint, nil
	}
	return h.remote.Scheme + "://" + h.remote.Host, nil
}
