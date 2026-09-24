// Package semanticrouter is the infrastructure client for the Semantic
// Router service.  The service ranks the platform's candidate models
// against the context of a single inference request; it only scores and
// orders candidates and never generates a completion itself.
//
// See the service's own API contract for the full response shape.  Only
// the fields the gateway actually uses are decoded here.
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

	// maxRankRequestBytes mirrors the service's own body limit.  Sending a
	// larger body would be rejected with 413, so oversized turns are
	// reported as a client-side error instead of being sent.
	maxRankRequestBytes = 2_000_000

	// maxErrorBodyBytes bounds how much of a failed response is read back
	// into the returned error message.
	maxErrorBodyBytes = 2048
)

// Message is one turn of the visible conversation handed to the ranking
// endpoint.  The service requires at least one message with role "user".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RankRequest is the POST /v1/rank body.  Tools carries the current tool
// definitions verbatim so the service sees the same context the upstream
// model would.
//
// The service also accepts a "debug" flag that returns a detailed ranking.
// It is deliberately not sent: it is a diagnostic switch, not part of the
// contract a production path should depend on.
type RankRequest struct {
	Messages []Message         `json:"messages"`
	Tools    []json.RawMessage `json:"tools,omitempty"`
}

// Candidate is one entry of the returned ranking, ordered by descending
// Score.  Model is the service's own benchmark identifier.  ProviderModel
// is the upstream model name, which the service reports only in its
// detailed ranking; it is empty on the ordinary path and callers must be
// able to resolve a candidate from Model alone.
type Candidate struct {
	Rank            int     `json:"rank"`
	Model           string  `json:"model"`
	ProviderModel   string  `json:"provider_model"`
	ProviderBaseURL string  `json:"provider_base_url"`
	Score           float64 `json:"score"`
}

// RankResponse is the normalized POST /v1/rank result.  IndexVersion and
// PolicyVersion identify the ranking inputs and are worth recording
// alongside the model that was finally chosen; both are empty when the
// service reported only the slim ordered list.
type RankResponse struct {
	Ranking       []Candidate
	IndexVersion  string
	PolicyVersion string
	ElapsedMS     float64
}

// rankDetail is the detailed ranking payload.  Deployments of the 0.1.1
// contract return these fields at the top level of the response; newer
// ones move them under "detail" and include them only when the request
// sets debug.
type rankDetail struct {
	Ranking       []Candidate `json:"ranking"`
	IndexVersion  string      `json:"index_version"`
	PolicyVersion string      `json:"policy_version"`
	ElapsedMS     float64     `json:"elapsed_ms"`
}

// rankPayload accepts both response shapes.  ModelList is the ordered
// list of benchmark identifiers newer deployments always return, and is
// the only ranking available when the detailed payload is withheld.
type rankPayload struct {
	rankDetail
	ModelList []string    `json:"model_list"`
	Detail    *rankDetail `json:"detail"`
}

// normalize reduces whichever shape arrived to a single RankResponse,
// preferring the detailed ranking because only it names the upstream
// model.  Falling back to the ordered benchmark identifiers keeps the
// client useful against a deployment that reports nothing else.
func (p *rankPayload) normalize() *RankResponse {
	detail := p.rankDetail
	if p.Detail != nil && len(p.Detail.Ranking) > 0 {
		detail = *p.Detail
	}
	if len(detail.Ranking) > 0 {
		return &RankResponse{
			Ranking:       detail.Ranking,
			IndexVersion:  detail.IndexVersion,
			PolicyVersion: detail.PolicyVersion,
			ElapsedMS:     detail.ElapsedMS,
		}
	}

	ranking := make([]Candidate, 0, len(p.ModelList))
	for i, model := range p.ModelList {
		ranking = append(ranking, Candidate{Rank: i + 1, Model: model})
	}
	return &RankResponse{Ranking: ranking}
}

// Readiness is what the service reports about itself.  IndexVersion
// identifies the data the rankings are produced from and is worth
// recording alongside a routing decision.
type Readiness struct {
	Ready            bool   `json:"ready"`
	IndexVersion     string `json:"index_version"`
	TrainingRequests int    `json:"training_requests"`
}

// Client talks to a Semantic Router deployment.
type Client interface {
	// Rank returns the service's ordered candidate list for one turn.
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
	if req == nil || len(req.Messages) == 0 {
		return nil, fmt.Errorf("semantic router rank requires at least one message")
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

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call semantic router rank: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("semantic router rank returned %d: %s", resp.StatusCode, readErrorBody(resp.Body))
	}

	var payload rankPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode semantic router rank response: %w", err)
	}
	ranked := payload.normalize()
	if len(ranked.Ranking) == 0 {
		return nil, fmt.Errorf("semantic router returned an empty ranking")
	}
	return ranked, nil
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
