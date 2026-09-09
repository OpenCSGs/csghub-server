package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// maskedAuthSecret is the placeholder used to redact sensitive header values
// in the request summary returned to the frontend.
const maskedAuthSecret = "................."

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
// redacted. Content-Type and Anthropic-Version are non-secret protocol metadata;
// all other values are masked because authentication header names are user-defined.
func maskRequestHeaders(headers map[string]string) map[string]string {
	masked := make(map[string]string, len(headers))
	for k, v := range headers {
		switch strings.ToLower(k) {
		case "content-type", "anthropic-version":
			masked[k] = v
		default:
			masked[k] = maskedAuthSecret
		}
	}
	return masked
}

// requestSummary is the masked request representation sent to the frontend.
type requestSummary struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
}

func summarizeSampleRequestBody(headers http.Header, body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return nil, nil
	}
	mediaType, parameters, err := mime.ParseMediaType(headers.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("parse sample request content type: %w", err)
	}
	switch mediaType {
	case "application/json":
		var summary map[string]any
		if err := json.Unmarshal(body, &summary); err != nil {
			return nil, fmt.Errorf("decode sample request body: %w", err)
		}
		return summary, nil
	case "multipart/form-data":
		boundary := parameters["boundary"]
		if boundary == "" {
			return nil, fmt.Errorf("decode sample multipart body: boundary is missing")
		}
		return summarizeMultipartBody(multipart.NewReader(bytes.NewReader(body), boundary))
	default:
		return nil, fmt.Errorf("unsupported sample request content type %q", mediaType)
	}
}

func summarizeMultipartBody(reader *multipart.Reader) (map[string]any, error) {
	summary := map[string]any{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return summary, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode sample multipart body: %w", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("read sample multipart field %q: %w", part.FormName(), err)
		}
		var value any = string(data)
		if part.FileName() != "" {
			value = map[string]any{
				"filename":     part.FileName(),
				"content_type": part.Header.Get("Content-Type"),
				"size":         len(data),
			}
		}
		appendRequestSummaryValue(summary, part.FormName(), value)
	}
}

func appendRequestSummaryValue(summary map[string]any, key string, value any) {
	if existing, ok := summary[key]; ok {
		if values, ok := existing.([]any); ok {
			summary[key] = append(values, value)
		} else {
			summary[key] = []any{existing, value}
		}
		return
	}
	summary[key] = value
}

// doUpstreamTest performs the protocol-specific sample request against the
// upstream and returns the test result. It applies the provider's inference
// policy to both the request context and HTTP client.
func doUpstreamTest(ctx context.Context, provider aigatewaytypes.SampleProvider, url, modelName string, authHeaders map[string]string) (*types.TestUpstreamResult, error) {
	policy, err := provider.ExecutionPolicy(aigatewaytypes.SampleKindInference)
	if err != nil {
		return nil, fmt.Errorf("get sample execution policy: %w", err)
	}
	if policy.Timeout <= 0 {
		return nil, fmt.Errorf("sample execution timeout must be positive")
	}
	testCtx, cancel := context.WithTimeout(ctx, policy.Timeout)
	defer cancel()
	client := &http.Client{Timeout: policy.Timeout}

	headers := make(http.Header, len(authHeaders))
	for k, v := range authHeaders {
		headers.Set(k, v)
	}

	execution, err := provider.Execute(testCtx, aigatewaytypes.SampleKindInference, aigatewaytypes.SampleInput{
		Endpoint: url,
		Headers:  headers,
		Model:    modelName,
		Text:     "hi",
	}, client)
	if err != nil {
		return nil, err
	}
	if execution == nil || execution.Request == nil {
		return nil, fmt.Errorf("sample execution returned no request")
	}

	requestHeaders := make(map[string]string, len(execution.Request.Headers))
	for k, values := range execution.Request.Headers {
		if len(values) > 0 {
			requestHeaders[k] = values[0]
		}
	}
	body, err := summarizeSampleRequestBody(execution.Request.Headers, execution.Request.Body)
	if err != nil {
		return nil, err
	}

	summary := requestSummary{
		URL:     execution.Request.Endpoint,
		Method:  execution.Request.Method,
		Headers: maskRequestHeaders(requestHeaders),
		Body:    body,
	}
	summaryBytes, _ := json.MarshalIndent(summary, "", "  ")
	if execution.Error != nil {
		return &types.TestUpstreamResult{
			Request: string(summaryBytes),
			Error:   execution.Error.Error(),
		}, nil
	}
	rawText := string(execution.ResponseBody)

	var prettyBody string
	var jsonObj map[string]any
	if json.Unmarshal(execution.ResponseBody, &jsonObj) == nil {
		pretty, _ := json.MarshalIndent(jsonObj, "", "  ")
		prettyBody = string(pretty)
	} else {
		prettyBody = rawText
	}

	return &types.TestUpstreamResult{
		Request:      string(summaryBytes),
		OK:           execution.StatusCode >= 200 && execution.StatusCode < 300,
		Status:       execution.StatusCode,
		StatusText:   execution.Status,
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

	provider, ok := s.sampleRegistry.Find(url)
	if !ok {
		return nil, errorx.ErrUpstreamConnectionTestNotSupported
	}

	authHeaders, err := parseAuthHeader(dbUp.AuthHeader)
	if err != nil {
		return nil, fmt.Errorf("invalid auth_header: %w", err)
	}

	slog.InfoContext(ctx, "testing upstream connection",
		slog.Int64("upstream_id", dbUp.ID),
		slog.String("url", url),
	)

	return doUpstreamTest(ctx, provider, url, modelName, authHeaders)
}

func (s *llmServiceComponentImpl) validateHealthCheckEndpoint(url string, healthCheckEnabled bool) error {
	if !healthCheckEnabled {
		return nil
	}
	if _, ok := s.sampleRegistry.Find(strings.TrimSpace(url)); !ok {
		return errorx.ErrUpstreamHealthCheckNotSupported
	}
	return nil
}
