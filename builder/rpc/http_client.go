package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/common/config"
)

func NewHttpClient(endpoint string, opts ...RequestOption) *HttpClient {
	defaultClient := &HttpClient{
		endpoint: endpoint,
		hc:       http.DefaultClient,
		authOpts: opts,
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return defaultClient
	}
	if !cfg.Proxy.Enable || cfg.Proxy.URL == "" {
		return defaultClient
	}
	proxyHosts := cfg.Proxy.Hosts
	proxyURL, err := url.Parse(cfg.Proxy.URL)
	if err != nil {
		return defaultClient
	}

	for _, host := range proxyHosts {
		if strings.Contains(endpoint, host) {
			return &HttpClient{
				endpoint: endpoint,
				hc: &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxyURL),
					},
				},
				authOpts: opts,
			}
		}
	}

	return defaultClient
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get response, path:%s, status:%d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(outObj)
}

func (c *HttpClient) Post(ctx context.Context, path string, data interface{}, outObj interface{}) error {
	path = fmt.Sprintf("%s%s", c.endpoint, path)
	// serialize data as http request body
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewBuffer(jsonData))
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get response, path:%s, status:%d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(outObj)
}
