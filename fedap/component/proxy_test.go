//go:build saas

package component

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	mockfedap "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/fedap/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/fedap/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type proxyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f proxyRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newStubHTTPClient(fn proxyRoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestProxyComponent_Proxy_UsesAccessTokenAndReturnsUpstreamResponse(t *testing.T) {
	client := newStubHTTPClient(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "https://site.example.com/api/v1/models/detail?builtin=1&page=2&tag=a&tag=b", r.URL.String())
		assert.Equal(t, "OAuth access-token-1", r.Header.Get("Authorization"))

		return &http.Response{
			StatusCode: http.StatusAccepted,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Upstream":   []string{"models"},
			},
			Body: io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-1", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	resp, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Method:   http.MethodGet,
		Path:     "/api/v1/models/detail?builtin=1",
		Query: map[string][]string{
			"page": {"2"},
			"tag":  {"a", "b"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, []byte(`{"ok":true}`), resp.Body)
	assert.Equal(t, []string{"application/json"}, resp.Headers["Content-Type"])
	assert.Equal(t, []string{"models"}, resp.Headers["X-Upstream"])
}

func TestProxyComponent_Proxy_UsesUpstreamResponseAsIs(t *testing.T) {
	client := newStubHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
		}, nil
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-2", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	resp, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Path:     "/missing",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, []byte("not found"), resp.Body)
}

func TestBuildProxyURL_RejectsAbsolutePath(t *testing.T) {
	_, err := buildProxyURL("https://site.example.com", "https://evil.example.com/api", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relative")
}

func TestBuildProxyURL_RequiresPath(t *testing.T) {
	_, err := buildProxyURL("https://site.example.com", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestCloneResponseHeaders(t *testing.T) {
	headers := http.Header{
		"X-Test": []string{"a", "b"},
	}

	cloned := cloneResponseHeaders(headers)
	require.Equal(t, []string{"a", "b"}, cloned["X-Test"])

	headers["X-Test"][0] = "changed"
	assert.Equal(t, []string{"a", "b"}, cloned["X-Test"])
}

func TestProxyComponent_NewProxyComponent(t *testing.T) {
	cfg := &config.Config{}
	cfg.APIServer.PublicDomain = "http://localhost"
	cfg.APIServer.Port = 8080
	comp, err := NewProxyComponent(cfg, mockfedap.NewMockOAuthComponent(t))
	require.NoError(t, err)
	assert.NotNil(t, comp)
}

func TestProxyComponent_ProxyBuildsDefaultMethod(t *testing.T) {
	var gotMethod string
	client := newStubHTTPClient(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-3", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	_, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Path:     "/healthz",
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
}

func TestProxyComponent_ProxyWithRelativePathWithoutLeadingSlash(t *testing.T) {
	var gotURL string
	client := newStubHTTPClient(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-4", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	_, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Path:     "api/v1/models",
	})

	require.NoError(t, err)
	assert.Equal(t, "https://site.example.com/api/v1/models", gotURL)
}

func TestProxyComponent_Proxy_ReturnsProxyProcessErrorWhenRequestFails(t *testing.T) {
	client := newStubHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("request failed")
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-5", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	_, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Path:     "api/v1/models",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, errorx.ErrProxyRequestProcess)
}

func TestProxyComponent_ProxyPropagatesSiteLookupError(t *testing.T) {
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "missing-site").
		Return(nil, assert.AnError).
		Once()

	comp := &proxyComponentImpl{
		oauth:        mockfedap.NewMockOAuthComponent(t),
		siteProvider: siteProvider,
		httpClient:   &http.Client{},
	}

	_, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "missing-site",
		Path:     "/api/v1/models",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, errorx.ErrSiteFetchFailed)
}

func TestProxyComponent_Proxy_ReturnsProxyProcessErrorWhenReadBodyFails(t *testing.T) {
	client := newStubHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(errReader{err: errors.New("read body failed")}),
		}, nil
	})

	oauth := mockfedap.NewMockOAuthComponent(t)
	oauth.EXPECT().
		GetValidAccessToken(mock.Anything, "user-1", "site-1").
		Return("access-token-6", nil).
		Once()
	siteProvider := mockfedap.NewMockSiteConfigProvider(t)
	siteProvider.EXPECT().
		GetSiteConfig(mock.Anything, "site-1").
		Return(&types.SiteConfig{SiteID: "site-1", BaseURL: "https://site.example.com"}, nil).
		Once()

	comp := &proxyComponentImpl{
		oauth:        oauth,
		siteProvider: siteProvider,
		httpClient:   client,
	}

	_, err := comp.Proxy(context.Background(), types.ProxyRequest{
		UserUUID: "user-1",
		SiteID:   "site-1",
		Path:     "/broken",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, errorx.ErrProxyRequestProcess)
}

type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
