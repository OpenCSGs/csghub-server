package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

// unsupportedFeature returns an error indicating a feature is not supported
// by the current adapter path.  The error message uses the prefix
// "unsupported_feature:" so that the handler can distinguish it from
// validation errors.
func unsupportedFeature(feature string) error {
	return fmt.Errorf("unsupported_feature:%s", feature)
}

// checkAdapterCapabilities inspects the request against the upstream protocol
// capabilities and returns a list of capabilities that are requested but not
// supported.  An empty list means all capabilities are satisfied.
func checkAdapterCapabilities(req *types.AnthropicMessagesRequest, cap types.ProtocolCapability) []string {
	var missing []string

	if req.HasCacheControl() && !cap.PromptCaching {
		missing = append(missing, "prompt_caching")
	}
	if req.HasThinking() && !cap.Thinking {
		missing = append(missing, "thinking")
	}
	if req.HasVision() && !cap.Vision {
		missing = append(missing, "vision")
	}
	if len(req.Tools) > 0 && !cap.Tools {
		missing = append(missing, "tools")
	}

	return missing
}

// --- Anthropic Messages → OpenAI Chat Completions conversion ---

// messagesToChatMessages converts Anthropic messages (including system prompt)
// into OpenAI Chat Completions messages.  When includeReasoning is false,
// thinking blocks in assistant history are omitted (they are not meaningful
// for upstreams that do not support reasoning).
func messagesToChatMessages(req *types.AnthropicMessagesRequest, includeReasoning bool) ([]map[string]any, error) {
	var messages []map[string]any

	// System prompt → system message
	if len(req.System) > 0 && string(req.System) != "null" {
		systemText := types.AnthropicMessageContentText(req.System)
		if systemText != "" {
			messages = append(messages, map[string]any{
				"role":    "system",
				"content": systemText,
			})
		}
	}

	for _, msg := range req.Messages {
		blocks, err := types.ParseAnthropicContentBlocks(msg.Content)
		if err != nil {
			return nil, fmt.Errorf("parse message content: %w", err)
		}

switch msg.Role {
			case "user":
				userMsgs, err := anthropicUserBlocksToChat(blocks)
				if err != nil {
					return nil, err
				}
				messages = append(messages, userMsgs...)
			case "assistant":
				asstMsg, err := anthropicAssistantBlocksToChat(blocks, includeReasoning)
				if err != nil {
					return nil, err
				}
				if asstMsg != nil {
					messages = append(messages, asstMsg)
				}
			case "system":
				// Anthropic allows system messages in the messages array (e.g.
				// Claude Code sends them).  Convert to a Chat-compatible system
				// message — only the text content is meaningful.
				text := types.AnthropicMessageContentText(msg.Content)
				if text != "" {
					messages = append(messages, map[string]any{
						"role":    "system",
						"content": text,
					})
				}
			default:
			return nil, fmt.Errorf("unsupported role: %s", msg.Role)
		}
	}

	return messages, nil
}

// anthropicUserBlocksToChat converts Anthropic user content blocks to Chat messages.
// A tool_result block becomes a separate {role: "tool"} message.
func anthropicUserBlocksToChat(blocks []types.AnthropicContentBlock) ([]map[string]any, error) {
	var toolResults []map[string]any
	var otherParts []map[string]any

	for _, block := range blocks {
		switch block.Type {
		case "text":
			otherParts = append(otherParts, map[string]any{"type": "text", "text": block.Text})
		case "image":
		 imageURL, err := anthropicImageToChatURL(block.Source)
			if err != nil {
				return nil, err
			}
			otherParts = append(otherParts, map[string]any{
				"type":      "image_url",
				"image_url": imageURL,
			})
		case "tool_result":
			resultContent := extractToolResultContent(block)
			toolResults = append(toolResults, map[string]any{
				"role":         "tool",
				"tool_call_id": block.ToolUseID,
				"content":      resultContent,
			})
		default:
			return nil, unsupportedFeature("content." + block.Type)
		}
	}

	var messages []map[string]any
	if len(otherParts) > 0 {
		// If there's only one text part, use a plain string for compatibility.
		if len(otherParts) == 1 && otherParts[0]["type"] == "text" {
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": otherParts[0]["text"],
			})
		} else {
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": otherParts,
			})
		}
	}
	messages = append(messages, toolResults...)
	return messages, nil
}

