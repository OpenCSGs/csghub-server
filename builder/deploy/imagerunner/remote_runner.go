package imagerunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	rel := &url.URL{Path: "/push_data"}
	u := h.remote.ResolveReference(rel)
	response, err := h.doRequest(http.MethodPost, u.String(), req)
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

func (h *RemoteRunner) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	rel := &url.URL{Path: fmt.Sprintf("/%s/%s/%s/status", req.OrgName, req.Name, req.Ref)}
	u := h.remote.ResolveReference(rel)
	response, err := h.doRequest(http.MethodGet, u.String(), req)
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

func (h *RemoteRunner) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	rel := &url.URL{Path: fmt.Sprintf("/%s/%s/%s/logs", req.OrgName, req.Name, req.Ref)}
	u := h.remote.ResolveReference(rel)
	response, err := h.doRequest(http.MethodGet, u.String(), req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var logsResponse LogsResponse
	if err := json.NewDecoder(response.Body).Decode(&logsResponse); err != nil {
		return nil, err
	}

	return &logsResponse, nil
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
		return nil, errors.New("unexpected http status code")
	}

	return resp, nil
}
