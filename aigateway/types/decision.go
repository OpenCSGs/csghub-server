package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// DecisionRequest is the opaque JSON contract accepted by POST /v1/decisions.
// Input and questions are preserved as raw JSON so multimodal inputs and
// protocol extensions pass through without interpretation.
type DecisionRequest struct {
	Model            string          `json:"model"`
	Input            json.RawMessage `json:"input"`
	Questions        json.RawMessage `json:"questions"`
	SafetyIdentifier string          `json:"safety_identifier,omitempty"`
	RawBody          json.RawMessage `json:"-"`
	ClientModel      string          `json:"-"`
}

func (r *DecisionRequest) UnmarshalJSON(data []byte) error {
	type alias DecisionRequest
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	decoded.RawBody = append(json.RawMessage(nil), data...)
	decoded.ClientModel = decoded.Model
	*r = DecisionRequest(decoded)
	return nil
}

func (r *DecisionRequest) Validate() error {
	if strings.TrimSpace(r.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if len(bytes.TrimSpace(r.Input)) == 0 || bytes.Equal(bytes.TrimSpace(r.Input), []byte("null")) {
		return fmt.Errorf("input is required")
	}
	if len(bytes.TrimSpace(r.Questions)) == 0 || bytes.Equal(bytes.TrimSpace(r.Questions), []byte("null")) {
		return fmt.Errorf("questions is required")
	}
	return nil
}

// PromptText implements PromptTextProvider: the input and questions text, so
// the Planner can run sensitive-content checks and the admission layer can
// estimate the prompt size without knowing the protocol. Empty containers
// carry no checkable content and are skipped.
func (r *DecisionRequest) PromptText() string {
	var b strings.Builder
	appendText := func(raw json.RawMessage) {
		text := JSONToText(raw)
		if text == "" || text == "{}" || text == "[]" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	appendText(r.Input)
	appendText(r.Questions)
	return b.String()
}

// DecisionResponse is the parsed view needed for usage accounting and tracing.
// The raw upstream response is forwarded unchanged.
type DecisionResponse struct {
	Model   string          `json:"model"`
	Answers json.RawMessage `json:"answers"`
	Usage   *DecisionUsage  `json:"usage"`
}

type DecisionUsage struct {
	InputTokens         int64                      `json:"input_tokens"`
	InputTokensDetails  DecisionInputTokenDetails  `json:"input_tokens_details"`
	OutputTokens        int64                      `json:"output_tokens"`
	OutputTokensDetails DecisionOutputTokenDetails `json:"output_tokens_details"`
	TotalTokens         int64                      `json:"total_tokens"`
	ComputeUnits        *int64                     `json:"compute_units,omitempty"`
}

func (u DecisionUsage) EffectiveTotalTokens() int64 {
	if u.TotalTokens != 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
}

type DecisionInputTokenDetails struct {
	CachedTokens     int64 `json:"cached_tokens"`
	CacheWriteTokens int64 `json:"cache_write_tokens"`
}

type DecisionOutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens"`
}