// anthropicAssistantBlocksToChat converts Anthropic assistant content blocks to
// a single Chat assistant message.  text → content, tool_use → tool_calls,
// thinking → reasoning_content (when includeReasoning is true).
func anthropicAssistantBlocksToChat(blocks []types.AnthropicContentBlock, includeReasoning bool) (map[string]any, error) {
	var textParts []string
	var toolCalls []map[string]any
	var reasoningContent string

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			argsStr := string(block.Input)
			if argsStr == "" {
				argsStr = "{}"
			}
			toolCalls = append(toolCalls, map[string]any{
				"id":   block.ID,
				"type": "function",
				"function": map[string]any{
					"name":      block.Name,
					"arguments": argsStr,
				},
			})
		case "thinking":
			if includeReasoning && block.Thinking != "" {
				if reasoningContent != "" {
					reasoningContent += "\n"
				}
				reasoningContent += block.Thinking
			}
		default:
			return nil, unsupportedFeature("content." + block.Type)
		}
	}

	msg := map[string]any{
		"role":    "assistant",
		"content": strings.Join(textParts, ""),
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	if reasoningContent != "" {
		msg["reasoning_content"] = reasoningContent
	}

	// Skip empty assistant messages.
	if msg["content"] == "" && len(toolCalls) == 0 && reasoningContent == "" {
		return nil, nil
	}
	return msg, nil
}

func anthropicImageToChatURL(source *types.AnthropicImageSource) (map[string]any, error) {
	if source == nil {
		return nil, fmt.Errorf("image source is nil")
	}
	switch source.Type {
	case "base64":
		dataURL := fmt.Sprintf("data:%s;base64,%s", source.MediaType, source.Data)
		return map[string]any{"url": dataURL}, nil
	case "url":
		return map[string]any{"url": source.URL}, nil
	default:
		return nil, fmt.Errorf("unsupported image source type: %s", source.Type)
	}
}

func extractToolResultContent(block types.AnthropicContentBlock) string {
	if len(block.Content) == 0 {
		return ""
	}
	// Try string first.
	var asString string
	if err := json.Unmarshal(block.Content, &asString); err == nil {
		return asString
	}
	// Fall back to extracting text from content blocks.
	return types.AnthropicMessageContentText(block.Content)
}

// messagesToolsToChatTools converts Anthropic tools to OpenAI Chat tools.
func messagesToolsToChatTools(tools []types.AnthropicTool) []map[string]any {
	chatTools := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		schema := tool.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		chatTool := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  json.RawMessage(schema),
			},
		}
		chatTools = append(chatTools, chatTool)
	}
	return chatTools
}

// --- OpenAI Chat → Anthropic Messages response conversion ---

