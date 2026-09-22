package types

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ModelRequiresToolCallReasoningContent reports whether the upstream model
// family enforces DeepSeek's thinking-mode contract: when the request carries
// the tools parameter, assistant messages that contain tool_calls must include
// the reasoning_content field, otherwise the API rejects the request with
// "The `reasoning_content` in the thinking mode must be passed back to the
// API." (https://api-docs.deepseek.com/guides/thinking_mode). Kimi/Moonshot
// models expose the same contract.
func ModelRequiresToolCallReasoningContent(modelName string) bool {
	name := strings.ToLower(modelName)
	return strings.Contains(name, "deepseek") || strings.Contains(name, "kimi") || strings.Contains(name, "moonshot")
}

// InjectToolCallReasoningContent sets `reasoning_content` to an empty string
// on assistant messages that carry tool calls but no reasoning content. The
// upstream only validates the presence of the field, so an empty value
// satisfies the contract without fabricating reasoning output. The body is
// returned unchanged when nothing needs to be injected.
func InjectToolCallReasoningContent(body []byte) []byte {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	messagesRaw, ok := payload["messages"]
	if !ok {
		return body
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesRaw, &messages); err != nil {
		return body
	}
	changed := false
	for i, msgRaw := range messages {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		if !isAssistantToolCallMessage(msg) {
			continue
		}
		// Treat an existing value as present only when it is a JSON string:
		// some SDKs send an explicit null instead of omitting the key, and
		// the upstream requires a string field to consider reasoning passed
		// back. null or non-string values are overwritten with "".
		if rc, exists := msg["reasoning_content"]; exists && isJSONString(rc) {
			continue
		}
		msg["reasoning_content"] = json.RawMessage(`""`)
		updated, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		messages[i] = updated
		changed = true
	}
	if !changed {
		return body
	}
	mergedMessages, err := json.Marshal(messages)
	if err != nil {
		return body
	}
	payload["messages"] = mergedMessages
	merged, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return merged
}

// isJSONString reports whether the raw JSON value is a string literal.
// JSON null and non-string values do not count.
func isJSONString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"'
}

// IsJSONNull reports whether the raw JSON value is a literal null.
func IsJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

// isAssistantToolCallMessage reports whether the message is an assistant
// message that actually carries tool calls (a `tool_calls: null` value does
// not count).
func isAssistantToolCallMessage(msg map[string]json.RawMessage) bool {
	var role string
	if roleRaw, ok := msg["role"]; ok {
		_ = json.Unmarshal(roleRaw, &role)
	}
	if role != "assistant" {
		return false
	}
	var toolCalls []json.RawMessage
	if toolCallsRaw, ok := msg["tool_calls"]; ok {
		_ = json.Unmarshal(toolCallsRaw, &toolCalls)
	}
	return len(toolCalls) > 0
}
