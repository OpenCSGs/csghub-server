package imagerunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	u := fmt.Sprintf("%s/%s/%s/run", h.remote, req.OrgName, req.SpaceName)
	response, err := h.doRequest(http.MethodPost, u, req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var buildResponse RunResponse
	if err := json.NewDecoder(response.Body).Decode(&buildResponse); err != nil {
		return nil, err
	}

	return &buildResponse, nil
}

func (h *RemoteRunner) Stop(ctx context.Context, req *StopRequest) (*StopResponse, error) {
	u := fmt.Sprintf("%s/%s/%s/stop", h.remote, req.OrgName, req.SpaceName)
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
	u := fmt.Sprintf("%s/%s/%s/status", h.remote, req.OrgName, req.SpaceName)
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

func (h *RemoteRunner) StatusAll(ctx context.Context) (map[string]int, error) {
	u := fmt.Sprintf("%s/status-all", h.remote)
	response, err := h.doRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	statusAll := make(map[string]int)
	if err := json.NewDecoder(response.Body).Decode(&statusAll); err != nil {
		return nil, err
	}

	return statusAll, nil
}

func (h *RemoteRunner) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	u := fmt.Sprintf("%s/%s/%s/logs", h.remote, req.OrgName, req.SpaceName)
	rc, err := h.doSSERequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return &LogsResponse{
		SSEReadCloser: rc,
	}, nil
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

func (h *RemoteRunner) doSSERequest(method, url string, data interface{}) (io.ReadCloser, error) {
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
