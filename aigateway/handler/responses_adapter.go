package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	return nil
}

func unsupportedResponsesFeature(field string) error {
	return fmt.Errorf("unsupported_feature:%s", field)
}

func isTrue(v *bool) bool {
	return v != nil && *v
}

// normalizeChatRole maps OpenAI Responses-style roles to chat completions roles.
// The developer role is used by OpenAI Responses-style inputs. Many Chat
// Completions-compatible upstreams only accept system/user/assistant/tool,
// so developer is downgraded to system.
func normalizeChatRole(role string) string {
	switch role {
	case "":
		return "user"
	case "developer":
		return "system"
	default:
		return role
	}
}

func responsesToChatRequest(ctx context.Context, req *types.ResponsesRequest, modelName string, upstreamMetadata map[string]any) (*ChatCompletionRequest, error) {
	messages, err := responsesInputToChatMessages(ctx, req)
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
	mergeChatRawJSONRaw(chatReq, "messages", rawMessages)
	if req.MaxOutputTokens != nil {
		chatReq.MaxTokens = *req.MaxOutputTokens
	}
	allowedFunctionTools := map[string]struct{}{}
	if len(req.Tools) > 0 {
		chatTools, functionTools, err := responsesToolsToChatTools(ctx, req.Tools, modelName)
		if err != nil {
			return nil, err
		}
		allowedFunctionTools = functionTools
		if len(chatTools) > 0 {
			if err := json.Unmarshal(chatTools, &chatReq.Tools); err != nil {
				return nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
			}
		}
	}
	if len(req.ToolChoice) > 0 {
		if !json.Valid(req.ToolChoice) {
			return nil, fmt.Errorf("invalid responses tool_choice")
		}
		chatToolChoice := responsesToolChoiceToChatToolChoice(ctx, req.ToolChoice, allowedFunctionTools, modelName)
		if len(chatToolChoice) > 0 {
			if err := json.Unmarshal(chatToolChoice, &chatReq.ToolChoice); err != nil {
				return nil, fmt.Errorf("convert responses tool_choice to chat tool_choice: %w", err)
			}
			mergeChatRawJSONRaw(chatReq, "tool_choice", chatToolChoice)
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
	if len(req.Reasoning) > 0 {
		cfg := loadReasoningRequestConfig(upstreamMetadata)
		if err := applyAdapterReasoningRequest(ctx, chatReq, cfg, req.Reasoning); err != nil {
			return nil, err
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

var knownReasoningEfforts = map[string]string{
	"none":    "none",
	"low":     "low",
	"medium":  "medium",
	"high":    "high",
	"minimal": "minimal",
	// xhigh is a Responses API effort value not widely supported by chat upstreams;
	// normalize to max, the highest effort level those upstreams typically accept.
	"xhigh": "max",
	"max":   "max",
}

type adapterReasoningRequestConfig struct {
	Enabled      bool            `json:"enabled"`
	EffortField  string          `json:"effort_field"`
	EnableExtra  json.RawMessage `json:"enable_extra"`
	DisableExtra json.RawMessage `json:"disable_extra"`
}

func loadReasoningRequestConfig(metadata map[string]any) *adapterReasoningRequestConfig {
	if len(metadata) == 0 {
		return nil
	}
	responses, ok := metadata["responses"].(map[string]any)
	if !ok {
		return nil
	}
	chatAdapter, ok := responses["chat_adapter"].(map[string]any)
	if !ok {
		return nil
	}
	reasoningRequest, ok := chatAdapter["reasoning_request"].(map[string]any)
	if !ok {
		return nil
	}
	raw, err := json.Marshal(reasoningRequest)
	if err != nil {
		return nil
	}
	cfg := &adapterReasoningRequestConfig{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil
	}
	return cfg
}

func applyAdapterReasoningRequest(ctx context.Context, chatReq *ChatCompletionRequest, cfg *adapterReasoningRequestConfig, rawReasoning json.RawMessage) error {
	if cfg == nil {
		return nil
	}
	effort := parseReasoningEffort(rawReasoning)
	if effort == "" {
		return nil
	}
	normalized, known := normalizeEffort(effort)
	if !known {
		slog.WarnContext(ctx, "reject unknown reasoning effort for chat adapter",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", effort))
		return fmt.Errorf("invalid reasoning effort: %q", effort)
	}
	if !cfg.Enabled {
		if normalized == "none" {
			slog.InfoContext(ctx, "drop reasoning request for disabled upstream",
				slog.String("api", "/v1/responses"),
				slog.String("adapter", "chat_completions"),
				slog.String("effort", normalized))
			return nil
		}
		slog.WarnContext(ctx, "reject reasoning request for disabled upstream",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", normalized))
		return unsupportedResponsesFeature("reasoning")
	}
	if normalized == "none" {
		if err := mergeChatRawJSONObject(chatReq, cfg.DisableExtra); err != nil {
			return err
		}
		slog.InfoContext(ctx, "merge chat adapter reasoning disable_extra",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("effort", normalized))
		return nil
	}
	if cfg.EffortField != "" {
		mergeChatRawJSON(chatReq, cfg.EffortField, normalized)
	}
	if err := mergeChatRawJSONObject(chatReq, cfg.EnableExtra); err != nil {
		return err
	}
	slog.InfoContext(ctx, "merge chat adapter reasoning enable fields",
		slog.String("api", "/v1/responses"),
		slog.String("adapter", "chat_completions"),
		slog.String("effort", normalized),
		slog.String("effort_field", cfg.EffortField))
	return nil
}

func parseReasoningEffort(rawReasoning json.RawMessage) string {
	if len(rawReasoning) == 0 {
		return ""
	}
	var reasoning struct {
		Effort string `json:"effort"`
	}
	if err := json.Unmarshal(rawReasoning, &reasoning); err != nil {
		return ""
	}
	return strings.TrimSpace(reasoning.Effort)
}

func normalizeEffort(effort string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	if normalized == "" {
		return "", false
	}
	mapped, ok := knownReasoningEfforts[normalized]
	if !ok {
		return "", false
	}
	return mapped, true
}

func mergeChatRawJSONObject(chatReq *ChatCompletionRequest, extra json.RawMessage) error {
	if len(extra) == 0 || string(extra) == "null" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(extra, &obj); err != nil {
		return fmt.Errorf("invalid reasoning extra json: %w", err)
	}
	for key, value := range obj {
		mergeChatRawJSONRaw(chatReq, key, value)
	}
	return nil
}

func responsesToolsToChatTools(ctx context.Context, raw json.RawMessage, modelName string) (json.RawMessage, map[string]struct{}, error) {
	tools := splitRawJSONArray(raw)
	if len(tools) == 0 {
		return raw, nil, nil
	}
	chatTools := make([]map[string]json.RawMessage, 0, len(tools))
	functionTools := map[string]struct{}{}
	for _, tool := range tools {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(tool, &obj); err != nil {
			return nil, nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
		}
		var toolType string
		_ = json.Unmarshal(obj["type"], &toolType)
		if toolType != "" && toolType != "function" {
			// Chat completions APIs only support function tools.
			// Drop unsupported tool types to avoid upstream errors.
			slog.InfoContext(ctx, "drop unsupported responses tool for chat adapter",
				slog.String("api", "/v1/responses"),
				slog.String("adapter", "chat_completions"),
				slog.String("model", modelName),
				slog.String("dropped_tool_type", toolType))
			continue
		}
		function := map[string]json.RawMessage{}
		if rawFunction, ok := obj["function"]; ok {
			if err := json.Unmarshal(rawFunction, &function); err != nil || function == nil {
				return nil, nil, fmt.Errorf("convert responses function tool: function must be an object")
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
			return nil, nil, fmt.Errorf("convert responses function tool: %w", err)
		}
		var functionName string
		_ = json.Unmarshal(function["name"], &functionName)
		if functionName != "" {
			functionTools[functionName] = struct{}{}
		}
		chatTool := map[string]json.RawMessage{
			"type":     json.RawMessage(`"function"`),
			"function": functionRaw,
		}
		chatTools = append(chatTools, chatTool)
	}
	if len(chatTools) == 0 {
		return nil, functionTools, nil
	}
	data, err := json.Marshal(chatTools)
	if err != nil {
		return nil, nil, fmt.Errorf("convert responses tools to chat tools: %w", err)
	}
	return data, functionTools, nil
}

func responsesToolChoiceToChatToolChoice(ctx context.Context, raw json.RawMessage, functionTools map[string]struct{}, modelName string) json.RawMessage {
	var choice string
	if err := json.Unmarshal(raw, &choice); err == nil {
		if choice == "required" && len(functionTools) == 0 {
			return nil
		}
		return raw
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		slog.WarnContext(ctx, "drop unsupported responses tool_choice for chat adapter",
			slog.String("api", "/v1/responses"),
			slog.String("adapter", "chat_completions"),
			slog.String("model", modelName),
			slog.Any("error", err))
		return nil
	}
	var toolType string
	_ = json.Unmarshal(obj["type"], &toolType)
	if toolType != "function" {
		return nil
	}
	var function struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(obj["function"], &function)
	if _, ok := functionTools[function.Name]; !ok {
		return nil
	}
	return raw
}

func floatPtrValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func responsesInputToChatMessages(ctx context.Context, req *types.ResponsesRequest) ([]map[string]any, error) {
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
	// Keep pending reasoning until the next assistant message or tool-call turn.
	// Do not attach it to user/system messages.
	pendingReasoning := ""
	lastAssistantIdx := -1
	for _, item := range items {
		itemType, _ := item["type"].(string)
		switch itemType {
		case "message", "":
			role, _ := item["role"].(string)
			role = normalizeChatRole(role)
			content, err := normalizeResponsesContent(item["content"])
			if err != nil {
				return nil, err
			}
			message := map[string]any{"role": role, "content": content}
			if pendingReasoning != "" && role == "assistant" {
				message["reasoning_content"] = pendingReasoning
				pendingReasoning = ""
			}
			messages = append(messages, message)
			if role == "assistant" {
				lastAssistantIdx = len(messages) - 1
			}
		case "function_call":
			message := map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]any{{
					"id":   item["call_id"],
					"type": "function",
					"function": map[string]any{
						"name":      item["name"],
						"arguments": item["arguments"],
					},
				}},
			}
			if pendingReasoning != "" {
				message["reasoning_content"] = pendingReasoning
				pendingReasoning = ""
			}
			messages = append(messages, message)
			lastAssistantIdx = len(messages) - 1
		case "function_call_output":
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": item["call_id"],
				"content":      item["output"],
			})
		case "reasoning":
			if reasoning := responsesReasoningItemText(item); reasoning != "" {
				if pendingReasoning == "" {
					pendingReasoning = reasoning
				} else {
					pendingReasoning += "\n" + reasoning
				}
			}
		default:
			return nil, unsupportedResponsesFeature("input." + itemType)
		}
	}
	if pendingReasoning != "" {
		if lastAssistantIdx >= 0 {
			if existing, _ := messages[lastAssistantIdx]["reasoning_content"].(string); existing != "" {
				messages[lastAssistantIdx]["reasoning_content"] = existing + "\n" + pendingReasoning
			} else {
				messages[lastAssistantIdx]["reasoning_content"] = pendingReasoning
			}
		} else {
			slog.DebugContext(ctx, "drop orphan reasoning input with no assistant target",
				slog.String("api", "/v1/responses"))
		}
	}
	return messages, nil
}

func responsesReasoningItemText(item map[string]any) string {
	if text := responsesContentText(item["summary"]); text != "" {
		return text
	}
	return responsesContentText(item["content"])
}

func responsesContentText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := responsesContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, ok := v["text"]; ok {
			return responsesContentText(text)
		}
		if text, ok := v["content"]; ok {
			return responsesContentText(text)
		}
		return ""
	default:
		return ""
	}
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
	return ""
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
			imageURL := obj["image_url"]
			if s, ok := imageURL.(string); ok {
				imageURL = map[string]any{"url": s}
			}
			chatParts = append(chatParts, map[string]any{"type": "image_url", "image_url": imageURL})
		case "input_audio":
			inputAudio := obj["input_audio"]
			if inputAudio == nil {
				inputAudio = map[string]any{
					"data":   obj["audio"],
					"format": obj["format"],
				}
			}
			chatParts = append(chatParts, map[string]any{"type": "input_audio", "input_audio": inputAudio})
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
	reasoning := chatResponseReasoning(data)
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
		appendResponsesReasoning(resp, reasoning)
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
		appendResponsesReasoning(resp, reasoning)
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
	appendResponsesReasoning(resp, reasoning)
	return resp, nil
}

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

func appendResponsesReasoning(resp *types.ResponsesResponse, reasoning string) {
	if resp == nil || reasoning == "" {
		return
	}
	resp.Output = append(resp.Output, responsesReasoningOutputItem(reasoning))
}

func responsesReasoningOutputItem(reasoning string) types.ResponsesOutputItem {
	return types.ResponsesOutputItem{
		Type:   "reasoning",
		Status: "completed",
		Summary: []types.ResponsesSummaryPart{{
			Type: "summary_text",
			Text: reasoning,
		}},
	}
}
