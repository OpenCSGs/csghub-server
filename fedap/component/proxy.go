//go:build saas

package component

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/fedap/types"
)

const proxyAuthorizationHeaderPrefix = "OAuth "

// ProxyComponent provides the fedap proxy operation.
// It resolves the remote site, loads a valid OAuth access token, and forwards the
// requested path/query to the remote service.
type ProxyComponent interface {
	Proxy(ctx context.Context, req types.ProxyRequest) (*types.ProxyResponse, error)
}

// proxyComponentImpl is the production implementation of ProxyComponent.
type proxyComponentImpl struct {
	oauth        OAuthComponent     // handles token lifecycle for outbound requests
	siteProvider SiteConfigProvider // resolves site_id to connection details
	httpClient   *http.Client       // shared HTTP client for outbound requests
}

// NewProxyComponent creates a new ProxyComponent. The OAuthComponent is injected
// to share token state across all resource components.
func NewProxyComponent(cfg *config.Config, oauth OAuthComponent) (ProxyComponent, error) {
	return &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: NewSiteConfigProvider(cfg),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Proxy forwards the request to the remote site using a valid OAuth access token
// and returns the upstream status, headers, and body as-is.
// RFC 8693 token exchange support is intentionally not used here yet; the current
// proxy flow directly forwards the site access token in the Authorization header.
func (c *proxyComponentImpl) Proxy(ctx context.Context, req types.ProxyRequest) (*types.ProxyResponse, error) {
	siteCfg, err := c.siteProvider.GetSiteConfig(ctx, req.SiteID)
	if err != nil {
		return nil, errorx.SiteFetchFailedErr(err, errorx.Ctx().Set("site_id", req.SiteID))
	}

	accessToken, err := c.oauth.GetValidAccessToken(ctx, req.UserUUID, req.SiteID)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "proxy request",
		slog.String("base_url", siteCfg.BaseURL),
		slog.String("path", req.Path),
		slog.String("method", req.Method),
		slog.String("site_id", req.SiteID),
		slog.String("user_uuid", req.UserUUID),
	)
	reqURL, err := buildProxyURL(siteCfg.BaseURL, req.Path, req.Query)
	if err != nil {
		return nil, errorx.ProxyRequestProcessErr(err,
			errorx.Ctx().Set("error_info", "build proxy URL error").
				Set("base_url", siteCfg.BaseURL).
				Set("path", req.Path),
		)
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, errorx.ProxyRequestProcessErr(err,
			errorx.Ctx().Set("error_info", "build request error").
				Set("base_url", siteCfg.BaseURL).
				Set("path", req.Path),
		)
	}

	// Authorization: OAuth <token>
	httpReq.Header.Set("Authorization", proxyAuthorizationHeaderPrefix+accessToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errorx.ProxyRequestProcessErr(err, errorx.Ctx().Set("error_info", "request error").Set("url", reqURL))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorx.ProxyRequestProcessErr(err, errorx.Ctx().Set("error_info", "read body error"))
	}

	slog.InfoContext(ctx, "proxy response", slog.Int("status", resp.StatusCode), slog.Int("body-length", len(body)))
	return &types.ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    cloneResponseHeaders(resp.Header),
		Body:       body,
	}, nil
}

func buildProxyURL(siteBaseURL, requestPath string, requestQuery map[string][]string) (string, error) {
	baseURL, err := url.Parse(siteBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse site base URL: %w", err)
	}

	pathURL, err := url.Parse(strings.TrimSpace(requestPath))
	if err != nil {
		return "", fmt.Errorf("parse request path: %w", err)
	}
	if pathURL.IsAbs() || pathURL.Host != "" {
		return "", fmt.Errorf("request path must be relative")
	}
	if pathURL.Path == "" {
		return "", fmt.Errorf("request path is required")
	}
	if !strings.HasPrefix(pathURL.Path, "/") {
		pathURL.Path = "/" + pathURL.Path
	}

	resolvedURL := baseURL.ResolveReference(&url.URL{
		Path:    pathURL.Path,
		RawPath: pathURL.RawPath,
	})

	query := pathURL.Query()
	for key, values := range requestQuery {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	resolvedURL.RawQuery = query.Encode()

	return resolvedURL.String(), nil
}

func cloneResponseHeaders(headers http.Header) map[string][]string {
	result := make(map[string][]string, len(headers))
	for key, values := range headers {
		result[key] = append([]string(nil), values...)
	}
	return result
}
