package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactingHandlerRedactsSensitiveAttributes(t *testing.T) {
	type nestedCredentials struct {
		Username string `json:"username"`
		Token    string `json:"token"`
	}

	var output bytes.Buffer
	handler := NewRedactingHandler(slog.NewJSONHandler(&output, nil))
	logger := slog.New(handler).With("api_key", "bound-secret")
	logger.InfoContext(context.Background(), "request failed",
		slog.String("authorization", "Bearer direct-secret"),
		slog.Group("oauth", slog.String("refresh_token", "group-secret"), slog.String("provider", "example")),
		slog.Any("request", nestedCredentials{Username: "alice", Token: "nested-secret"}),
	)

	line := output.String()
	for _, secret := range []string{"bound-secret", "direct-secret", "group-secret", "nested-secret"} {
		require.NotContains(t, line, secret)
	}
	require.Contains(t, line, redactedValue)
	require.Contains(t, line, "alice")
	require.Contains(t, line, "example")
}

func TestRedactingHandlerSanitizesHTTPRequest(t *testing.T) {
	requestURL, err := url.Parse("https://alice:password@example.com/path?jwt_token=url-secret&plain=visible")
	require.NoError(t, err)
	request := &http.Request{
		Method: http.MethodGet,
		URL:    requestURL,
		Header: http.Header{
			"Authorization": []string{"Bearer header-secret"},
			"Private-Token": []string{"private-secret"},
			"X-Request-ID":  []string{"request-123"},
		},
	}

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))
	logger.Info("proxy request", slog.Any("request", request))

	line := output.String()
	for _, secret := range []string{"password", "url-secret", "header-secret", "private-secret"} {
		require.NotContains(t, line, secret)
	}
	require.Contains(t, line, redactedValue)
	require.Contains(t, line, "visible")
	require.Contains(t, line, "request-123")
}

func TestSafeHeadersDoesNotMutateInput(t *testing.T) {
	headers := http.Header{
		"Authorization": []string{"Bearer secret"},
		"X-User-Token":  []string{"user-secret"},
		"Content-Type":  []string{"application/json"},
	}

	got := SafeHeaders(headers)

	require.Equal(t, "Bearer secret", headers.Get("Authorization"))
	require.Equal(t, redactedValue, got.Get("Authorization"))
	require.Equal(t, redactedValue, got.Get("X-User-Token"))
	require.Equal(t, "application/json", got.Get("Content-Type"))
}

func TestSafeURLStringRedactsCredentials(t *testing.T) {
	rawURL := "https://alice:password@example.com/path?access_token=secret&plain=visible"

	got := SafeURLString(rawURL)

	require.NotContains(t, got, "password")
	require.NotContains(t, got, "secret")
	require.Contains(t, got, url.QueryEscape(redactedValue))
	require.Contains(t, got, "plain=visible")
	require.False(t, strings.Contains(got, "alice"))
}

func TestSafeErrorRedactsURLCredentials(t *testing.T) {
	err := &url.Error{
		Op:  "Get",
		URL: "https://alice:password@example.com/path?access_token=secret",
		Err: context.DeadlineExceeded,
	}

	got := SafeError(err)

	require.NotContains(t, got.Error(), "password")
	require.NotContains(t, got.Error(), "secret")
	require.Contains(t, got.Error(), url.QueryEscape(redactedValue))
}

func TestSafeErrorRedactsCredentialTextAndPreservesCause(t *testing.T) {
	cause := context.Canceled
	err := fmt.Errorf("request failed with Authorization: Bearer jwt-secret and api_key=key-secret: %w", cause)

	got := SafeError(err)

	require.NotContains(t, got.Error(), "jwt-secret")
	require.NotContains(t, got.Error(), "key-secret")
	require.Contains(t, got.Error(), redactedValue)
	require.ErrorIs(t, got, cause)
}

func TestSensitiveAPIKeySuffixIsRedacted(t *testing.T) {
	type providerConfig struct {
		OpenAIAPIKey string `json:"openai_api_key"`
	}

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))
	logger.Error("provider failed", "error", errors.New("api_key=error-secret"), "config", providerConfig{OpenAIAPIKey: "field-secret"})

	line := output.String()
	require.NotContains(t, line, "error-secret")
	require.NotContains(t, line, "field-secret")
	require.NotContains(t, SafeURLString("/?openai_api_key=query-secret"), "query-secret")
}

