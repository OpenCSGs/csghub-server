package handler

import (
	"bytes"
	"encoding/json"

	commontypes "opencsg.com/csghub-server/common/types"

	"opencsg.com/csghub-server/aigateway/types"
)

type llmlogToolCall struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}

func llmlogMessagesToGenerationMessages(msgs []commontypes.LLMLogMessage) []types.GenerationMessage {
	if len(msgs) == 0 {
		return nil
	}
	result := make([]types.GenerationMessage, 0, len(msgs))
	for _, msg := range msgs {
		gm := convertLLMLogMessage(msg)
		if len(gm.Parts) > 0 {
			result = append(result, gm)
		}
	}
	return result
}

func convertLLMLogMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	switch msg.Role {
	case "tool_call":
		return convertToolCallMessage(msg)
	case "tool_response":
		return types.GenerationMessage{
			Role: "tool",
			Parts: []types.GenerationPart{{
				Kind: "text",
				Text: msg.Content,
			}},
		}
	default:
		return convertRegularMessage(msg)
	}
}

func convertToolCallMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	var tc llmlogToolCall
	if err := json.Unmarshal([]byte(msg.Content), &tc); err != nil {
		return types.GenerationMessage{
			Role: "assistant",
			Parts: []types.GenerationPart{{
				Kind: "text",
				Text: msg.Content,
			}},
		}
	}
	return types.GenerationMessage{
		Role: "assistant",
		Parts: []types.GenerationPart{{
			Kind: "tool_call",
			ToolCall: &types.GenerationToolCall{
				Name:      tc.Name,
				InputJSON: llmlogToolCallInputJSON(tc.Arguments),
			},
		}},
	}
}

func llmlogToolCallInputJSON(arguments any) json.RawMessage {
	switch v := arguments.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, []byte(v)); err == nil {
			return compacted.Bytes()
		}
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return data
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return data
	}
}

func convertRegularMessage(msg commontypes.LLMLogMessage) types.GenerationMessage {
	parts := make([]types.GenerationPart, 0, 2)
	if msg.ReasoningContent != "" {
		parts = append(parts, types.GenerationPart{
			Kind:     "thinking",
			Thinking: msg.ReasoningContent,
		})
	}
	if msg.Content != "" {
		parts = append(parts, types.GenerationPart{
			Kind: "text",
			Text: msg.Content,
		})
	}
	return types.GenerationMessage{
		Role:  msg.Role,
		Parts: parts,
	}
}
