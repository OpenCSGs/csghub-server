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

	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var _ Runner = (*RemoteRunner)(nil)

type RemoteRunner struct {
	remote       *url.URL
	client       rpc.HttpDoer
	clusterStore database.ClusterInfoStore
	config       common.DeployConfig
}

func NewRemoteRunner(remoteURL string, c common.DeployConfig) (Runner, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	// init cluster store
	clusterStore := database.NewClusterInfoStore()

	return &RemoteRunner{
		remote:       parsedURL,
		client:       rpc.NewHttpClient("").WithRetry(2).WithDelay(time.Second * 1),
		config:       c,
		clusterStore: clusterStore,
	}, nil
}

func (h *RemoteRunner) Run(ctx context.Context, req *types.RunRequest) (*types.RunResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	slog.Debug("send request", slog.Any("body", req))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/run", remote, svcName)
	response, err := h.doRequest(ctx, http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp types.RunResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
	}
	resp.Message = svcName
	slog.Debug("call image run", slog.Any("response", resp))
	return &resp, nil
}

func (h *RemoteRunner) Stop(ctx context.Context, req *types.StopRequest) (*types.StopResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/stop", remote, svcName)
	response, err := h.doRequest(ctx, http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var StopResponse types.StopResponse
	if err := json.NewDecoder(response.Body).Decode(&StopResponse); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
	}

	return &StopResponse, nil
}

func (h *RemoteRunner) Purge(ctx context.Context, req *types.PurgeRequest) (*types.PurgeResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/purge", remote, svcName)
	response, err := h.doRequest(ctx, http.MethodDelete, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var purgeResponse types.PurgeResponse
	if err := json.NewDecoder(response.Body).Decode(&purgeResponse); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
	}

	return &purgeResponse, nil
}

