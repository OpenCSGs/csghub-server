package types

import (
	"bytes"
	"encoding/json"
	"strings"
)

func ResponsesPromptText(req *ResponsesRequest) string {
	if req == nil {
		return ""
	}
	var b strings.Builder
	if sys := ResponsesInstructionText(req.Instructions); sys != "" {
		b.WriteString(sys)
		b.WriteByte('\n')
	}
	b.WriteString(ResponsesInputText(req.Input))
	return b.String()
}

func ResponsesInstructionText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

func ResponsesInputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return strings.TrimSpace(string(raw))
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString(responsesItemText(item))
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func responsesItemText(item map[string]any) string {
	var b strings.Builder
	switch item["type"] {
	case "function_call":
		writeAnyResponsesText(&b, item["name"])
		writeAnyResponsesText(&b, item["arguments"])
	case "function_call_output":
		writeAnyResponsesText(&b, item["output"])
	default:
		writeAnyResponsesText(&b, item["content"])
	}
	return b.String()
}

func writeAnyResponsesText(b *strings.Builder, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		b.WriteString(v)
	case []any:
		for _, item := range v {
			writeAnyResponsesText(b, item)
		}
	case map[string]any:
		if text, _ := v["text"].(string); text != "" {
			b.WriteString(text)
			return
		}
		for _, key := range []string{"output", "arguments", "content"} {
			if child, ok := v[key]; ok {
				writeAnyResponsesText(b, child)
			}
		}
	default:
		data, err := json.Marshal(v)
		if err == nil {
			var compact bytes.Buffer
			if json.Compact(&compact, data) == nil {
				b.Write(compact.Bytes())
			}
		}
	}
}
