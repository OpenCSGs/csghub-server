package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Jev (System One) is the state-and-questions evaluation protocol served by
// the AIGateway under /v1/systemone, following the TypeSafe SDK guide.  A
// client submits a situation ("state") and a set of named typed questions;
// the upstream model answers each question and reports token usage.
//
// Request:
//
//	{
//	  "model": "jev-1.13",
//	  "state": "I was charged twice for my subscription.",
//	  "questions": {
//	    "refund": {
//	      "type": "noul",
//	      "instructions": "Is the customer asking for money back?"
//	    }
//	  }
//	}
//
// Response:
//
//	{
//	  "id": "gen-dec-...",
//	  "model": "typesafe/jev-1.13-20260917",
//	  "provider": "TypeSafe",
//	  "answers": {"refund": {"type": "noul", "noul": 0.98}},
//	  "usage": {"input_tokens": 275, "output_tokens": 20, "cost": 0.00003}
//	}

// JevRequest is a POST /v1/systemone request.
// Unknown fields are preserved in ExtraFields for native passthrough.
type JevRequest struct {
	Model string `json:"model"`
	// State is the situation to evaluate. The protocol allows any JSON
	// value: a plain string or a structured object/array (e.g. a state
	// carrying ticket, order, and policy fields). It is kept as raw JSON so
	// structured states survive the gateway round-trip unchanged.
	State json.RawMessage `json:"state"`
	// Questions holds the named typed questions submitted to the model.
	Questions map[string]JevQuestion `json:"questions"`
	// RawBody keeps the client's original request bytes. The native
	// systemone path proxies this body with only the field the gateway
	// must override (model), so every other field — including protocol
	// additions the gateway does not model — is forwarded exactly as sent.
	RawBody json.RawMessage `json:"-"`
	// ClientModel is the model name exactly as the client sent it. The
	// handler overwrites Model with the resolved upstream name, so this
	// field tells the raw-body fast path whether a model rewrite is
	// actually needed.
	ClientModel string `json:"-"`
	// ExtraFields holds unknown top-level fields for the typed
	// serialization path (requests built programmatically, without a raw
	// body).
	ExtraFields map[string]json.RawMessage `json:"-"`
}

// JevQuestion is one named question submitted to the Jev model.
type JevQuestion struct {
	// Type is the question kind, e.g. "noul" (numeric score) or "choice".
	Type string `json:"type"`
	// Instructions tells the model how to answer the question. The
	// protocol's EntryType allows any JSON value (string, object, array,
	// or null), so it is kept as raw JSON.
	Instructions json.RawMessage `json:"instructions,omitempty"`
	// Criteria is the type-specific question configuration kept as raw
	// JSON: a choice map for "choice" questions (required for that type),
	// or other JSON shapes (e.g. score definitions) for other types.
	Criteria json.RawMessage `json:"criteria,omitempty"`
	// ExtraFields holds unknown question fields for passthrough fidelity.
	ExtraFields map[string]json.RawMessage `json:"-"`
}

// Validate checks the required fields of a Jev request.
func (r *JevRequest) Validate() error {
	if strings.TrimSpace(r.Model) == "" {
		return fmt.Errorf("model is required")
	}
	if !jevStatePresent(r.State) {
		return fmt.Errorf("state is required and must not be empty")
	}
	if len(r.Questions) == 0 {
		return fmt.Errorf("questions is required and must not be empty")
	}
	for name, q := range r.Questions {
		if strings.TrimSpace(q.Type) == "" {
			return fmt.Errorf("questions.%s.type is required", name)
		}
		if q.Type == "choice" && len(q.Criteria) == 0 {
			return fmt.Errorf("questions.%s.criteria is required for choice questions", name)
		}
	}
	return nil
}

// jevStatePresent reports whether the raw state JSON carries a usable value:
// present, not null, and not an empty (or whitespace-only) string.
func jevStatePresent(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return false
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil && strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

// JevJSONToText renders a raw JSON value as plain text for prompt checks,
// token estimation, and log capture: JSON strings are used verbatim, JSON
// null renders as empty, and every other JSON value (object, array, number)
// is kept as compact JSON.
func JevJSONToText(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return s
		}
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err == nil {
		return buf.String()
	}
	return string(trimmed)
}

