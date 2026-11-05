package types

import "encoding/json"

// GenerationConfig carries submitted facts, not effective provider defaults.
type GenerationConfig struct {
	SchemaVersion   string          `json:"schema_version"`
	Protocol        string          `json:"protocol"`
	ValueStage      string          `json:"value_stage"`
	Parameters      json.RawMessage `json:"parameters"`
	ContextControls json.RawMessage `json:"context_controls"`
}