// chatResponseToMessagesResponse converts an OpenAI Chat Completions response
// to an Anthropic Messages response.
func chatResponseToMessagesResponse(data []byte, model string) (*types.AnthropicMessagesResponse, error) {
	var chat types.ChatCompletion
	if err := json.Unmarshal(data, &chat); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	resp := &types.AnthropicMessagesResponse{
		ID:    newMessagesResponseID(),
		Type:  "message",
		Role:  "assistant",
		Model: model,
		Usage: types.AnthropicMessagesUsage{
			InputTokens:          int(chat.Usage.PromptTokens),
			OutputTokens:         int(chat.Usage.CompletionTokens),
			CacheReadInputTokens: int(chat.Usage.PromptTokensDetails.CachedTokens),
		},
	}

	reasoning := chatResponseReasoning(data)

	if len(chat.Choices) == 0 {
		endTurn := "end_turn"
		resp.StopReason = &endTurn
		return resp, nil
	}

	choice := chat.Choices[0]
	msg := choice.Message
	var contentBlocks []types.AnthropicContentBlock

	// Build content blocks in Anthropic's canonical order:
	// thinking → text → tool_use.
	if reasoning != "" {
		contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
			Type:     "thinking",
			Thinking: reasoning,
		})
	}

	// Text content.
	if msg.Content != "" {
		contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}

	// Tool calls.
	if len(msg.ToolCalls) > 0 {
		for _, call := range msg.ToolCalls {
			contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    call.ID,
				Name:  call.Function.Name,
				Input: json.RawMessage(call.Function.Arguments),
			})
		}
	}

	if len(contentBlocks) == 0 {
		// Ensure content is never null.
		contentBlocks = []types.AnthropicContentBlock{{Type: "text", Text: ""}}
	}
	resp.Content = contentBlocks

	stopReason := chatFinishReasonToStopReason(string(choice.FinishReason))
	resp.StopReason = &stopReason

	return resp, nil
}

// chatResponseReasoning extracts reasoning content from a raw Chat response.
func chatResponseReasoning(data []byte) string {
	var raw struct {
		Choices []struct {
			Message map[string]json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || len(raw.Choices) == 0 {
		return ""
	}
	return reasoningFromRawFields(raw.Choices[0].Message)
}

func reasoningFromRawFields(fields map[string]json.RawMessage) string {
	if len(fields) == 0 {
		return ""
	}
	for _, key := range []string{"reasoning_content", "reasoning"} {
		var value string
		if err := json.Unmarshal(fields[key], &value); err == nil {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func chatFinishReasonToStopReason(finishReason string) string {
	switch finishReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

// messagesStopReasonToFinishReason is the inverse of chatFinishReasonToStopReason,
// mapping an Anthropic stop_reason back to an OpenAI finish_reason for the
// LLM log recorder.
func messagesStopReasonToFinishReason(stopReason string) string {
	switch stopReason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop_sequence"
	default:
		return "stop"
	}
}

// newMessagesResponseID generates an Anthropic-style message ID.
func newMessagesResponseID() string {
	return fmt.Sprintf("msg_agw_%s", randomHexID())
}

// --- OpenAI Responses → Anthropic Messages response conversion ---

// responsesResponseToMessagesResponse converts an OpenAI Responses response
// to an Anthropic Messages response.
func responsesResponseToMessagesResponse(resp *types.ResponsesResponse, model string) (*types.AnthropicMessagesResponse, error) {
	result := &types.AnthropicMessagesResponse{
		ID:    newMessagesResponseID(),
		Type:  "message",
		Role:  "assistant",
		Model: model,
	}

	if resp.Usage != nil {
		usage := types.AnthropicMessagesUsage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		}
		if resp.Usage.InputTokensDetails != nil {
			usage.CacheReadInputTokens = int(resp.Usage.InputTokensDetails.CachedTokens)
			usage.CacheCreationInputTokens = int(resp.Usage.InputTokensDetails.CachedCreationTokens)
		}
		result.Usage = usage
	}

	var contentBlocks []types.AnthropicContentBlock

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
						Type: "text",
						Text: part.Text,
					})
				case "refusal":
					contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
						Type: "text",
						Text: part.Refusal,
					})
				}
			}
		case "function_call":
			inputArgs := json.RawMessage(item.Arguments)
			if len(inputArgs) == 0 {
				inputArgs = json.RawMessage(`{}`)
			}
			contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
				Type:  "tool_use",
				ID:    item.CallID,
				Name:  item.Name,
				Input: inputArgs,
			})
		case "reasoning":
			for _, summary := range item.Summary {
				if summary.Text != "" {
					contentBlocks = append(contentBlocks, types.AnthropicContentBlock{
						Type:     "thinking",
						Thinking: summary.Text,
					})
				}
			}
		}
	}

	if len(contentBlocks) == 0 {
		contentBlocks = []types.AnthropicContentBlock{{Type: "text", Text: ""}}
	}
	result.Content = contentBlocks

	stopReason := responsesStatusToStopReason(resp.Status)
	result.StopReason = &stopReason

	return result, nil
}