// PromptText implements PromptTextProvider: the state plus the question
// instructions and criteria, so the Planner can run sensitive-content
// checks and the admission layer can estimate the prompt size without
// knowing the protocol.  Unknown request and question fields are included
// too — they are forwarded to the upstream verbatim, so they must not
// bypass the safety gate.
func (r *JevRequest) PromptText() string {
	var b strings.Builder
	appendText := func(raw json.RawMessage) {
		text := JevJSONToText(raw)
		// Empty containers carry no checkable content.
		if text == "" || text == "{}" || text == "[]" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	appendText(r.State)
	for _, name := range sortedJevQuestionNames(r.Questions) {
		q := r.Questions[name]
		appendText(q.Instructions)
		appendText(q.Criteria)
		for _, key := range sortedJevFieldNames(q.ExtraFields) {
			appendText(q.ExtraFields[key])
		}
	}
	for _, key := range sortedJevFieldNames(r.ExtraFields) {
		appendText(r.ExtraFields[key])
	}
	return b.String()
}

// sortedJevQuestionNames returns the question names in sorted order so
// PromptText stays deterministic.
func sortedJevQuestionNames(questions map[string]JevQuestion) []string {
	names := make([]string, 0, len(questions))
	for name := range questions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// sortedJevFieldNames returns the map keys in sorted order so PromptText
// stays deterministic.
func sortedJevFieldNames(fields map[string]json.RawMessage) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// jevRequestKnownFields lists the fields decoded into typed struct members;
// everything else lands in ExtraFields.
var jevRequestKnownFields = []string{"model", "state", "questions"}

// UnmarshalJSON preserves unknown fields in ExtraFields.
func (r *JevRequest) UnmarshalJSON(data []byte) error {
	type alias JevRequest
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	for _, key := range jevRequestKnownFields {
		delete(allFields, key)
	}
	tmp.ExtraFields = allFields
	// Copy the original bytes: the decoder may reuse its internal buffer,
	// and the raw body is proxied to the upstream after the request body
	// has been consumed.
	tmp.RawBody = append(json.RawMessage(nil), data...)
	// Model still holds the client's value here; the handler patches the
	// raw body (or overwrites Model) with the resolved upstream name before
	// proxying.
	tmp.ClientModel = tmp.Model
	*r = JevRequest(tmp)
	return nil
}

// MarshalJSON merges ExtraFields back into the output.
func (r JevRequest) MarshalJSON() ([]byte, error) {
	type alias JevRequest
	known, err := json.Marshal(alias(r))
	if err != nil {
		return nil, err
	}
	if len(r.ExtraFields) == 0 {
		return known, nil
	}
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(known, &knownFields); err != nil {
		return nil, err
	}
	for k, v := range r.ExtraFields {
		if _, exists := knownFields[k]; !exists {
			knownFields[k] = v
		}
	}
	return json.Marshal(knownFields)
}

var jevQuestionKnownFields = []string{"type", "instructions", "criteria"}

// UnmarshalJSON preserves unknown fields in ExtraFields.
func (q *JevQuestion) UnmarshalJSON(data []byte) error {
	type alias JevQuestion
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	var allFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &allFields); err != nil {
		return err
	}
	for _, key := range jevQuestionKnownFields {
		delete(allFields, key)
	}
	tmp.ExtraFields = allFields
	*q = JevQuestion(tmp)
	return nil
}

// MarshalJSON merges ExtraFields back into the output.
func (q JevQuestion) MarshalJSON() ([]byte, error) {
	type alias JevQuestion
	known, err := json.Marshal(alias(q))
	if err != nil {
		return nil, err
	}
	if len(q.ExtraFields) == 0 {
		return known, nil
	}
	var knownFields map[string]json.RawMessage
	if err := json.Unmarshal(known, &knownFields); err != nil {
		return nil, err
	}
	for k, v := range q.ExtraFields {
		if _, exists := knownFields[k]; !exists {
			knownFields[k] = v
		}
	}
	return json.Marshal(knownFields)
}

// JevResponse is the parsed view of a successful upstream /v1/systemone
// response.  It is used for usage extraction, tracing, and training-log
// capture; the raw upstream body is forwarded to the client unchanged, so no
// ExtraFields bookkeeping is needed here.
//
// Answers and Usage are required by the protocol on a success response; the
// component client (component/jev) rejects 2xx responses lacking either so a
// malformed success is never billed as a zero-usage completion.
type JevResponse struct {
	ID       string                     `json:"id"`
	Model    string                     `json:"model"`
	Provider string                     `json:"provider"`
	Answers  map[string]json.RawMessage `json:"answers"`
	Usage    *JevUsage                  `json:"usage"`
}

// JevUsage reports token consumption for a Jev call.
type JevUsage struct {
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	Cost         *float64 `json:"cost,omitempty"`
}

// TotalTokens returns the combined input and output token count.
func (u JevUsage) TotalTokens() int64 {
	return u.InputTokens + u.OutputTokens
}
