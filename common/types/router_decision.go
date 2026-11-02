package types

import (
	"encoding/json"
	"unicode/utf8"
)

// RouterDecision is an immutable ranking decision, independent of the model
// that eventually serves an attempt after an availability change or fallback.
type RouterDecision struct {
	Version    string `json:"version"`
	DecisionID string `json:"decision_id"`
	Selected   bool   `json:"selected"`
	// TargetModelID is the Gateway model ID (Router policy requested_model),
	// not the benchmark candidate ID. See semantic_model/router_service/app.py
	// model_list and exploration/ledger.py commit metadata (Router MR !31).
	TargetModelID   string `json:"target_model_id"`
	CandidateDigest string `json:"candidate_digest"`
	CountsVersion   string `json:"counts_version"`
}

// Clone returns nil for unknown or malformed optional metadata.
func (d *RouterDecision) Clone() *RouterDecision {
	if d == nil || d.Version != "simple-exploration-decision/1" || d.DecisionID == "" || utf8.RuneCountInString(d.DecisionID) > 256 ||
		utf8.RuneCountInString(d.TargetModelID) > 256 || utf8.RuneCountInString(d.CandidateDigest) > 256 || utf8.RuneCountInString(d.CountsVersion) > 256 ||
		(d.Selected && d.TargetModelID == "") {
		return nil
	}
	copy := *d
	return &copy
}

// UnmarshalJSON preserves the ranking when its optional audit is malformed.
// A missing/null selected flag is unknown, rather than a false decision.
func (d *RouterDecision) UnmarshalJSON(data []byte) error {
	type plain RouterDecision
	var wire struct {
		plain
		Selected *bool `json:"selected"`
	}
	*d = RouterDecision{}
	if len(data) > 8192 || json.Unmarshal(data, &wire) != nil || wire.Selected == nil {
		return nil
	}
	candidate := RouterDecision(wire.plain)
	candidate.Selected = *wire.Selected
	if valid := candidate.Clone(); valid != nil {
		*d = *valid
	}
	return nil
}
