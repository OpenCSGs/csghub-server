package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SystemOne (jev) is the state-and-questions decision-model protocol: a
// client submits a state (the content to judge) plus named typed questions,
// and the model answers each question — "noul" questions with a 0-1 risk
// score, "choice" questions with a selected label. This SDK lets any module
// call a SystemOne decision model without re-implementing the wire format.
//
// Request:
//
//	{"model":"...","state":{"role":"user","content":"..."},
//	 "questions":{"q":{"type":"noul","instructions":"..."}}}
//
// Response:
//
//	{"answers":{"q":{"type":"noul","noul":0.87}},"usage":{...}}

// SystemOneQuestion is one named typed question of a SystemOne request.
type SystemOneQuestion struct {
	// Type is the question kind, e.g. "noul" (0-1 numeric score) or "choice"
	// (a label selected from Criteria).
	Type string `json:"type"`
	// Instructions tells the model how to answer the question.
	Instructions string `json:"instructions,omitempty"`
	// Criteria is the type-specific configuration as raw JSON: for "choice"
	// questions a label-to-description map the model picks from.
	Criteria json.RawMessage `json:"criteria,omitempty"`
}

// SystemOneRequest is a SystemOne request. Model, State, and Questions are
// required by the protocol.
type SystemOneRequest struct {
	Model     string                       `json:"model"`
	RequestID string                       `json:"request_id,omitempty"`
	State     json.RawMessage              `json:"state"`
	Questions map[string]SystemOneQuestion `json:"questions"`
}

// SystemOneState is the conventional state shape when the judged content is
// a single message.
type SystemOneState struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SystemOneResponse is a SystemOne response. Each answer is typed JSON keyed
// by the question name ("noul" answers carry a noul score, "choice" answers
// a selected label).
type SystemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
	Usage   *SystemOneUsage            `json:"usage"`
}

// SystemOneUsage reports token consumption and input handling for the call.
type SystemOneUsage struct {
	InputTokens        int64    `json:"input_tokens"`
	OutputTokens       int64    `json:"output_tokens"`
	Truncated          bool     `json:"truncated"`
	TruncatedQuestions []string `json:"truncated_questions,omitempty"`
}

// maxSystemOneResponseBytes bounds how much of a response body is read: a
// misbehaving endpoint must not be able to exhaust memory.
const maxSystemOneResponseBytes = 1 << 20 // 1 MiB

// SystemOneStatusError reports a non-200 response. It carries the status
// code only: the upstream body may echo the submitted state, which must not
// leak into error strings that end up in logs.
type SystemOneStatusError struct {
	StatusCode int
}

func (e *SystemOneStatusError) Error() string {
	return fmt.Sprintf("systemone endpoint returned status %d", e.StatusCode)
}

// SystemOneBadResponseError marks a 2xx response that cannot be parsed into
// a usable SystemOneResponse (undecodable body, missing answers). Callers
// use it to distinguish "the model answered garbage" from transport
// failures.
type SystemOneBadResponseError struct {
	Err error
}

func (e *SystemOneBadResponseError) Error() string { return e.Err.Error() }
func (e *SystemOneBadResponseError) Unwrap() error { return e.Err }

// SystemOneClientConfig configures a SystemOneClient.
type SystemOneClientConfig struct {
	// Endpoint is the full URL of the SystemOne service.
	Endpoint string
	// Headers are custom HTTP headers sent with every request. "Host" is
	// moved to req.Host to comply with Go's HTTP client behavior.
	Headers map[string]string
	// HTTPClient is optional; a plain client is used when nil. Callers that
	// need a hard backstop pass one with a Timeout set — per-call deadlines
	// otherwise come from the context.
	HTTPClient *http.Client
}

// SystemOneClient calls a SystemOne decision-model endpoint. It is safe for
// concurrent use.
type SystemOneClient struct {
	httpClient *http.Client
	endpoint   string
	headers    map[string]string
	host       string
}

// NewSystemOneClient builds a client for the configured endpoint.
func NewSystemOneClient(cfg SystemOneClientConfig) *SystemOneClient {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	headers := make(map[string]string, len(cfg.Headers))
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	host := headers["Host"]
	delete(headers, "Host")
	return &SystemOneClient{
		httpClient: httpClient,
		endpoint:   cfg.Endpoint,
		headers:    headers,
		host:       host,
	}
}

// Post submits the request and decodes the typed response. Transport errors
// are returned as-is; non-200 responses as *SystemOneStatusError; unusable
// 2xx bodies as *SystemOneBadResponseError.
func (c *SystemOneClient) Post(ctx context.Context, req *SystemOneRequest) (*SystemOneResponse, error) {
	if req == nil {
		return nil, &SystemOneBadResponseError{fmt.Errorf("nil request")}
	}
	if req.Model == "" {
		return nil, &SystemOneBadResponseError{fmt.Errorf("model is required")}
	}
	if len(req.State) == 0 || string(req.State) == "null" {
		return nil, &SystemOneBadResponseError{fmt.Errorf("state is required")}
	}
	if len(req.Questions) == 0 {
		return nil, &SystemOneBadResponseError{fmt.Errorf("questions is required")}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal systemone request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create systemone request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	if c.host != "" {
		httpReq.Host = c.host
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call systemone endpoint: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSystemOneResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read systemone response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &SystemOneStatusError{StatusCode: resp.StatusCode}
	}

	var out SystemOneResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &SystemOneBadResponseError{fmt.Errorf("decode systemone response: %w", err)}
	}
	if out.Answers == nil {
		return nil, &SystemOneBadResponseError{fmt.Errorf("systemone response has no answers")}
	}
	return &out, nil
}

// ParseSystemOneNoul extracts the 0-1 score from a "noul" answer. Answers
// that violate the protocol (wrong question type, missing or out-of-range
// score) are rejected so upstream/protocol anomalies degrade to the
// caller's bad-response path instead of a wrong verdict.
func ParseSystemOneNoul(raw json.RawMessage) (float64, error) {
	var answer struct {
		Type string   `json:"type"`
		Noul *float64 `json:"noul"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return 0, fmt.Errorf("decode noul answer: %w", err)
	}
	if answer.Type != "noul" {
		return 0, fmt.Errorf("unexpected answer type %q, want noul", answer.Type)
	}
	if answer.Noul == nil {
		return 0, fmt.Errorf("noul answer has no score")
	}
	if *answer.Noul < 0 || *answer.Noul > 1 {
		return 0, fmt.Errorf("noul score %v out of range [0,1]", *answer.Noul)
	}
	return *answer.Noul, nil
}

// ParseSystemOneChoice extracts the selected label from a "choice" answer.
func ParseSystemOneChoice(raw json.RawMessage) (string, error) {
	var answer struct {
		Type   string `json:"type"`
		Choice string `json:"choice"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return "", fmt.Errorf("decode choice answer: %w", err)
	}
	return answer.Choice, nil
}
