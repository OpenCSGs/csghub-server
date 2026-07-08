
package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// upstreamTestTimeout is the maximum duration allowed for an upstream
// connectivity test request.
const upstreamTestTimeout = 30 * time.Second

// maskedAuthSecret is the placeholder used to redact sensitive header values
// in the request summary returned to the frontend.
const maskedAuthSecret = "................."

// endpointChatCompletions and endpointResponses are the two supported
// upstream endpoint path suffixes.
const (
	endpointChatCompletions = "/chat/completions"
	endpointResponses       = "/responses"
)

// testEndpointKind describes which protocol the upstream URL speaks.
type testEndpointKind int

const (
	endpointKindUnsupported testEndpointKind = iota
	endpointKindChatCompletions
	endpointKindResponses
)

// detectEndpointKind inspects the upstream URL path and returns the
// supported endpoint kind. Only /chat/completions and /responses are
// supported; anything else returns endpointKindUnsupported.
func detectEndpointKind(rawURL string) testEndpointKind {
	// Trim query string and fragment before checking the path suffix.
	u := rawURL
	if idx := strings.IndexAny(u, "?#"); idx >= 0 {
		u = u[:idx]
	}
	u = strings.TrimRight(u, "/")
	switch {
	case strings.HasSuffix(u, endpointChatCompletions):
		return endpointKindChatCompletions
	case strings.HasSuffix(u, endpointResponses):
		return endpointKindResponses
	default:
		return endpointKindUnsupported
	}
}

// parseAuthHeader parses the upstream auth_header field into a map of
// HTTP headers. The auth_header is either a plain "Bearer xxx" string or
// a JSON object string like {"Authorization":"Bearer xxx"}.
func parseAuthHeader(authHeader string) (map[string]string, error) {
	trimmed := strings.TrimSpace(authHeader)
	if trimmed == "" {
		return map[string]string{}, nil
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		// Not a JSON object; treat as a bare Authorization value.
		return map[string]string{
			"Authorization": trimmed,
		}, nil
	}

	headers := make(map[string]string, len(parsed))
	for k, v := range parsed {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		headers[key] = v
	}
	return headers, nil
}

// maskRequestHeaders returns a copy of the headers with sensitive values
// redacted. Only Content-Type is preserved verbatim; all other values
// (including Authorization / apikey) are masked.
func maskRequestHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for k, v := range headers {
		switch strings.ToLower(k) {
		case "content-type":
			masked[k] = v
		default:
			masked[k] = maskedAuthSecret
		}
	}
	return masked
}

// buildTestRequestBody constructs the request body for the given endpoint
// kind. A simple "hi" prompt is used for both protocols.
func buildTestRequestBody(kind testEndpointKind, modelName string) (map[string]any, error) {
	switch kind {
	case endpointKindChatCompletions:
		return map[string]any{
			"model":    modelName,
			"messages": []map[string]string{{"role": "user", "content": "hi"}},
			"stream":   false,
		}, nil
	case endpointKindResponses:
		return map[string]any{
			"model":  modelName,
			"input":  "hi",
			"stream": false,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported upstream endpoint, only %s and %s are supported", endpointChatCompletions, endpointResponses)
	}
}

// requestSummary is the masked request representation sent to the frontend.
type requestSummary struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
}

// doUpstreamTest performs the HTTP request against the upstream and returns
// the test result. It is split from TestUpstream so it can be unit tested
// with an injectable http.Client.
func doUpstreamTest(ctx context.Context, client *http.Client, url string, kind testEndpointKind, modelName string, authHeaders map[string]string) (*types.TestUpstreamResult, error) {
	body, err := buildTestRequestBody(kind, modelName)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	requestHeaders := map[string]string{
		"Content-Type": "application/json",
	}
	for k, v := range authHeaders {
		requestHeaders[k] = v
	}

	summary := requestSummary{
		URL:     url,
		Method:  http.MethodPost,
		Headers: maskRequestHeaders(requestHeaders),
		Body:    body,
	}
	summaryBytes, _ := json.MarshalIndent(summary, "", "  ")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return &types.TestUpstreamResult{
			Request: string(summaryBytes),
			Error:   err.Error(),
		}, nil
	}
	for k, v := range requestHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return &types.TestUpstreamResult{
			Request: string(summaryBytes),
			Error:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &types.TestUpstreamResult{
			Request:     string(summaryBytes),
			OK:          false,
			Status:      resp.StatusCode,
			StatusText:  resp.Status,
			ResponseBody: "",
			Error:       fmt.Sprintf("failed to read response body: %v", err),
		}, nil
	}
	rawText := string(rawBytes)

	var prettyBody string
	var jsonObj map[string]any
	if json.Unmarshal(rawBytes, &jsonObj) == nil {
		pretty, _ := json.MarshalIndent(jsonObj, "", "  ")
		prettyBody = string(pretty)
	} else {
		prettyBody = rawText
	}

	return &types.TestUpstreamResult{
		Request:      string(summaryBytes),
		OK:           resp.StatusCode >= 200 && resp.StatusCode < 300,
		Status:       resp.StatusCode,
		StatusText:   resp.Status,
		Content:      rawText,
		ResponseBody: prettyBody,
	}, nil
}

// TestUpstream tests connectivity to an upstream endpoint by ID.
func (s *llmServiceComponentImpl) TestUpstream(ctx context.Context, req *types.TestUpstreamReq) (*types.TestUpstreamResult, error) {
	dbUp, err := s.upstreamStore.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("upstream not found: %w", err)
	}

	url := strings.TrimSpace(dbUp.URL)
	if url == "" {
		return nil, fmt.Errorf("upstream url is empty")
	}
	modelName := strings.TrimSpace(dbUp.ModelName)
	if modelName == "" {
		return nil, fmt.Errorf("upstream model_name is empty")
	}

	kind := detectEndpointKind(url)
	if kind == endpointKindUnsupported {
		return nil, errorx.ReqParamInvalid(
			fmt.Errorf("unsupported upstream endpoint, only %s and %s are supported", endpointChatCompletions, endpointResponses),
			nil,
		)
	}

	authHeaders, err := parseAuthHeader(dbUp.AuthHeader)
	if err != nil {
		return nil, fmt.Errorf("invalid auth_header: %w", err)
	}

	testCtx, cancel := context.WithTimeout(ctx, upstreamTestTimeout)
	defer cancel()

	client := &http.Client{Timeout: upstreamTestTimeout}

	slog.InfoContext(ctx, "testing upstream connection",
		slog.Int64("upstream_id", dbUp.ID),
		slog.String("url", url),
		slog.String("endpoint_kind", endpointKindString(kind)),
	)

	return doUpstreamTest(testCtx, client, url, kind, modelName, authHeaders)
}

func endpointKindString(k testEndpointKind) string {
	switch k {
	case endpointKindChatCompletions:
		return endpointChatCompletions
	case endpointKindResponses:
		return endpointResponses
	default:
		return "unsupported"
	}
}
