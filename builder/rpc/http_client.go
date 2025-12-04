package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/utils/trace"

	"github.com/avast/retry-go/v4"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
)

type HttpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type HttpClient struct {
	endpoint   string
	hc         *http.Client
	authOpts   []RequestOption
	logger     *slog.Logger
	retry      uint
	retryDelay time.Duration
}

func NewHttpDoer(endpoint string, opts ...RequestOption) HttpDoer {
	return NewHttpClient(endpoint, opts...)
}

func NewHttpClient(endpoint string, opts ...RequestOption) *HttpClient {
	defaultClient := &HttpClient{
		endpoint: endpoint,
		hc: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		authOpts:   opts,
		retry:      1,
		retryDelay: 100 * time.Millisecond,
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		// return default client without proxy but with otel and logging
		defaultClient.logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
		return defaultClient
	}

	// As a temporary solution, a new logger is created here.
	// For a better long-term solution, a centralized logging instance should be injected.
	handlers := []slog.Handler{
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		}),
	}
	if cfg.Instrumentation.OTLPEndpoint != "" && cfg.Instrumentation.OTLPLogging {
		handlers = append(handlers, otelslog.NewHandler("csghub-server"))
	}
	defaultClient.logger = slog.New(slogmulti.Fanout(handlers...))

	if !cfg.Proxy.Enable || cfg.Proxy.URL == "" {
		return defaultClient
	}
	proxyHosts := cfg.Proxy.Hosts
	proxyURL, err := url.Parse(cfg.Proxy.URL)
	if err != nil {
		slog.Error("failed to parse proxy url", "url", cfg.Proxy.URL, "error", err)
		return defaultClient
	}

	for _, host := range proxyHosts {
		if strings.Contains(endpoint, host) {
			defaultClient.hc = &http.Client{
				Transport: otelhttp.NewTransport(&http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				}),
			}
			return defaultClient
		}
	}

	return defaultClient
}

// WithRetry sets the total number of attempts for the requests. retry number is n-1
// 0 - means forever retry req | 1 - means no retry, n - means retry n-1 times
func (c *HttpClient) WithRetry(attempts uint) *HttpClient {
	c.retry = max(attempts, 1)
	return c
}

func (c *HttpClient) WithDelay(delay time.Duration) *HttpClient {
	c.retryDelay = delay
	return c
}

func (c *HttpClient) Get(ctx context.Context, path string, outObj interface{}) error {
	fullPath := fmt.Sprintf("%s%s", c.endpoint, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullPath, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", errorx.ErrInternalServerError)
	}
	for _, opt := range c.authOpts {
		opt.Set(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp httpbase.R
		jsonErr := json.NewDecoder(resp.Body).Decode(&errResp)
		if jsonErr == nil {
			customErr := errorx.ParseError(errResp.Msg, errorx.ErrRemoteServiceFail, errResp.Context)
			return customErr
		}
		return fmt.Errorf("failed to get response, path:%s, status:%d", path, resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(outObj)
	if err != nil {
		return fmt.Errorf("failed to decode resp body in HttpClient.Get, err:%w", errorx.ErrInternalServerError)
	}
	return nil
}

func (c *HttpClient) Post(ctx context.Context, path string, data interface{}, outObj interface{}) error {
	fullPath := fmt.Sprintf("%s%s", c.endpoint, path)
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullPath, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, opt := range c.authOpts {
		opt.Set(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp httpbase.R
		jsonErr := json.NewDecoder(resp.Body).Decode(&errResp)
		if jsonErr == nil {
			customErr := errorx.ParseError(errResp.Msg, errorx.ErrRemoteServiceFail, errResp.Context)
			return customErr
		}
		return fmt.Errorf("failed to get response, path:%s, status:%d", path, resp.StatusCode)
	}

	if outObj != nil {
		err = json.NewDecoder(resp.Body).Decode(outObj)
		if err != nil {
			return fmt.Errorf("failed to decode resp body in HttpClient.Post, err:%w", errorx.ErrInternalServerError)
		}
	}

	return nil
}

func (c *HttpClient) Do(req *http.Request) (resp *http.Response, err error) {
	ctx := req.Context()
	fullPath := req.URL.String()
	traceID, traceParent, _ := trace.GetOrGenTraceIDFromContext(ctx)
	if traceParent != "" {
		req.Header.Set(trace.HeaderTraceparent, traceParent)
	}

	startTime := time.Now()
	retryTime := time.Now()
	err = retry.Do(
		func() error {
			var doErr error
			retryTime = time.Now()
			// resp will be closed by caller and OnRetry
			r, doErr := c.hc.Do(req)
			if doErr != nil {
				if r != nil {
					_ = r.Body.Close()
				}
				return fmt.Errorf("failed to do http request, path:%s, err:%w", fullPath, doErr)
			}
			resp = r
			return nil
		},
		retry.Attempts(c.retry),
		retry.Delay(c.retryDelay),
		retry.OnRetry(func(n uint, err error) {
			if n == 0 {
				return
			}
			retryLatency := time.Since(retryTime).Milliseconds()
			c.logger.InfoContext(ctx, "retrying http request",
				slog.String("trace_id", traceID),
				slog.String("method", req.Method),
				slog.String("url", fullPath),
				slog.Uint64("retry_attempt", uint64(n)),
				slog.Int64("latency(ms)", retryLatency),
				slog.String("error", err.Error()),
			)
		}),
	)
	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		c.logger.ErrorContext(ctx, "http request failed after retries",
			slog.String("trace_id", traceID),
			slog.String("method", req.Method),
			slog.String("url", fullPath),
			slog.Int64("latency(ms)", latency),
			slog.String("error", err.Error()),
		)
		return nil, err
	} else {
		c.logger.InfoContext(ctx, "http request",
			slog.String("trace_id", traceID),
			slog.String("method", req.Method),
			slog.String("url", fullPath),
			slog.Int("status", resp.StatusCode),
			slog.Int64("latency(ms)", latency),
		)
	}

	return resp, nil
}

func (c *HttpClient) PostResponse(ctx context.Context, path string, data interface{}) (*http.Response, error) {
	fullPath := fmt.Sprintf("%s%s", c.endpoint, path)
	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal data: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullPath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, opt := range c.authOpts {
		opt.Set(req)
	}

	return c.Do(req)
}
