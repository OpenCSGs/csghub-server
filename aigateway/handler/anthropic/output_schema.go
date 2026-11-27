package anthropic

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"opencsg.com/csghub-server/aigateway/types"
)

// outputSchemaMismatch is the error returned to the client when the upstream
// reply does not satisfy output_config.format.
const outputSchemaMismatch = "upstream response does not match output_config.format schema"

// outputSchemaMismatchBody carries the same message to the stream error writer.
const outputSchemaMismatchBody = `{"error":{"message":"` + outputSchemaMismatch + `"}}`

// outputSchema is the structured output contract of one request.  The upstream
// may not enforce it, so the gateway checks completed replies itself.
type outputSchema struct {
	resolved *jsonschema.Resolved
}

// newOutputSchema compiles the requested schema.  It returns nil when the
// request asks for no structured output.
func newOutputSchema(req *types.AnthropicMessagesRequest) (*outputSchema, error) {
	raw := parseOutputConfig(req).jsonSchema()
	if raw == nil {
		return nil, nil
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("invalid output_config.format schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("invalid output_config.format schema: %w", err)
	}
	return &outputSchema{resolved: resolved}, nil
}

// strictSchema reports whether the schema can run in OpenAI strict mode.
func strictSchema(raw json.RawMessage) bool {
	var node map[string]any
	return json.Unmarshal(raw, &node) == nil && node["type"] == "object" && strictCompatible(node)
}

// strictCompatible reports whether a schema node stays within the subset OpenAI
// strict mode accepts: every object is closed and requires all its properties.
// Unlisted keywords count as incompatible.
func strictCompatible(node map[string]any) bool {
	for key, value := range node {
		switch key {
		case "type", "required", "enum", "const", "description", "title", "$ref":
		case "additionalProperties":
			if value != false {
				return false
			}
		case "items":
			child, ok := value.(map[string]any)
			if !ok || !strictCompatible(child) {
				return false
			}
		case "properties", "$defs", "definitions":
			children, ok := value.(map[string]any)
			if !ok {
				return false
			}
			for _, c := range children {
				child, ok := c.(map[string]any)
				if !ok || !strictCompatible(child) {
					return false
				}
			}
		case "anyOf":
			children, ok := value.([]any)
			if !ok {
				return false
			}
			for _, c := range children {
				child, ok := c.(map[string]any)
				if !ok || !strictCompatible(child) {
					return false
				}
			}
		default:
			return false
		}
	}

	kinds, _ := node["type"].([]any)
	if t, ok := node["type"].(string); ok {
		kinds = []any{t}
	}
	if slices.Contains(kinds, any("array")) && node["items"] == nil {
		return false
	}
	properties, hasProperties := node["properties"].(map[string]any)
	if !hasProperties && !slices.Contains(kinds, any("object")) {
		// A node that constrains nothing is rejected by strict mode.
		return node["type"] != nil || node["anyOf"] != nil || node["$ref"] != nil ||
			node["enum"] != nil || node["const"] != nil
	}
	if node["additionalProperties"] != false {
		return false
	}
	required, _ := node["required"].([]any)
	for name := range properties {
		if !slices.Contains(required, any(name)) {
			return false
		}
	}
	return true
}

// accepts reports whether a reply may be returned as a success.  Only completed
// turns are checked: truncated, refused and tool-call turns are reported as such
// by their stop reason.  A nil schema accepts everything.
func (s *outputSchema) accepts(stopReason, text string) bool {
	if s == nil || stopReason != "end_turn" {
		return true
	}
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &value); err != nil {
		return false
	}
	return s.resolved.Validate(value) == nil
}

// replyText joins the text blocks of a converted reply.
func replyText(blocks []types.AnthropicContentBlock) string {
	var text strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

// stopReasonOf dereferences an optional stop_reason.
func stopReasonOf(reason *string) string {
	if reason == nil {
		return ""
	}
	return *reason
}
