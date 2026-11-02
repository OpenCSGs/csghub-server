package pii

import (
	"context"
	"encoding/json"
)

// PIIRuleRow is one DB-managed detection rule as the PII check consumes it.
// The component owns this shape so the CE build has no dependency on the
// EE-gated database layer: the EE wiring adapts the store onto this
// interface.
type PIIRuleRow struct {
	Name        string
	Type        string
	Method      string
	Pattern     string
	Replacement string
	Action      string
	Params      json.RawMessage
	Enabled     bool
}

// PIIRuleSource provides the rules the PII check masks against. Rows merge
// over the compiled-in defaults by name: a disabled row switches a
// built-in rule off, an unknown name adds a new rule.
type PIIRuleSource interface {
	List(ctx context.Context) ([]PIIRuleRow, error)
}