func responsesStatusToStopReason(status string) string {
	switch status {
	case "completed":
		return "end_turn"
	case "incomplete":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

// --- Anthropic Messages → OpenAI Responses request conversion ---

// messagesToResponsesRequest converts an Anthropic Messages request to an
// OpenAI Responses request.
func messagesToResponsesRequest(req *types.AnthropicMessagesRequest, modelName string) (*types.ResponsesRequest, error) {
	// Build input items from messages.
	inputItems, err := messagesToResponsesInput(req)
	if err != nil {
		return nil, err
	}

	inputRaw, err := json.Marshal(inputItems)
	if err != nil {
		return nil, fmt.Errorf("marshal responses input: %w", err)
	}

	respReq := &types.ResponsesRequest{
		Model:           modelName,
		Input:           inputRaw,
		Stream:          req.Stream,
		MaxOutputTokens: &req.MaxTokens,
	}
	if req.Temperature != nil {
		respReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		respReq.TopP = req.TopP
	}
	if len(req.StopSequences) > 0 {
		stopRaw, err := json.Marshal(req.StopSequences)
		if err != nil {
			return nil, fmt.Errorf("marshal stop sequences: %w", err)
		}
		if respReq.ExtraFields == nil {
			respReq.ExtraFields = make(map[string]json.RawMessage)
		}
		respReq.ExtraFields["stop"] = stopRaw
	}
	if len(req.ToolChoice) > 0 {
		converted, err := convertToolChoiceForResponses(req.ToolChoice)
		if err != nil {
			return nil, fmt.Errorf("convert tool_choice: %w", err)
		}
		respReq.ToolChoice = converted
	}
	if req.Metadata != nil && req.Metadata.UserID != "" {
		respReq.User = req.Metadata.UserID
	}

	// System prompt → instructions.
	if len(req.System) > 0 && string(req.System) != "null" {
		systemText := types.AnthropicMessageContentText(req.System)
		if systemText != "" {
			respReq.Instructions = json.RawMessage(jsonString(systemText))
		}
	}

	// Tools conversion.
	if len(req.Tools) > 0 {
		respTools := messagesToolsToResponsesTools(req.Tools)
		toolsRaw, err := json.Marshal(respTools)
		if err != nil {
			return nil, fmt.Errorf("marshal responses tools: %w", err)
		}
		respReq.Tools = toolsRaw
	}

	// Thinking → reasoning.
	if req.HasThinking() {
		effort := "medium"
		if req.Thinking.BudgetTokens > 0 {
			effort = budgetToEffort(req.Thinking.BudgetTokens)
		}
		respReq.Reasoning = json.RawMessage(fmt.Sprintf(`{"effort":%q}`, effort))
	}

	return respReq, nil
}

// messagesToResponsesInput converts Anthropic messages to Responses API input items.
func messagesToResponsesInput(req *types.AnthropicMessagesRequest) ([]map[string]any, error) {
	var items []map[string]any

	for _, msg := range req.Messages {
		blocks, err := types.ParseAnthropicContentBlocks(msg.Content)
		if err != nil {
			return nil, fmt.Errorf("parse message content: %w", err)
		}

		switch msg.Role {
		case "user":
			// Group text and image blocks into a single message with multipart
			// content, then append tool_result blocks as separate items.
			var contentParts []map[string]any
			for _, block := range blocks {
				switch block.Type {
				case "text":
					contentParts = append(contentParts, map[string]any{
						"type": "input_text",
						"text": block.Text,
					})
				case "image":
					if block.Source != nil {
						switch block.Source.Type {
						case "base64":
							contentParts = append(contentParts, map[string]any{
								"type":      "input_image",
								"image_url": fmt.Sprintf("data:%s;base64,%s",
									block.Source.MediaType, block.Source.Data),
							})
						case "url":
							contentParts = append(contentParts, map[string]any{
								"type":      "input_image",
								"image_url": block.Source.URL,
							})
						}
					}
				case "tool_result":
					outputContent := extractToolResultContent(block)
					items = append(items, map[string]any{
						"type":    "function_call_output",
						"call_id": block.ToolUseID,
						"output":  outputContent,
					})
				}
			}
			if len(contentParts) > 0 {
				items = append(items, map[string]any{
					"type":    "message",
					"role":    "user",
					"content": contentParts,
				})
			}
		case "assistant":
			for _, block := range blocks {
				switch block.Type {
				case "text":
					items = append(items, map[string]any{
						"type": "message",
						"role": "assistant",
						"content": []map[string]any{{
							"type": "output_text",
							"text": block.Text,
						}},
					})
				case "tool_use":
					argsStr := string(block.Input)
					if argsStr == "" {
						argsStr = "{}"
					}
					items = append(items, map[string]any{
						"type":      "function_call",
						"call_id":   block.ID,
						"name":      block.Name,
						"arguments": argsStr,
					})
				case "thinking":
					if block.Thinking != "" {
						items = append(items, map[string]any{
							"type": "reasoning",
							"summary": []map[string]any{{
								"type": "summary_text",
								"text": block.Thinking,
							}},
						})
					}
				}
			}
		}
	}

	return items, nil
}

// messagesToolsToResponsesTools converts Anthropic tools to Responses API tools.
func messagesToolsToResponsesTools(tools []types.AnthropicTool) []map[string]any {
	respTools := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		schema := tool.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		respTool := map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  json.RawMessage(schema),
		}
		respTools = append(respTools, respTool)
	}
	return respTools
}

