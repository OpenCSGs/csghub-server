package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
)

func validateResponsesAdapterRequest(req *types.ResponsesRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	if req.PreviousResponseID != "" {
		return unsupportedResponsesFeature("previous_response_id")
	}
	if isTrue(req.Store) {
		return unsupportedResponsesFeature("store")
	}
	if isTrue(req.Background) {
		return unsupportedResponsesFeature("background")
	}
	if len(req.Conversation) > 0 {
		return unsupportedResponsesFeature("conversation")
	}
	if len(req.Prompt) > 0 {
		return unsupportedResponsesFeature("prompt")
	}
	if req.MaxToolCalls != nil {
		return unsupportedResponsesFeature("max_tool_calls")
	}
	if len(req.Reasoning) > 0 {
		return unsupportedResponsesFeature("reasoning")
	}
	for _, tool := range splitRawJSONArray(req.Tools) {
		var obj map[string]any
		if err := json.Unmarshal(tool, &obj); err == nil {
			if toolType, _ := obj["type"].(string); toolType != "" && toolType != "function" {
				return unsupportedResponsesFeature("tools." + toolType)
			}
		}
	}
	return nil
}

func unsupportedResponsesFeature(field string) error {
	return fmt.Errorf("unsupported_feature:%s", field)
}

func isTrue(v *bool) bool {
	return v != nil && *v
}

func responsesToChatRequest(req *types.ResponsesRequest, modelName string) (*ChatCompletionRequest, error) {
	messages, err := responsesInputToChatMessages(req)
	if err != nil {
		return nil, err
	}
	rawMessages, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	var sdkMessages []openai.ChatCompletionMessageParamUnion
	if err := json.Unmarshal(rawMessages, &sdkMessages); err != nil {
		return nil, fmt.Errorf("convert responses input to chat messages: %w", err)
	}

	chatReq := &ChatCompletionRequest{
		Model:       modelName,
		Messages:    sdkMessages,
		Stream:      req.Stream,
		Temperature: floatPtrValue(req.Temperature),
		TopP:        floatPtrValue(req.TopP),
	}
	if req.MaxOutputTokens != nil {
		chatReq.MaxTokens = *req.MaxOutputTokens
	}
	if len(req.Tools) > 0 {
		chatTools, err := responsesToolsToChatTools(req.Tools)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(chatTools, &chatReq.Tools); err != nil {
			return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
		}
	}
	if len(req.ToolChoice) > 0 {
		if err := json.Unmarshal(req.ToolChoice, &chatReq.ToolChoice); err != nil {
			return nil, fmt.Errorf("convert responses tool_choice to chat tool_choice: %w", err)
		}
	}
	parallel := true
	if req.ParallelToolCalls != nil {
		parallel = *req.ParallelToolCalls
	}
	mergeChatRawJSON(chatReq, "parallel_tool_calls", parallel)
	if len(req.Text) > 0 {
		var textObj map[string]json.RawMessage
		if err := json.Unmarshal(req.Text, &textObj); err == nil {
			if format, ok := textObj["format"]; ok {
				mergeChatRawJSONRaw(chatReq, "response_format", format)
			}
		}
	}
	return chatReq, nil
}

func mergeChatRawJSON(chatReq *ChatCompletionRequest, key string, value any) {
	rawValue, err := json.Marshal(value)
	if err != nil {
		return
	}
	mergeChatRawJSONRaw(chatReq, key, rawValue)
}

func mergeChatRawJSONRaw(chatReq *ChatCompletionRequest, key string, value json.RawMessage) {
	if chatReq == nil || key == "" {
		return
	}
	raw := map[string]json.RawMessage{}
	if len(chatReq.RawJSON) > 0 {
		_ = json.Unmarshal(chatReq.RawJSON, &raw)
	}
	raw[key] = value
	chatReq.RawJSON, _ = json.Marshal(raw)
}

func responsesToolsToChatTools(raw json.RawMessage) (json.RawMessage, error) {
	tools := splitRawJSONArray(raw)
	if len(tools) == 0 {
		return raw, nil
	}
	chatTools := make([]map[string]json.RawMessage, 0, len(tools))
	for _, tool := range tools {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(tool, &obj); err != nil {
			return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
		}
		var toolType string
		_ = json.Unmarshal(obj["type"], &toolType)
		if toolType != "" && toolType != "function" {
			return nil, unsupportedResponsesFeature("tools." + toolType)
		}
		function := map[string]json.RawMessage{}
		if rawFunction, ok := obj["function"]; ok {
			if err := json.Unmarshal(rawFunction, &function); err != nil || function == nil {
				return nil, fmt.Errorf("convert responses function tool: function must be an object")
			}
		} else {
			for _, key := range []string{"name", "description", "parameters", "strict"} {
				if value, ok := obj[key]; ok {
					function[key] = value
				}
			}
		}
		functionRaw, err := json.Marshal(function)
		if err != nil {
			return nil, fmt.Errorf("convert responses function tool: %w", err)
		}
		chatTool := map[string]json.RawMessage{
			"type":     json.RawMessage(`"function"`),
			"function": functionRaw,
		}
		chatTools = append(chatTools, chatTool)
	}
	data, err := json.Marshal(chatTools)
	if err != nil {
		return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
	}
	return data, nil
}

func floatPtrValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func responsesInputToChatMessages(req *types.ResponsesRequest) ([]map[string]any, error) {
	messages := []map[string]any{}
	if instructionText := responsesInstructionText(req.Instructions); instructionText != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructionText})
	}
	var asString string
	if err := json.Unmarshal(req.Input, &asString); err == nil {
		messages = append(messages, map[string]any{"role": "user", "content": asString})
		return messages, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(req.Input, &items); err != nil {
		return nil, fmt.Errorf("unsupported responses input shape")
	}
	for _, item := range items {
		itemType, _ := item["type"].(string)
		switch itemType {
		case "message", "":
			role, _ := item["role"].(string)
			if role == "" {
				role = "user"
			}
			content, err := normalizeResponsesContent(item["content"])
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{"role": role, "content": content})
		case "function_call":
			messages = append(messages, map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id":   item["call_id"],
					"type": "function",
					"function": map[string]any{
						"name":      item["name"],
						"arguments": item["arguments"],
					},
				}},
			})
		case "function_call_output":
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": item["call_id"],
				"content":      item["output"],
			})
		default:
			return nil, unsupportedResponsesFeature("input." + itemType)
		}
	}
	return messages, nil
}

func splitRawJSONArray(raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var items []json.RawMessage
	_ = json.Unmarshal(raw, &items)
	return items
}

func responsesInstructionText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func normalizeResponsesContent(content any) (any, error) {
	parts, ok := content.([]any)
	if !ok {
		return content, nil
	}
	chatParts := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		obj, ok := part.(map[string]any)
		if !ok {
			continue
		}
		switch obj["type"] {
		case "input_text", "output_text", "text":
			chatParts = append(chatParts, map[string]any{"type": "text", "text": obj["text"]})
		case "input_image", "image_url":
			chatParts = append(chatParts, map[string]any{"type": "image_url", "image_url": obj["image_url"]})
		default:
			partType, _ := obj["type"].(string)
			if partType == "" {
				partType = "unknown"
			}
			return nil, unsupportedResponsesFeature("input.content." + partType)
		}
	}
	return chatParts, nil
}

func chatResponseToResponses(data []byte, publicModel string) (*types.ResponsesResponse, error) {
	var chat types.ChatCompletion
	if err := json.Unmarshal(data, &chat); err != nil {
		return nil, err
	}
	// TODO: If adapter mode later supports previous_response_id, pass the
	// public previous ID into this conversion and echo it in ResponsesResponse.
	resp := &types.ResponsesResponse{
		ID:        newAdapterResponseID(),
		Object:    "response",
		CreatedAt: chat.Created,
		Status:    "completed",
		Model:     publicModel,
		Usage: &types.ResponsesUsage{
			InputTokens:  chat.Usage.PromptTokens,
			OutputTokens: chat.Usage.CompletionTokens,
			TotalTokens:  chat.Usage.TotalTokens,
		},
	}
	if resp.CreatedAt == 0 {
		resp.CreatedAt = time.Now().Unix()
	}
	if len(chat.Choices) == 0 {
		return resp, nil
	}
	msg := chat.Choices[0].Message
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			resp.Output = append(resp.Output, types.ResponsesOutputItem{
				ID:        call.ID,
				Type:      "function_call",
				Status:    "completed",
				CallID:    call.ID,
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			})
		}
		return resp, nil
	}
	if msg.Refusal != "" {
		resp.Output = append(resp.Output, types.ResponsesOutputItem{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type:    "refusal",
				Refusal: msg.Refusal,
			}},
		})
		return resp, nil
	}
	text := msg.Content
	resp.OutputText = text
	resp.Output = append(resp.Output, types.ResponsesOutputItem{
		Type:   "message",
		Status: "completed",
		Role:   "assistant",
		Content: []types.ResponsesContentPart{{
			Type: "output_text",
			Text: text,
		}},
	})
	return resp, nil
}

