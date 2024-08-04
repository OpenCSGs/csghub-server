package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

func NewHttpClient(endpoint string, opts ...RequestOption) *HttpClient {
	return &HttpClient{
		endpoint: endpoint,
		hc:       http.DefaultClient,
		authOpts: opts,
	}
}

type HttpClient struct {
	endpoint string
	hc       *http.Client
	authOpts []RequestOption
}

func (c *HttpClient) Get(ctx context.Context, path string, outObj interface{}) error {
	path = fmt.Sprintf("%s%s", c.endpoint, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	for _, opt := range c.authOpts {
		opt.Set(req)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do http request, path:%s, err:%w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get response, path:%s, status:%d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(outObj)
}