func budgetToEffort(budget int) string {
	switch {
	case budget <= 5000:
		return "low"
	case budget <= 20000:
		return "medium"
	default:
		return "high"
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// convertToolChoiceForResponses converts an Anthropic tool_choice value to
// the format expected by the OpenAI Responses API.
// Anthropic: {"type":"auto"}, {"type":"any"}, {"type":"tool","name":"..."}
// Responses: "auto", "required", {"type":"function","name":"..."}
func convertToolChoiceForResponses(raw json.RawMessage) (json.RawMessage, error) {
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, fmt.Errorf("invalid tool_choice: %w", err)
	}
	switch tc.Type {
	case "auto":
		return json.RawMessage(`"auto"`), nil
	case "any":
		return json.RawMessage(`"required"`), nil
	case "tool":
		return json.RawMessage(fmt.Sprintf(`{"type":"function","name":%s}`, jsonString(tc.Name))), nil
	default:
		return json.RawMessage(`"auto"`), nil
	}
}

// convertToolChoiceForChat converts an Anthropic tool_choice value to the
// format expected by the OpenAI Chat Completions API.
// Anthropic: {"type":"auto"}, {"type":"any"}, {"type":"tool","name":"..."}
// Chat:      "auto", "required", {"type":"function","function":{"name":"..."}}
func convertToolChoiceForChat(raw json.RawMessage) (json.RawMessage, error) {
	var tc struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tc); err != nil {
		return nil, fmt.Errorf("invalid tool_choice: %w", err)
	}
	switch tc.Type {
	case "auto":
		return json.RawMessage(`"auto"`), nil
	case "any":
		return json.RawMessage(`"required"`), nil
	case "tool":
		return json.RawMessage(fmt.Sprintf(`{"type":"function","function":{"name":%s}}`, jsonString(tc.Name))), nil
	default:
		return json.RawMessage(`"auto"`), nil
	}
}
