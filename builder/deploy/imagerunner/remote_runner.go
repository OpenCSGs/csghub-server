package imagerunner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"opencsg.com/csghub-server/common/types"
)

var _ Runner = (*RemoteRunner)(nil)

type RemoteRunner struct {
	remote *url.URL
	client *http.Client
}

func NewRemoteRunner(remoteURL string) (Runner, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	return &RemoteRunner{
		remote: parsedURL,
		client: http.DefaultClient,
	}, nil
}

func (h *RemoteRunner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	slog.Debug("send request", slog.Any("body", req))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/run", h.remote, svcName)
	response, err := h.doRequest(http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp types.RunResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}
	resp.Message = svcName
	slog.Debug("call image run", slog.Any("response", resp))
	return &resp, nil
}

func (h *RemoteRunner) Stop(ctx context.Context, req *types.StopRequest) (*types.StopResponse, error) {
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/stop", h.remote, svcName)
	response, err := h.doRequest(http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var StopResponse types.StopResponse
	if err := json.NewDecoder(response.Body).Decode(&StopResponse); err != nil {
		return nil, err
	}

	return &StopResponse, nil
}

func (h *RemoteRunner) Purge(ctx context.Context, req *types.PurgeRequest) (*types.PurgeResponse, error) {
	// u := fmt.Sprintf("%s/%s/stop", h.remote, common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/purge", h.remote, svcName)
	response, err := h.doRequest(http.MethodDelete, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var purgeResponse types.PurgeResponse
	if err := json.NewDecoder(response.Body).Decode(&purgeResponse); err != nil {
		return nil, err
	}

	return &purgeResponse, nil
}

func (h *RemoteRunner) Status(ctx context.Context, req *types.StatusRequest) (*types.StatusResponse, error) {
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/status", h.remote, svcName)

	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse types.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, err
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) StatusAll(ctx context.Context) (map[string]types.StatusResponse, error) {
	u := fmt.Sprintf("%s/api/v1/service/status-all", h.remote)
	response, err := h.doRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	statusAll := make(map[string]types.StatusResponse)
	if err := json.NewDecoder(response.Body).Decode(&statusAll); err != nil {
		return nil, err
	}

	return statusAll, nil
}

func (h *RemoteRunner) Logs(ctx context.Context, req *types.LogsRequest) (<-chan string, error) {
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/logs", h.remote, svcName)
	slog.Debug("logs request", slog.String("url", u), slog.String("appname", svcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}

func (h *RemoteRunner) Exist(ctx context.Context, req *types.CheckRequest) (*types.StatusResponse, error) {
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/get", h.remote, svcName)
	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse types.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, err
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) GetReplica(ctx context.Context, req *types.StatusRequest) (*types.ReplicaResponse, error) {
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/replica", h.remote, svcName)
	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp types.ReplicaResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (h *RemoteRunner) readToChannel(rc io.ReadCloser) <-chan string {
	output := make(chan string, 2)

	buf := make([]byte, 256)
	br := bufio.NewReader(rc)

	go func() {
		for {
			n, err := br.Read(buf)
			if err != nil {
				slog.Error("remot runner log reader aborted", slog.Any("error", err))
				rc.Close()
				close(output)
				break
			}

			if n > 0 {
				output <- string(buf[:n])
			} else {
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return output
}

// Helper method to execute the actual HTTP request and read the response.
func (h *RemoteRunner) doRequest(method, url string, data interface{}) (*http.Response, error) {
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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

func (h *RemoteRunner) doSteamRequest(ctx context.Context, method, url string, data interface{}) (io.ReadCloser, error) {
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
	req.Header.Set("Connection", "keep-alive")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected http status code:%d", resp.StatusCode)
	}

	return resp.Body, nil
}

// InstanceLogs implements Runner.
func (h *RemoteRunner) InstanceLogs(ctx context.Context, req *types.InstanceLogsRequest) (<-chan string, error) {
	u := fmt.Sprintf("%s/api/v1/service/%s/logs/%s", h.remote, req.SvcName, req.InstanceName)
	slog.Info("Instance logs request", slog.String("url", u), slog.String("svcname", req.SvcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}

func (h *RemoteRunner) ListCluster(ctx context.Context) ([]types.ClusterResponse, error) {
	url := fmt.Sprintf("%s/api/v1/cluster", h.remote)
	// Send a GET request to resources runner
	response, err := h.client.Get(url)
	if err != nil {
		fmt.Printf("Error sending request to resoures runner: %s\n", err)
		return nil, fmt.Errorf("failed to list cluster info, %w", err)
	}
	defer response.Body.Close()
	var resp []types.ClusterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *RemoteRunner) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterResponse, error) {
	url := fmt.Sprintf("%s/api/v1/cluster/%s", h.remote, clusterId)
	// Send a GET request to resources runner
	response, err := h.doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var resp types.ClusterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (h *RemoteRunner) UpdateCluster(ctx context.Context, data *types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	url := fmt.Sprintf("%s/api/v1/cluster/%s", h.remote, data.ClusterID)
	// Create a new HTTP client with a timeout
	response, err := h.doRequest(http.MethodPut, url, data)
	if err != nil {
		fmt.Printf("Error sending request to k8s cluster: %s\n", err)
		return nil, fmt.Errorf("failed to update cluster info, %w", err)
	}
	defer response.Body.Close()
	var resp types.UpdateClusterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