func (h *RemoteRunner) Status(ctx context.Context, req *types.StatusRequest) (*types.StatusResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/status", remote, svcName)

	response, err := h.doRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse types.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) Logs(ctx context.Context, req *types.LogsRequest) (<-chan string, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/logs", remote, svcName)
	slog.Debug("logs request", slog.String("url", u), slog.String("appname", svcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}

func (h *RemoteRunner) Exist(ctx context.Context, req *types.CheckRequest) (*types.StatusResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/get", remote, svcName)
	response, err := h.doRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse types.StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) GetReplica(ctx context.Context, req *types.StatusRequest) (*types.ReplicaResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	svcName := req.SvcName
	u := fmt.Sprintf("%s/api/v1/service/%s/replica", remote, svcName)
	response, err := h.doRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp types.ReplicaResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, errorx.InternalServerError(err,
			errorx.Ctx().
				Set("svc_name", svcName),
		)
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
func (h *RemoteRunner) doRequest(ctx context.Context, method, url string, data interface{}) (*http.Response, error) {
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, errorx.InternalServerError(err, nil)
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.config.APIKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData interface{}
		err := json.NewDecoder(resp.Body).Decode(&errData)
		if err != nil {
			err := fmt.Errorf("unexpected http status: %d, error: %w", resp.StatusCode, err)
			return nil, errorx.RemoteSvcFail(err, nil)
		} else {
			err := fmt.Errorf("unexpected http status: %d, error: %v", resp.StatusCode, errData)
			return nil, errorx.RemoteSvcFail(err, nil)
		}
	}

	return resp, nil
}

func (h *RemoteRunner) doSteamRequest(ctx context.Context, method, url string, data interface{}) (io.ReadCloser, error) {
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, errorx.InternalServerError(err, nil)
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Authorization", "Bearer "+h.config.APIKey)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, errorx.RemoteSvcFail(err, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("unexpected http status code:%d", resp.StatusCode)
		return nil, errorx.RemoteSvcFail(err, nil)
	}

	return resp.Body, nil
}

// InstanceLogs implements Runner.
func (h *RemoteRunner) InstanceLogs(ctx context.Context, req *types.InstanceLogsRequest) (<-chan string, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/api/v1/service/%s/logs/%s", remote, req.SvcName, req.InstanceName)
	slog.Info("Instance logs request", slog.String("url", u), slog.String("svcname", req.SvcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}

func (h *RemoteRunner) ListCluster(ctx context.Context) ([]types.ClusterResponse, error) {
	clusters, err := h.clusterStore.List(ctx)
	if err != nil {
		slog.Error("failed to list clusters from db", slog.Any("error", err))
		return nil, err
	}
	var resp []types.ClusterResponse
	for _, cluster := range clusters {
		if !cluster.Enable {
			continue
		}
		resp = append(resp, types.ClusterResponse{
			ClusterID: cluster.ClusterID,
			Region:    cluster.Region,
			Zone:      cluster.Zone,
			Provider:  cluster.Provider,
			UpdatedAt: cluster.UpdatedAt,
		})
	}
	return resp, nil
}

func (h *RemoteRunner) GetClusterById(ctx context.Context, clusterId string) (*types.ClusterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	remote, err := h.GetRemoteRunnerHost(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/cluster/%s", remote, clusterId)
	// Send a GET request to resources runner
	response, err := h.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var resp types.ClusterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	return &resp, nil
}

func (h *RemoteRunner) UpdateCluster(ctx context.Context, data *types.ClusterRequest) (*types.UpdateClusterResponse, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, data.ClusterID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/cluster/%s", remote, data.ClusterID)
	// Create a new HTTP client with a timeout
	response, err := h.doRequest(ctx, http.MethodPut, url, data)
	if err != nil {
		fmt.Printf("Error sending request to k8s cluster: %s\n", err)
		return nil, fmt.Errorf("failed to update cluster info, %w", err)
	}
	defer response.Body.Close()
	var resp types.UpdateClusterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}

	return &resp, nil
}

// submit argo workflow
func (h *RemoteRunner) SubmitWorkFlow(ctx context.Context, req *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/workflows", remote)
	// Create a new HTTP client with a timeout
	response, err := h.doRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit evaluation job, %w", err)
	}
	defer response.Body.Close()

	var res types.ArgoWorkFlowRes
	if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	return &res, nil
}

// delete workflow
func (h *RemoteRunner) DeleteWorkFlow(ctx context.Context, req types.ArgoWorkFlowDeleteReq) (*httpbase.R, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/workflows/%d", remote, req.ID)
	// Create a new HTTP client with a timeout
	response, err := h.doRequest(ctx, http.MethodDelete, url, req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete evaluation job, %w", err)
	}
	defer response.Body.Close()
	var res httpbase.R
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	return &res, nil
}

func (h *RemoteRunner) GetWorkFlow(ctx context.Context, req types.ArgoWorkFlowDeleteReq) (*types.ArgoWorkFlowRes, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/workflows/%d", remote, req.ID)
	response, err := h.doRequest(ctx, http.MethodGet, url, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var res types.ArgoWorkFlowRes
	if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	return &res, nil
}

// get remote runner host
func (h *RemoteRunner) GetRemoteRunnerHost(ctx context.Context, clusterID string) (string, error) {
	if clusterID == "" {
		// use default remote
		return h.remote.Scheme + "://" + h.remote.Host, nil
	}
	cluster, err := h.clusterStore.ByClusterID(ctx, clusterID)
	if err != nil {
		slog.Error("failed to get cluster info from db in image runner", slog.String("cluster_id", clusterID), slog.Any("error", err))
		return "", err
	}
	if cluster.Mode == types.ConnectModeInCluster {
		return cluster.RunnerEndpoint, nil
	}
	return h.remote.Scheme + "://" + h.remote.Host, nil
}

func (h *RemoteRunner) SubmitFinetuneJob(ctx context.Context, req *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error) {
	remote, err := h.GetRemoteRunnerHost(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/api/v1/workflows", remote)
	// Create a new HTTP client with a timeout
	response, err := h.doRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit finetune job from deployer, %w", err)
	}
	defer response.Body.Close()

	var res types.ArgoWorkFlowRes
	if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
		return nil, errorx.InternalServerError(err, nil)
	}
	return &res, nil
}
