package prometheus

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type PrometheusClient interface {
	SerialData(query string) (*types.PrometheusResponse, error)
}

type prometheusClientImpl struct {
	client    *http.Client
	apiURL    string
	basicAuth string
}

func NewPrometheusClient(cfg *config.Config) PrometheusClient {
	client := &http.Client{}
	if strings.HasPrefix(cfg.Prometheus.ApiAddress, "https://") {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	}
	return &prometheusClientImpl{
		client:    client,
		apiURL:    cfg.Prometheus.ApiAddress,
		basicAuth: cfg.Prometheus.BasicAuth,
	}
}

func (p *prometheusClientImpl) SerialData(query string) (*types.PrometheusResponse, error) {
	if len(p.apiURL) < 1 {
		return nil, fmt.Errorf("prometheus api address is not configured")
	}
	url := fmt.Sprintf("%s?query=%s", p.apiURL, url.QueryEscape(query))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if p.basicAuth != "" {
		req.Header.Add("Authorization", "Basic "+p.basicAuth)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData any
		err := json.NewDecoder(resp.Body).Decode(&errData)
		if err != nil {
			return nil, fmt.Errorf("unexpected http status code: %d, %w", resp.StatusCode, err)
		} else {
			return nil, fmt.Errorf("unexpected http status and error: %d, %v", resp.StatusCode, errData)
		}
	}

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	res := &types.PrometheusResponse{}

	err = json.NewDecoder(resp.Body).Decode(res)
	if err != nil {
		return nil, fmt.Errorf("decode response error: %w", err)
	}

	return res, nil
}
