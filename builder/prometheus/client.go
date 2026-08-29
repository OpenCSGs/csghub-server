package prometheus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type PrometheusClient interface {
	SerialData(query string, apiSuffix string) (*types.PrometheusResponse, error)
	// QueryRange executes a PromQL range query against the Prometheus
	// /api/v1/query_range endpoint.  It returns the time-series result for
	// the given [start, end] range at the specified step interval.
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*types.PrometheusResponse, error)
	// QueryInstant executes a PromQL instant query against the Prometheus
	// /api/v1/query endpoint at the given timestamp.
	QueryInstant(ctx context.Context, query string, at time.Time) (*types.PrometheusResponse, error)
}

type prometheusClientImpl struct {
	client    *http.Client
	apiURL    string
	basicAuth string
}

func NewPrometheusClient(cfg *config.Config) PrometheusClient {
	client := &http.Client{}
	if cfg.Prometheus.InsecureSkipVerify || strings.HasPrefix(cfg.Prometheus.ApiAddress, "https://") {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Prometheus.InsecureSkipVerify || strings.HasPrefix(cfg.Prometheus.ApiAddress, "https://")},
		}
		client = &http.Client{Transport: tr}
	}
	return &prometheusClientImpl{
		client:    client,
		apiURL:    cfg.Prometheus.ApiAddress,
		basicAuth: cfg.Prometheus.BasicAuth,
	}
}

func (p *prometheusClientImpl) SerialData(query string, apiSuffix string) (*types.PrometheusResponse, error) {
	if len(p.apiURL) < 1 {
		return nil, fmt.Errorf("prometheus api address is not configured")
	}
	url := fmt.Sprintf("%s%s?query=%s", p.apiURL, apiSuffix, query)
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

// queryRangeURL converts the configured apiURL (which may end with
// /api/v1/query or /api/v1/query_range) into a /api/v1/query_range endpoint.
func queryRangeURL(apiURL string) string {
	r := strings.TrimSuffix(apiURL, "/")
	r = strings.TrimSuffix(r, "/api/v1/query")
	r = strings.TrimSuffix(r, "/api/v1/query_range")
	return r + "/api/v1/query_range"
}

func (p *prometheusClientImpl) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*types.PrometheusResponse, error) {
	if len(p.apiURL) < 1 {
		return nil, fmt.Errorf("prometheus api address is not configured")
	}

	u := fmt.Sprintf("%s?query=%s&start=%d&end=%d&step=%d",
		queryRangeURL(p.apiURL),
		url.QueryEscape(query),
		start.Unix(),
		end.Unix(),
		int64(step.Seconds()),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData any
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errData); decodeErr != nil {
			return nil, fmt.Errorf("unexpected http status code: %d, %w", resp.StatusCode, decodeErr)
		}
		return nil, fmt.Errorf("unexpected http status and error: %d, %v", resp.StatusCode, errData)
	}

	res := &types.PrometheusResponse{}
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, fmt.Errorf("decode response error: %w", err)
	}
	return res, nil
}

// queryInstantURL converts the configured apiURL into a /api/v1/query endpoint.
func queryInstantURL(apiURL string) string {
	r := strings.TrimSuffix(apiURL, "/")
	r = strings.TrimSuffix(r, "/api/v1/query")
	r = strings.TrimSuffix(r, "/api/v1/query_range")
	return r + "/api/v1/query"
}

// QueryInstant executes an instant PromQL query at the given timestamp.
func (p *prometheusClientImpl) QueryInstant(ctx context.Context, query string, at time.Time) (*types.PrometheusResponse, error) {
	if len(p.apiURL) < 1 {
		return nil, fmt.Errorf("prometheus api address is not configured")
	}

	u := fmt.Sprintf("%s?query=%s&time=%d",
		queryInstantURL(p.apiURL),
		url.QueryEscape(query),
		at.Unix(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData any
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errData); decodeErr != nil {
			return nil, fmt.Errorf("unexpected http status code: %d, %w", resp.StatusCode, decodeErr)
		}
		return nil, fmt.Errorf("unexpected http status and error: %d, %v", resp.StatusCode, errData)
	}

	res := &types.PrometheusResponse{}
	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return nil, fmt.Errorf("decode response error: %w", err)
	}
	return res, nil
}
