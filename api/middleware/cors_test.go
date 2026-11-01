package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func TestMatchCORSOrigin(t *testing.T) {
	patterns := []string{
		"https://hub.opencsg.com",
		"https://*.opencsg.com",
		"http://localhost:3000",
		"not-an-origin",
	}
	cases := []struct {
		origin string
		want   bool
	}{
		{origin: "https://hub.opencsg.com", want: true},         // exact entry
		{origin: "HTTPS://HUB.OPENCSG.COM/", want: true},        // case and trailing slash normalized
		{origin: "https://a.opencsg.com", want: true},           // wildcard subdomain
		{origin: "https://a.b.opencsg.com", want: false},        // wildcard covers one label only
		{origin: "https://opencsg.com", want: false},            // wildcard never covers the apex
		{origin: "https://evil-opencsg.com", want: false},       // suffix lookalike
		{origin: "https://a.opencsg.com.evil.com", want: false}, // suffix injection
		{origin: "https://hub.opencsg.com:8443", want: false},   // different port is a different origin
		{origin: "http://hub.opencsg.com", want: false},         // scheme mismatch
		{origin: "http://localhost:3000", want: true},           // exact entry with port
		{origin: "https://localhost:3000", want: false},         // scheme mismatch with port
		{origin: "not-an-origin", want: false},                  // invalid origin never matches
		{origin: "", want: false},                               // empty origin never matches
	}
	for _, tc := range cases {
		got := matchCORSOrigin(patterns, tc.origin)
		require.Equal(t, tc.want, got, "origin %q", tc.origin)
	}
}

func newCORSTestEngine(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BuildCORS(cfg))
	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.GET("/sse", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.WriteHeader(http.StatusOK)
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = c.Writer.WriteString("data: hello\n\n")
	})
	return r
}

func corsTestConfig(origins []string, frontendURL string) *config.Config {
	cfg := config.Config{}
	cfg.CORS.AllowedOrigins = origins
	cfg.Frontend.URL = frontendURL
	return &cfg
}

func TestBuildCORS_WhitelistedOriginReflectsExactOriginWithCredentials(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"https://hub.opencsg.com"}, ""))

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Origin", "https://hub.opencsg.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://hub.opencsg.com", w.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestBuildCORS_NonWhitelistedOriginRejectedWithoutReadHeaders(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"https://hub.opencsg.com"}, ""))

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestBuildCORS_EmptyWhitelistDeniesForeignOrigin(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig(nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestBuildCORS_PreflightWhitelistedOrigin(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"https://*.opencsg.com"}, ""))

	req := httptest.NewRequest(http.MethodOptions, "/json", nil)
	req.Header.Set("Origin", "https://hub.opencsg.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, "https://hub.opencsg.com", w.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	require.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
}

func TestBuildCORS_PreflightNonWhitelistedOriginRejected(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"https://hub.opencsg.com"}, ""))

	req := httptest.NewRequest(http.MethodOptions, "/json", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestBuildCORS_RequestWithoutOriginPassesThrough(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig(nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestBuildCORS_SameOriginRequestPassesThrough(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig(nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestBuildCORS_SSEResponseCarriesAllowedOrigin(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"https://hub.opencsg.com"}, ""))

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Origin", "https://hub.opencsg.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://hub.opencsg.com", w.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, w.Body.String(), "data: hello")
}

func TestBuildCORS_FrontendURLOriginAlwaysAllowed(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig(nil, "https://opencsg.com/"))

	allowed := httptest.NewRequest(http.MethodGet, "/json", nil)
	allowed.Header.Set("Origin", "https://opencsg.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, allowed)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://opencsg.com", w.Header().Get("Access-Control-Allow-Origin"))

	other := httptest.NewRequest(http.MethodGet, "/json", nil)
	other.Header.Set("Origin", "https://evil.example")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, other)
	require.Equal(t, http.StatusForbidden, w2.Code)
}

func TestBuildCORS_PublicDomainOriginAlwaysAllowed(t *testing.T) {
	cfg := corsTestConfig(nil, "")
	cfg.APIServer.PublicDomain = "https://hub.opencsg.com"
	r := newCORSTestEngine(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Origin", "https://hub.opencsg.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://hub.opencsg.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestBuildCORS_PublicRootDomainDerivesWildcard(t *testing.T) {
	cfg := corsTestConfig(nil, "")
	cfg.Space.PublicRootDomain = "space.opencsg.com"
	r := newCORSTestEngine(t, cfg)

	cases := []struct {
		origin string
		want   int
	}{
		{origin: "https://s-abc123.space.opencsg.com", want: http.StatusOK},
		{origin: "http://s-abc123.space.opencsg.com", want: http.StatusOK},
		{origin: "https://space.opencsg.com", want: http.StatusForbidden},     // apex not covered
		{origin: "https://a.b.space.opencsg.com", want: http.StatusForbidden}, // single label only
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, tc.want, w.Code, "origin %q", tc.origin)
	}
}

func TestBuildCORS_ExplicitListReplacesDerivation(t *testing.T) {
	cfg := corsTestConfig([]string{"https://partner.example"}, "https://opencsg.com")
	cfg.APIServer.PublicDomain = "https://hub.opencsg.com"
	cfg.Space.PublicRootDomain = "space.opencsg.com"
	r := newCORSTestEngine(t, cfg)

	allowed := httptest.NewRequest(http.MethodGet, "/json", nil)
	allowed.Header.Set("Origin", "https://partner.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, allowed)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://partner.example", w.Header().Get("Access-Control-Allow-Origin"))

	// Derived origins (frontend, public domain, space wildcard) are disabled
	// once an explicit list is set: the list is the whole whitelist.
	for _, origin := range []string{
		"https://opencsg.com",
		"https://hub.opencsg.com",
		"https://s-abc123.space.opencsg.com",
		"https://agentichub.opencsg.com",
	} {
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		req.Header.Set("Origin", origin)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req)
		require.Equal(t, http.StatusForbidden, w2.Code, "origin %q", origin)
	}
}

func TestBuildCORS_EmptyConfigTrustsOnlyDeploymentOrigins(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig(nil, ""))

	// Without any configuration the whitelist is derived from the
	// deployment's own addresses only; nothing else is trusted.
	for _, origin := range []string{
		"https://agentichub.opencsg.com",
		"http://localhost:8090",
		"https://api.github.com",
	} {
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, "origin %q", origin)
	}
}

func TestBuildCORS_InvalidWhitelistEntriesNeverMatch(t *testing.T) {
	r := newCORSTestEngine(t, corsTestConfig([]string{"hub.opencsg.com", "https://hub.opencsg.com/path"}, ""))

	for _, origin := range []string{"https://hub.opencsg.com", "hub.opencsg.com"} {
		req := httptest.NewRequest(http.MethodGet, "/json", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, "origin %q", origin)
	}
}
