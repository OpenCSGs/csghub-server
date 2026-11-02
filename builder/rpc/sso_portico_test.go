package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
)

func newPorticoTestConfig(t *testing.T, endpoint string) (*config.Config, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.Portico.Endpoint = endpoint
	cfg.Portico.ClientID = "client-id"
	cfg.Portico.ClientSecret = "client-secret"
	cfg.Portico.RedirectURI = "http://hub.example.com/api/v1/callback/portico"
	cfg.Portico.Scopes = "openid profile email"
	cfg.Redis.Endpoint = mr.Addr()
	return cfg, mr
}

func TestPorticoClient_BuildAuthorizationURL(t *testing.T) {
	cfg, mr := newPorticoTestConfig(t, "http://portico.example.com/")
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	authURL, oauthState, err := p.BuildAuthorizationURL(context.Background(), "vscode")
	require.NoError(t, err)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "portico.example.com", u.Host)
	assert.Equal(t, "/authorize", u.Path)
	q := u.Query()
	assert.Equal(t, "client-id", q.Get("client_id"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, cfg.Portico.RedirectURI, q.Get("redirect_uri"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "openid profile email", q.Get("scope"))

	// the returned state token must equal the state emitted in the URL
	assert.Equal(t, q.Get("state"), oauthState)

	nonce, routingState, ok := SplitPorticoSigninState(q.Get("state"))
	require.True(t, ok)
	assert.Equal(t, "vscode", routingState)
	assert.Len(t, nonce, porticoNonceLen)

	// the code challenge must be derived from the verifier kept in redis
	verifier, err := mr.Get(porticoSigninStateKey(nonce))
	require.NoError(t, err)
	require.NotEmpty(t, verifier)
	challenge := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(challenge[:]), q.Get("code_challenge"))
}

func TestPorticoClient_GetOAuthToken(t *testing.T) {
	var gotAuth string
	var gotForm url.Values
	endpoint := "http://portico-token"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/oauth/token", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/oauth/token", r.URL.Path)
		require.NoError(t, r.ParseForm())
		gotAuth = r.Header.Get("Authorization")
		gotForm = r.PostForm
		return httpmock.NewStringResponse(http.StatusOK, `{
			"access_token": "access-token-1",
			"token_type": "Bearer",
			"expires_in": 3600
		}`), nil
	})

	cfg, mr := newPorticoTestConfig(t, endpoint)
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	nonce := strings.Repeat("ab", 16)
	verifier := "test-code-verifier"
	require.NoError(t, mr.Set(porticoSigninStateKey(nonce), verifier))

	token, err := p.GetOAuthToken(context.Background(), "auth-code", nonce+"casdoor")
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "access-token-1", token.AccessToken)

	// portico clients default to client_secret_basic
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("client-id:client-secret")), gotAuth)
	assert.Equal(t, "authorization_code", gotForm.Get("grant_type"))
	assert.Equal(t, "auth-code", gotForm.Get("code"))
	assert.Equal(t, verifier, gotForm.Get("code_verifier"))
	assert.Equal(t, cfg.Portico.RedirectURI, gotForm.Get("redirect_uri"))

	// the state must be consumed once
	_, err = p.GetOAuthToken(context.Background(), "auth-code", nonce+"casdoor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestPorticoClient_GetOAuthToken_InvalidState(t *testing.T) {
	cfg, _ := newPorticoTestConfig(t, "http://portico.example.com")
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	_, err = p.GetOAuthToken(context.Background(), "auth-code", "casdoor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid portico signin state")
}

func TestPorticoClient_GetOAuthToken_TokenEndpointError(t *testing.T) {
	endpoint := "http://portico-token-error"
	httpmock.RegisterResponder(http.MethodPost, endpoint+"/oauth/token", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
	})

	cfg, mr := newPorticoTestConfig(t, endpoint)
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	nonce := strings.Repeat("ab", 16)
	require.NoError(t, mr.Set(porticoSigninStateKey(nonce), "test-code-verifier"))

	_, err = p.GetOAuthToken(context.Background(), "auth-code", nonce+"casdoor")
	require.Error(t, err)
}

func TestPorticoClient_GetUserInfo(t *testing.T) {
	var gotAuth string
	endpoint := "http://portico-userinfo"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/userinfo", func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/userinfo", r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		return httpmock.NewStringResponse(http.StatusOK, `{
			"sub": "user-uuid-1",
			"name": "Display Name",
			"preferred_username": "alice",
			"email": "alice@example.com",
			"phone_number": "+8613800000000"
		}`), nil
	})

	cfg, _ := newPorticoTestConfig(t, endpoint)
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	info, err := p.GetUserInfo(context.Background(), "access-token-1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer access-token-1", gotAuth)
	assert.Equal(t, &SSOUserInfo{
		Name:        "alice",
		Email:       "alice@example.com",
		UUID:        "user-uuid-1",
		RegProvider: SSOTypePortico,
		Phone:       "+8613800000000",
	}, info)
}

func TestPorticoClient_GetUserInfo_NameFallback(t *testing.T) {
	endpoint := "http://portico-userinfo-fallback"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/userinfo", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusOK, `{
			"sub": "user-uuid-1",
			"name": "display-name"
		}`), nil
	})

	cfg, _ := newPorticoTestConfig(t, endpoint)
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	info, err := p.GetUserInfo(context.Background(), "access-token-1")
	require.NoError(t, err)
	assert.Equal(t, "display-name", info.Name)
}

func TestPorticoClient_GetUserInfo_Non200(t *testing.T) {
	endpoint := "http://portico-userinfo-error"
	httpmock.RegisterResponder(http.MethodGet, endpoint+"/userinfo", func(r *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(http.StatusInternalServerError, "boom"), nil
	})

	cfg, _ := newPorticoTestConfig(t, endpoint)
	p, err := NewPorticoClient(cfg)
	require.NoError(t, err)

	_, err = p.GetUserInfo(context.Background(), "access-token-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code 500")
}

func TestSplitPorticoSigninState(t *testing.T) {
	nonce, routingState, ok := SplitPorticoSigninState(strings.Repeat("ab", 16) + "http://flow.example.com/cb")
	assert.True(t, ok)
	assert.Equal(t, strings.Repeat("ab", 16), nonce)
	assert.Equal(t, "http://flow.example.com/cb", routingState)

	_, _, ok = SplitPorticoSigninState("casdoor")
	assert.False(t, ok)

	_, _, ok = SplitPorticoSigninState(strings.Repeat("ab", 16))
	assert.True(t, ok)
}
