package types

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

// SDKBaseURL resolves the base URL an official SDK expects when calling this
// upstream through the given API format: the openai SDKs (chat completions
// and responses) join their resource path onto a base that already carries
// the /v1 version segment, while the anthropic SDK appends the versioned
// resource path ("v1/messages") to the base itself. A per-protocol endpoint
// override or a full URL has the resource tail stripped from its path; a
// base URL gets /v1 appended (openai) or dropped (anthropic) when missing.
// Query parameters (e.g. Azure-style api-version) are preserved on the
// returned base. Formats without an SDK convention fall back to the URL
// as-is.
func (uc UpstreamConfig) SDKBaseURL(format ApiFormat) string {
	switch format.Normalized() {
	case ApiFormatChat:
		return uc.openaiSDKBaseURL(ApiFormatChat)
	case ApiFormatResponses:
		return uc.openaiSDKBaseURL(ApiFormatResponses)
	case ApiFormatMessages:
		return uc.anthropicSDKBaseURL()
	default:
		return strings.TrimRight(strings.TrimSpace(uc.URL), "/")
	}
}

func (uc UpstreamConfig) openaiSDKBaseURL(format ApiFormat) string {
	resourceTail := PathSegResponses
	if format == ApiFormatChat {
		resourceTail = PathSegChat + "/" + PathSegCompletions
	}
	if override := uc.DeclaredProtocolEndpoint(format); override != "" {
		return openaiBaseFromEndpoint(override, resourceTail)
	}
	if uc.IsFullURL {
		return openaiBaseFromEndpoint(uc.URL, resourceTail)
	}
	if inferred, ok := InferProtocolFromURLPath(uc.URL); ok && inferred == format {
		return openaiBaseFromEndpoint(uc.URL, resourceTail)
	}
	return withV1Suffix(uc.URL)
}

func (uc UpstreamConfig) anthropicSDKBaseURL() string {
	if override := uc.DeclaredProtocolEndpoint(ApiFormatMessages); override != "" {
		return anthropicBaseFromEndpoint(override)
	}
	if uc.IsFullURL {
		return anthropicBaseFromEndpoint(uc.URL)
	}
	if inferred, ok := InferProtocolFromURLPath(uc.URL); ok && inferred == ApiFormatMessages {
		return anthropicBaseFromEndpoint(uc.URL)
	}
	return withoutV1Suffix(uc.URL)
}

func openaiBaseFromEndpoint(endpoint, resourceTail string) string {
	u, ok := parseEndpointURL(endpoint)
	if !ok {
		return endpoint
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/"+resourceTail) {
		u.Path = strings.TrimSuffix(path, "/"+resourceTail)
		return buildEndpointURL(u)
	}
	if strings.HasSuffix(path, "/"+PathSegV1) {
		return buildEndpointURL(u)
	}
	slog.Warn("upstream endpoint does not end with the standard /v1/"+resourceTail+" path; the SDK will request <base>/"+resourceTail,
		slog.String("endpoint", endpoint))
	return buildEndpointURL(u)
}

func anthropicBaseFromEndpoint(endpoint string) string {
	u, ok := parseEndpointURL(endpoint)
	if !ok {
		return endpoint
	}
	path := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(path, "/"+PathSegV1+"/"+PathSegMessages) {
		u.Path = strings.TrimSuffix(path, "/"+PathSegV1+"/"+PathSegMessages)
		return buildEndpointURL(u)
	}
	slog.Warn("upstream messages endpoint does not end with the standard /v1/messages path; the SDK will request <base>/v1/messages",
		slog.String("endpoint", endpoint))
	u.Path = strings.TrimSuffix(path, "/"+PathSegMessages)
	return buildEndpointURL(u)
}

func withV1Suffix(base string) string {
	u, ok := parseEndpointURL(base)
	if !ok {
		return base
	}
	path := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(path, "/"+PathSegV1) {
		path += "/" + PathSegV1
	}
	u.Path = path
	return buildEndpointURL(u)
}

func withoutV1Suffix(base string) string {
	u, ok := parseEndpointURL(base)
	if !ok {
		return base
	}
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), "/"+PathSegV1)
	return buildEndpointURL(u)
}

// parseEndpointURL parses an endpoint or base URL, prepending a scheme when
// absent (mirroring InferProtocolFromURLPath). It reports false when the
// value cannot be parsed, in which case callers fall back to the raw string.
func parseEndpointURL(rawURL string) (*url.URL, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, false
	}
	return parsed, true
}

// buildEndpointURL rebuilds the URL string after path edits, dropping a
// stale RawPath and preserving the query string (e.g. Azure's
// ?api-version=...), mirroring ResolveSampleEndpoint.
func buildEndpointURL(u *url.URL) string {
	u.RawPath = ""
	return u.String()
}

// ParseAuthHeaders parses an upstream auth header value: a JSON object of
// arbitrary headers, or a plain "Bearer ..." string mapped to the
// Authorization header. It returns an error for values that are neither.
func ParseAuthHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return parsed, nil
	}
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return map[string]string{"Authorization": raw}, nil
	}
	return nil, fmt.Errorf("auth header is neither a JSON header map nor a Bearer token")
}

// InternalHost returns the virtual-host override for CSGHub-hosted deploy
// upstreams, or "" when the metadata carries none.
func (uc UpstreamConfig) InternalHost() string {
	if uc.Metadata != nil && uc.Metadata.InternalModelInfo != nil {
		return uc.Metadata.InternalModelInfo.Host
	}
	return ""
}
