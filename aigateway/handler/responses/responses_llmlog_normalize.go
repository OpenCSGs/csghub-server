package responses

import (
	"bytes"
	"encoding/json"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func normalizeResponsesInputMessages(req *types.ResponsesRequest) ([]commontypes.LLMLogMessage, error) {
	var messages []commontypes.LLMLogMessage
	if text := responsesLLMLogRawText(req.Instructions); strings.TrimSpace(text) != "" {
		messages = append(messages, commontypes.LLMLogMessage{Role: "system", Content: text})
	}
	inputMessages, err := normalizeResponsesInputRaw(req.Input)
	if err != nil {
		return nil, err
	}
	messages = append(messages, inputMessages...)
	return messages, nil
}

func normalizeResponsesInputRaw(raw json.RawMessage) ([]commontypes.LLMLogMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []commontypes.LLMLogMessage{{Role: "user", Content: text}}, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return []commontypes.LLMLogMessage{{Role: "user", Content: strings.TrimSpace(string(raw))}}, nil
	}
	messages := make([]commontypes.LLMLogMessage, 0, len(items))
	for _, item := range items {
		msgs, err := normalizeResponsesInputItem(item)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msgs...)
	}
	return messages, nil
}

func normalizeResponsesInputItem(item map[string]any) ([]commontypes.LLMLogMessage, error) {
	switch stringField(item, "type") {
	case "function_call":
		msg, err := newLLMLogToolCallMessage(stringField(item, "name"), stringifyResponsesValue(item["arguments"]))
		if err != nil {
			return nil, err
		}
		return []commontypes.LLMLogMessage{msg}, nil
	case "function_call_output":
		content := stringifyResponsesValue(item["output"])
		if strings.TrimSpace(content) == "" {
			return nil, nil
		}
		return []commontypes.LLMLogMessage{{Role: "tool_response", Content: compactJSONString(content)}}, nil
	default:
		role := stringField(item, "role")
		if role == "" {
			role = "user"
		}
		content := responsesLLMLogContentText(item["content"])
		if strings.TrimSpace(content) == "" {
			return nil, nil
		}
		return []commontypes.LLMLogMessage{{Role: role, Content: content}}, nil
	}
}

func normalizeResponsesOutputMessages(output []types.ResponsesOutputItem, outputText string) []commontypes.LLMLogMessage {
	messages := make([]commontypes.LLMLogMessage, 0, len(output)+1)
	for _, item := range output {
		if item.Type == "function_call" || item.Name != "" || item.Arguments != "" {
			msg, err := newLLMLogToolCallMessage(item.Name, item.Arguments)
			if err == nil {
				messages = append(messages, msg)
			}
			continue
		}
		var content strings.Builder
		var reasoning strings.Builder
		for _, part := range item.Content {
			switch part.Type {
			case "output_text", "text":
				content.WriteString(part.Text)
			case "refusal":
				content.WriteString(part.Refusal)
			case "reasoning":
				reasoning.WriteString(part.Text)
			}
		}
		if strings.TrimSpace(content.String()) != "" || strings.TrimSpace(reasoning.String()) != "" {
			role := item.Role
			if role == "" {
				role = "assistant"
			}
			messages = append(messages, commontypes.LLMLogMessage{
				Role:             role,
				Content:          content.String(),
				ReasoningContent: reasoning.String(),
			})
		}
	}
	if len(messages) == 0 && strings.TrimSpace(outputText) != "" {
		messages = append(messages, commontypes.LLMLogMessage{Role: "assistant", Content: outputText})
	}
	return messages
}

func newLLMLogToolCallMessage(name, arguments string) (commontypes.LLMLogMessage, error) {
	if arguments == "" {
		arguments = "{}"
	}
	payload, err := json.Marshal(map[string]any{
		"name":      name,
		"arguments": stringifyResponsesArguments(arguments),
	})
	if err != nil {
		return commontypes.LLMLogMessage{}, err
	}
	return commontypes.LLMLogMessage{Role: "tool_call", Content: string(payload)}, nil
}

func stringifyResponsesArguments(arguments string) string {
	if arguments == "" {
		return "{}"
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(arguments)); err == nil {
		return compacted.String()
	}
	return arguments
}

func responsesLLMLogContentText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			if part, ok := item.(map[string]any); ok {
				switch stringField(part, "type") {
				case "input_text", "output_text", "text":
					b.WriteString(stringField(part, "text"))
				case "refusal":
					b.WriteString(stringField(part, "refusal"))
				default:
					if text := stringField(part, "text"); text != "" {
						b.WriteString(text)
					}
				}
			} else {
				b.WriteString(stringifyResponsesValue(item))
			}
		}
		return b.String()
	case map[string]any:
		return responsesLLMLogRawTextFromMap(v)
	default:
		return stringifyResponsesValue(v)
	}
}

func responsesLLMLogRawText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return responsesLLMLogContentText(value)
}

func responsesLLMLogRawTextFromMap(value map[string]any) string {
	for _, key := range []string{"text", "output", "arguments", "content"} {
		if child, ok := value[key]; ok {
			if text := responsesLLMLogContentText(child); text != "" {
				return text
			}
		}
	}
	return stringifyResponsesValue(value)
}

func stringifyResponsesValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, data); err == nil {
			return compacted.String()
		}
		return string(data)
	}
}

func compactJSONString(value string) string {
	if value == "" {
		return value
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(value)); err == nil {
		return compacted.String()
	}
	return value
}
