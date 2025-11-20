package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/gorilla/websocket"
)

type Client interface {
	Push(ctx context.Context, req *LokiPushRequest) error
	Query(ctx context.Context, query string, limit int, start time.Time, direction string) (*LokiQueryResponse, error)
	QueryRange(ctx context.Context, params QueryRangeParams) (*LokiQueryResponse, error)
	Tail(ctx context.Context, query string, start time.Time) (<-chan *LokiPushRequest, error)
	Ready(ctx context.Context) error
}

func NewClient(rawURL string) (Client, error) {
	_, err := url.Parse(rawURL)
	return &client{
		url: rawURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, err
}

type client struct {
	url        string
	httpClient *http.Client
}

func (c *client) Push(ctx context.Context, pushRequest *LokiPushRequest) error {
	jsonData, err := json.Marshal(pushRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal push request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/loki/api/v1/push", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *client) Query(ctx context.Context, query string, limit int, start time.Time, direction string) (*LokiQueryResponse, error) {
	params := url.Values{}
	params.Add("query", query)
	if limit > 0 {
		params.Add("limit", fmt.Sprintf("%d", limit))
	}
	if !start.IsZero() {
		params.Add("start", fmt.Sprintf("%d", start.UnixNano()))
	}
	if direction != "" {
		params.Add("direction", direction)
	}

	queryURL := fmt.Sprintf("%s/loki/api/v1/query?%s", c.url, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send query to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResponse LokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return nil, fmt.Errorf("failed to decode Loki query response: %w", err)
	}

	return &queryResponse, nil
}

func (c *client) QueryRange(ctx context.Context, params QueryRangeParams) (*LokiQueryResponse, error) {
	urlParams := url.Values{}
	urlParams.Add("query", params.Query)
	if params.Limit > 0 {
		urlParams.Add("limit", fmt.Sprintf("%d", params.Limit))
	}
	if !params.Start.IsZero() {
		urlParams.Add("start", fmt.Sprintf("%d", params.Start.UnixNano()))
	}
	if !params.End.IsZero() {
		urlParams.Add("end", fmt.Sprintf("%d", params.End.UnixNano()))
	}
	if params.Since > 0 {
		urlParams.Add("since", params.Since.String())
	}
	if params.Step > 0 {
		urlParams.Add("step", params.Step.String())
	}
	if params.Interval > 0 {
		urlParams.Add("interval", params.Interval.String())
	}
	if params.Direction != "" {
		urlParams.Add("direction", params.Direction)
	}

	queryURL := fmt.Sprintf("%s/loki/api/v1/query_range?%s", c.url, urlParams.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create query_range request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send query_range to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("loki query_range failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResponse LokiQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResponse); err != nil {
		return nil, fmt.Errorf("failed to decode Loki query_range response: %w", err)
	}

	return &queryResponse, nil
}

func (c *client) Tail(ctx context.Context, query string, start time.Time) (<-chan *LokiPushRequest, error) {
	base, err := url.Parse(c.url)
	if err != nil {
		return nil, err
	}

	wsScheme := "ws"
	if base.Scheme == "https" {
		wsScheme = "wss"
	}

	u := url.URL{
		Scheme: wsScheme,
		Host:   base.Host,
		Path:   path.Join(base.Path, "/loki/api/v1/tail"),
	}

	params := url.Values{}
	params.Add("query", query)
	if !start.IsZero() {
		params.Add("start", fmt.Sprintf("%d", start.UnixNano()))
	}
	u.RawQuery = params.Encode()
	slog.Info("loki-tail", slog.Any("url", u.String()))
	ch := make(chan *LokiPushRequest)
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		if resp != nil && resp.Body != nil {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				slog.Error("Failed to read response body on websocket dial error", "err", readErr)
			}
			slog.Error("Failed to dial WebSocket", "err", err, "url", u.String(), "body", string(body))
			resp.Body.Close()
		} else {
			slog.Error("Failed to dial WebSocket", "err", err, "url", u.String())
		}
		return nil, err
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	go func() {
		defer conn.Close()
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				slog.Info("WebSocket connection closing due to context cancellation", "url", u.String())
				return
			default:
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				slog.Error("WebSocket read error", "err", err)
				return
			}

			var lokiLog LokiPushRequest
			if err := json.Unmarshal(message, &lokiLog); err != nil {
				slog.Error("Failed to unmarshal Loki log from websocket", "err", err)
				continue
			}

			select {
			case ch <- &lokiLog:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (c *client) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.url+"/ready", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki health check failed with status %d", resp.StatusCode)
	}

	return nil
}
