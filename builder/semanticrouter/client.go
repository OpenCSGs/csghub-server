// Package semanticrouter is the infrastructure client for the Semantic
// Router service.  The service ranks the platform's candidate models
// against the context of a single agent turn; it only scores and orders
// candidates and never generates a completion itself.
//
// The wire contract is the "agent-turn-input/1" version documented in the
// service's own API doc.  Only the fields the gateway actually uses are
// decoded here.
package semanticrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	rankPath  = "/v1/rank"
	readyPath = "/readyz"

	// headerSessionID and headerTurnID carry the routing identity the
	// service returns on its response and expects back on the next request.
	// They travel in HTTP headers only, never in the request or response
	// body.  The names are case-insensitive on the wire.
	headerSessionID = "CSG-Router-Session-Id"
	headerTurnID    = "CSG-Router-Turn-Id"

	// maxRankRequestBytes mirrors the service's own body limit.  Sending a
	// larger body would be rejected with 413, so oversized turns are
	// reported as a client-side error instead of being sent.
	maxRankRequestBytes = 2_000_000

	// maxErrorBodyBytes bounds how much of a failed response is read back
	// into the returned error message.
	maxErrorBodyBytes = 2048
)

// Message is one entry of the conversation handed to the ranking endpoint,
// kept at text level so the gateway can forward every protocol the same
// way.  The service requires non-empty content on each message the gateway
// sends, or tool_calls for messages whose only payload is a tool call; the
// gateway flattens to text and drops content-less turns before calling, so
// content is always set.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RankRequest is the POST /v1/rank body.  SystemPrompt is the model-visible
// system/developer rules, History is the conversation before the current
// user input, and CurrentInput is the current user input (which must
// contain at least one user message).  Tools carries the current tool
// definitions verbatim so the service sees the same context the upstream
// model would.
//
// SessionID and TurnID are the routing identity the service returns on a
// fresh request.  They are sent in the CSG-Router-Session-Id and
// CSG-Router-Turn-Id headers rather than in the body: a new session omits
// both, a new turn of an existing session sends SessionID only, and a
// repeated call within the same turn sends both and receives the ranking
// saved for that turn.
type RankRequest struct {
	SystemPrompt string            `json:"system_prompt"`
	Tools        []json.RawMessage `json:"tools,omitempty"`
	History      []Message         `json:"history,omitempty"`
	CurrentInput []Message         `json:"current_input"`
	SessionID    string            `json:"-"`
	TurnID       string            `json:"-"`
	Debug        bool              `json:"debug,omitempty"`
}

// RankResponse is the normalized POST /v1/rank result.  ModelList is the
// ordered list of gateway-callable model names, from most to least
// recommended.  SessionID and TurnID identify the routing record and are
// read from the response headers, not the body.
type RankResponse struct {
	SessionID string   `json:"-"`
	TurnID    string   `json:"-"`
	ModelList []string `json:"model_list"`
}

// Readiness is what the service reports about itself on GET /readyz.
// IndexVersion identifies the data the rankings are produced from and is
// worth recording alongside a routing decision.
type Readiness struct {
	Ready         bool   `json:"ready"`
	IndexVersion  string `json:"index_version"`
	TrainingTurns int    `json:"training_turns"`
}

// Client talks to a Semantic Router deployment.
type Client interface {
	// Rank returns the service's ordered model list for one turn.
	Rank(ctx context.Context, req *RankRequest) (*RankResponse, error)
	// Ready reports whether the service is currently able to rank, and
	// which index version it would rank against.  It returns an error
	// when the service is unreachable or reports itself as not ready.
	Ready(ctx context.Context) (*Readiness, error)
}

type httpClient struct {
	baseURL string
	client  *http.Client
}

// NewClient builds a Client for a Semantic Router base URL such as
// "https://semantic-router.example.com:1032".  A trailing slash and an
// accidental "/v1" suffix are both tolerated so operators can paste
// either form into the environment variable.
func NewClient(baseURL string, timeout time.Duration) (Client, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &httpClient{
		baseURL: normalized,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func normalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("semantic router base url is empty")
	}
	trimmed = strings.TrimRight(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/v1")
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid semantic router base url %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("semantic router base url %q must use http or https", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("semantic router base url %q has no host", raw)
	}
	return trimmed, nil
}

func (c *httpClient) Rank(ctx context.Context, req *RankRequest) (*RankResponse, error) {
	if req == nil || len(req.CurrentInput) == 0 {
		return nil, fmt.Errorf("semantic router rank requires at least one current input message")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode semantic router rank request: %w", err)
	}
	if len(body) > maxRankRequestBytes {
		return nil, fmt.Errorf("semantic router rank request is %d bytes, over the %d byte limit", len(body), maxRankRequestBytes)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+rankPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build semantic router rank request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.SessionID != "" {
		httpReq.Header.Set(headerSessionID, req.SessionID)
	}
	if req.TurnID != "" {
		httpReq.Header.Set(headerTurnID, req.TurnID)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call semantic router rank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semantic router rank returned %d: %s", resp.StatusCode, readErrorBody(resp.Body))
	}

	var ranked RankResponse
	if err := json.NewDecoder(resp.Body).Decode(&ranked); err != nil {
		return nil, fmt.Errorf("failed to decode semantic router rank response: %w", err)
	}
	if len(ranked.ModelList) == 0 {
		return nil, fmt.Errorf("semantic router returned an empty model list")
	}
	ranked.SessionID = resp.Header.Get(headerSessionID)
	ranked.TurnID = resp.Header.Get(headerTurnID)
	return &ranked, nil
}

func (c *httpClient) Ready(ctx context.Context) (*Readiness, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+readyPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build semantic router readiness request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call semantic router readiness: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semantic router readiness returned %d: %s", resp.StatusCode, readErrorBody(resp.Body))
	}

	var readiness Readiness
	if err := json.NewDecoder(resp.Body).Decode(&readiness); err != nil {
		return nil, fmt.Errorf("failed to decode semantic router readiness response: %w", err)
	}
	if !readiness.Ready {
		return nil, fmt.Errorf("semantic router reports itself as not ready")
	}
	return &readiness, nil
}

func readErrorBody(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, maxErrorBodyBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
