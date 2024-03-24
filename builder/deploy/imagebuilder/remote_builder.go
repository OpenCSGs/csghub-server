package imagebuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var _ Builder = (*RemoteBuilder)(nil)

type RemoteBuilder struct {
	remote *url.URL
	client *http.Client
}

func NewRemoteBuilder(remoteURL string) (*RemoteBuilder, error) {
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	return &RemoteBuilder{
		remote: parsedURL,
		client: http.DefaultClient,
	}, nil
}

func (h *RemoteBuilder) Build(ctx context.Context, req *BuildRequest) (*BuildResponse, error) {
	rel := &url.URL{Path: "/push_data"}
	u := h.remote.ResolveReference(rel)
	response, err := h.doRequest(http.MethodPost, u.String(), req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var buildResponse BuildResponse
	if err := json.NewDecoder(response.Body).Decode(&buildResponse); err != nil {
		return nil, err
	}

	return &buildResponse, nil
}

func (h *RemoteBuilder) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	u := fmt.Sprintf("/%s/%s/status?build_id=%d", req.OrgName, req.SpaceName, req.BuildID)
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

func (h *RemoteBuilder) Logs(ctx context.Context, req *LogsRequest) (*LogsResponse, error) {
	u := fmt.Sprintf("/%s/%s/status?build_id=%d", req.OrgName, req.SpaceName, req.BuildID)

	rc, err := h.doSSERequest(http.MethodGet, u, req)
	if err != nil {
		return nil, err
	}

	return &LogsResponse{
		SSEReadCloser: rc,
	}, nil
}

// Helper method to execute the actual HTTP request and read the response.
func (h *RemoteBuilder) doRequest(method, url string, data interface{}) (*http.Response, error) {
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

	{

		data, _ := httputil.DumpRequestOut(req, true)
		fmt.Println(string(data))

	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected http status code:%d", resp.StatusCode)
	}

	return resp, nil
}

func (h *RemoteBuilder) doSSERequest(method, url string, data interface{}) (io.ReadCloser, error) {
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