func TestCredentialFingerprintIsStableAndDoesNotExposeCredential(t *testing.T) {
	const credential = "short-secret"

	first := CredentialFingerprint("api_key", credential)
	second := CredentialFingerprint("api_key", credential)

	require.Equal(t, first, second)
	require.Equal(t, "api_key:sha256:4cce02651adfe686", first)
	require.NotContains(t, first, credential)
	require.Empty(t, CredentialFingerprint("api_key", ""))
}

func TestRedactingHandlerPreservesOrdinaryStructuredValues(t *testing.T) {
	type ordinary struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))
	logger.Info("ordinary", slog.Any("value", ordinary{Name: "item", Count: 2}))

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, map[string]any{"name": "item", "count": float64(2)}, record["value"])
}

func TestSafeURLRedactsCredentialPathsAndOAuthCallbacks(t *testing.T) {
	for _, raw := range []string{
		"/jwt/path-secret?plain=visible", "/api/v1/jwt/path-secret",
		"/token/path-secret", "/api/v1/token/path-secret/quotas",
		"/api/v1/token/path%2Dsecret/quotas", "/token/path-secret%2Fother",
		"https://example.com/api/v1/fedap/oauth/callback?code=code-secret&state=state-secret&plain=visible",
		"/api/v1/federation-adapter/oauth/callback?CODE=code-secret&State=state-secret",
		"/api/v1/callback/casdoor?code=code-secret&state=state-secret",
	} {
		t.Run(raw, func(t *testing.T) {
			parsed, err := url.Parse(raw)
			require.NoError(t, err)
			original := parsed.String()
			got := SafeURL(parsed).String()
			for _, secret := range []string{"path-secret", "path%2Dsecret", "code-secret", "state-secret"} {
				require.NotContains(t, got, secret)
			}
			require.Contains(t, got, url.QueryEscape(redactedValue))
			require.Equal(t, original, parsed.String())
		})
	}
	for _, raw := range []string{"/api/v1/token/csghub/my-key", "/models/alice/model?code=sample&state=ready"} {
		require.Equal(t, raw, SafeURLString(raw))
	}
	require.Equal(t, redactedValue, SafeURLString("https://user:secret@host/%zz"))
}

func TestSafeTextRedactsJSONAndEmbeddedURLs(t *testing.T) {
	for _, raw := range []string{
		`{"access_token":"json-secret","refresh_token":"other-secret"}`,
		`{"API_KEY": "json-secret", "password": "other-secret"}`,
		`{"access_token":"json-secret\"other-secret"}`,
		`request failed: https://user:json-secret@host/path?token=other-secret`,
		`jwt=json-secret token=other-secret`,
	} {
		got := SafeText(raw)
		require.NotContains(t, got, "json-secret")
		require.NotContains(t, got, "other-secret")
	}
}

func TestRedactingHandlerRedactsNestedURLStrings(t *testing.T) {
	config := struct {
		Database struct{ DSN string }
		Nats     struct{ URL string }
		Values   map[string]any
	}{}
	config.Database.DSN = "postgresql://alice:db-secret@localhost/db?sslmode=disable"
	config.Nats.URL = "nats://alice:nats-secret@localhost:4222"
	config.Values = map[string]any{"items": []string{"https://alice:list-secret@example.com/path", `{"access_token":"json-secret"}`}}

	var output bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&output, nil)))
	logger.Info("config", "data", config, "path", "/api/v1/jwt/path-secret", "targetUrl", "vscode://open?jwt=redirect-secret")
	logger.Info("database", "dsn", "host=localhost password=dsn-secret dbname=server")
	for _, secret := range []string{"db-secret", "nats-secret", "list-secret", "json-secret", "path-secret", "redirect-secret", "dsn-secret"} {
		require.NotContains(t, output.String(), secret)
	}
	require.Contains(t, output.String(), "sslmode=disable")
	require.Contains(t, config.Database.DSN, "db-secret")
	require.Contains(t, config.Nats.URL, "nats-secret")
}
