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

func (h *RemoteRunner) Run(ctx context.Context, req *RunRequest) (*RunResponse, error) {
	slog.Debug("send request", slog.Any("body", req))
	// svcName := common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID)
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/run", h.remote, svcName)
	response, err := h.doRequest(http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp RunResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return nil, err
	}
	resp.Message = svcName
	slog.Debug("call image run", slog.Any("response", resp))
	return &resp, nil
}

func (h *RemoteRunner) Stop(ctx context.Context, req *StopRequest) (*StopResponse, error) {
	// u := fmt.Sprintf("%s/%s/stop", h.remote, common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/stop", h.remote, svcName)
	response, err := h.doRequest(http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var stopResponse StopResponse
	if err := json.NewDecoder(response.Body).Decode(&stopResponse); err != nil {
		return nil, err
	}

	return &stopResponse, nil
}

func (h *RemoteRunner) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	// u := fmt.Sprintf("%s/%s/status", h.remote, common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/status", h.remote, svcName)

	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, err
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) StatusAll(ctx context.Context) (map[string]StatusResponse, error) {
	u := fmt.Sprintf("%s/status-all", h.remote)
	response, err := h.doRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	statusAll := make(map[string]StatusResponse)
	if err := json.NewDecoder(response.Body).Decode(&statusAll); err != nil {
		return nil, err
	}

	return statusAll, nil
}

func (h *RemoteRunner) Logs(ctx context.Context, req *LogsRequest) (<-chan string, error) {
	// appName := common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID)
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/logs", h.remote, svcName)
	slog.Debug("logs request", slog.String("url", u), slog.String("appname", svcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}

func (h *RemoteRunner) Exist(ctx context.Context, req *CheckRequest) (*StatusResponse, error) {
	// u := fmt.Sprintf("%s/%s/get", h.remote, common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/get", h.remote, svcName)
	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var statusResponse StatusResponse
	if err := json.NewDecoder(response.Body).Decode(&statusResponse); err != nil {
		return nil, err
	}

	return &statusResponse, nil
}

func (h *RemoteRunner) GetReplica(ctx context.Context, req *StatusRequest) (*ReplicaResponse, error) {
	// u := fmt.Sprintf("%s/%s/replica", h.remote, common.UniqueSpaceAppName(req.OrgName, req.RepoName, req.ID))
	svcName := req.SvcName
	u := fmt.Sprintf("%s/%s/replica", h.remote, svcName)
	response, err := h.doRequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var resp ReplicaResponse
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
		return nil, fmt.Errorf("unexpected http status code:%d", resp.StatusCode)
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
	// req.Header.Set("Accept", "text/event-stream")
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
func (h *RemoteRunner) InstanceLogs(ctx context.Context, req *InstanceLogsRequest) (<-chan string, error) {
	u := fmt.Sprintf("%s/%s/logs/%s", h.remote, req.SvcName, req.InstanceName)
	slog.Info("Instance logs request", slog.String("url", u), slog.String("svcname", req.SvcName))
	rc, err := h.doSteamRequest(ctx, http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return h.readToChannel(rc), nil
}
