package responses

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

type LLMLogRecorder struct {
	requestID string
	modelID   string
	userUUID  string
	tools     json.RawMessage
	messages  []commontypes.LLMLogMessage
	metadata  map[string]any

	finalResponse     *types.ResponsesResponse
	responseID        string
	finishReasons     []string
	text              strings.Builder
	refusal           strings.Builder
	toolCalls         map[string]*llmLogToolCall
	sawOutputToolCall bool
	toolOrder         []string
}

type llmLogToolCall struct {
	name      string
	arguments strings.Builder
}

func (c *llmLogToolCall) setArgumentsIfBetter(arguments string) {
	if arguments == "" {
		return
	}
	current := c.arguments.String()
	if current == arguments {
		return
	}
	if current == "" || (json.Valid([]byte(arguments)) && !json.Valid([]byte(current))) {
		c.arguments.Reset()
		c.arguments.WriteString(arguments)
	}
}

func NewLLMLogRecorder(requestID, modelID, userUUID string, req *types.ResponsesRequest, metadata map[string]any) (*LLMLogRecorder, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	recorder := &LLMLogRecorder{
		requestID: requestID,
		modelID:   modelID,
		userUUID:  userUUID,
		tools:     json.RawMessage("[]"),
		metadata:  metadata,
		toolCalls: map[string]*llmLogToolCall{},
	}
	if req == nil {
		return recorder, nil
	}
	if len(req.Tools) > 0 {
		recorder.tools = append(json.RawMessage(nil), req.Tools...)
	}
	messages, err := normalizeResponsesInputMessages(req)
	if err != nil {
		return nil, err
	}
	recorder.messages = messages
	return recorder, nil
}

func (r *LLMLogRecorder) CaptureResponse(resp *types.ResponsesResponse) {
	if r == nil || resp == nil {
		return
	}
	cp := *resp
	r.finalResponse = &cp
	if resp.ID != "" {
		r.responseID = resp.ID
	}
}

func (r *LLMLogRecorder) CaptureResponseID(responseID string) {
	if r == nil || responseID == "" {
		return
	}
	r.responseID = responseID
}

func (r *LLMLogRecorder) CaptureFinishReason(reason string) {
	if r == nil || reason == "" {
		return
	}
	for _, existing := range r.finishReasons {
		if existing == reason {
			return
		}
	}
	r.finishReasons = append(r.finishReasons, reason)
}

func (r *LLMLogRecorder) CaptureOutputTextDelta(delta string) {
	if r == nil || delta == "" {
		return
	}
	r.text.WriteString(delta)
}

func (r *LLMLogRecorder) CaptureOutputTextDone(text string) {
	if r == nil || text == "" {
		return
	}
	r.text.Reset()
	r.text.WriteString(text)
}

func (r *LLMLogRecorder) CaptureRefusalDelta(delta string) {
	if r == nil || delta == "" {
		return
	}
	r.refusal.WriteString(delta)
}

func (r *LLMLogRecorder) CaptureRefusalDone(refusal string) {
	if r == nil || refusal == "" {
		return
	}
	r.refusal.Reset()
	r.refusal.WriteString(refusal)
}

func (r *LLMLogRecorder) CaptureToolCallStart(itemID, name, arguments string) {
	if r == nil {
		return
	}
	call := r.ensureToolCall(itemID)
	if name != "" {
		call.name = name
	}
	call.setArgumentsIfBetter(arguments)
}

func (r *LLMLogRecorder) CaptureToolCallArgumentsDelta(itemID, delta string) {
	if r == nil || delta == "" {
		return
	}
	r.ensureToolCall(itemID).arguments.WriteString(delta)
}

func (r *LLMLogRecorder) CaptureToolCallArgumentsDone(itemID, arguments string) {
	if r == nil || arguments == "" {
		return
	}
	r.ensureToolCall(itemID).setArgumentsIfBetter(arguments)
}

func (r *LLMLogRecorder) CapturePayload(payload any) {
	if r == nil || payload == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return
	}
	r.CapturePayloadMap(obj)
}

func (r *LLMLogRecorder) CapturePayloadMap(obj map[string]any) {
	if r == nil || obj == nil {
		return
	}
	if response, ok := obj["response"].(map[string]any); ok {
		if resp := decodeResponsesResponseMap(response); resp != nil {
			r.CaptureResponse(resp)
		}
	}
	if obj["object"] == "response" {
		if resp := decodeResponsesResponseMap(obj); resp != nil {
			r.CaptureResponse(resp)
		}
	}
	eventType, _ := obj["type"].(string)
	if responseID, _ := obj["response_id"].(string); responseID != "" {
		r.responseID = responseID
	}
	switch eventType {
	case "response.output_text.delta":
		r.CaptureOutputTextDelta(stringField(obj, "delta"))
	case "response.output_text.done":
		r.CaptureOutputTextDone(stringField(obj, "text"))
	case "response.refusal.delta":
		r.CaptureRefusalDelta(stringField(obj, "delta"))
	case "response.refusal.done":
		r.CaptureRefusalDone(stringField(obj, "refusal"))
	case "response.function_call_arguments.delta":
		r.CaptureToolCallArgumentsDelta(stringField(obj, "item_id"), stringField(obj, "delta"))
	case "response.function_call_arguments.done":
		r.CaptureToolCallArgumentsDone(stringField(obj, "item_id"), stringField(obj, "arguments"))
	}
	if item, ok := obj["item"].(map[string]any); ok {
		r.captureOutputItemMap(item)
	}
	if part, ok := obj["part"].(map[string]any); ok {
		r.captureContentPartMap(part)
	}
	r.captureTerminalFinishReason(obj)
}

func (r *LLMLogRecorder) captureTerminalFinishReason(obj map[string]any) {
	resp, _ := obj["response"].(map[string]any)
	if resp == nil && obj["object"] == "response" {
		resp = obj
	}
	if resp == nil {
		return
	}
	status, _ := resp["status"].(string)
	switch status {
	case "completed":
		if r.sawOutputToolCall {
			r.CaptureFinishReason("tool_calls")
		} else {
			r.CaptureFinishReason("stop")
		}
	case "incomplete":
		if details, ok := resp["incomplete_details"].(map[string]any); ok {
			if reason, ok := details["reason"].(string); ok && reason != "" {
				r.CaptureFinishReason(reason)
			}
		}
	}
}

func (r *LLMLogRecorder) Record(usage *token.Usage) (*commontypes.LLMLogRecord, error) {
	if r == nil {
		return nil, nil
	}
	inputMessages, outputMessages := r.Messages()
	messages := append(inputMessages, outputMessages...)
	metadata := normalizeResponsesLogMetadata(r.metadata)
	if r.responseID != "" {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["response_id"] = r.responseID
	}
	return &commontypes.LLMLogRecord{
		RequestID:  r.requestID,
		EventTime:  time.Now().Format(time.RFC3339Nano),
		SampleType: "responses",
		ModelID:    r.modelID,
		UserUUID:   r.userUUID,
		Tools:      r.tools,
		Messages:   messages,
		Usage:      llmLogUsageFromTokenUsage(usage),
		Metadata:   metadata,
	}, nil
}

func (r *LLMLogRecorder) Messages() (input, output []commontypes.LLMLogMessage) {
	if r == nil {
		return nil, nil
	}
	input = append([]commontypes.LLMLogMessage(nil), r.messages...)
	output = r.outputMessages()
	return input, output
}

func (r *LLMLogRecorder) TraceInfo() commontypes.LLMLogTraceInfo {
	if r == nil {
		return commontypes.LLMLogTraceInfo{}
	}
	return commontypes.LLMLogTraceInfo{
		ResponseID:    r.responseID,
		FinishReasons: append([]string(nil), r.finishReasons...),
	}
}

func (r *LLMLogRecorder) outputMessages() []commontypes.LLMLogMessage {
	if r.finalResponse != nil && (len(r.finalResponse.Output) > 0 || strings.TrimSpace(r.finalResponse.OutputText) != "") {
		return normalizeResponsesOutputMessages(r.finalResponse.Output, r.finalResponse.OutputText)
	}
	var output []commontypes.LLMLogMessage
	for _, key := range r.orderedToolCallKeys() {
		call := r.toolCalls[key]
		if call == nil {
			continue
		}
		msg, err := newLLMLogToolCallMessage(call.name, call.arguments.String())
		if err == nil {
			output = append(output, msg)
		}
	}
	if strings.TrimSpace(r.text.String()) != "" || strings.TrimSpace(r.refusal.String()) != "" {
		content := strings.TrimSpace(strings.TrimSpace(r.text.String()) + "\n" + strings.TrimSpace(r.refusal.String()))
		output = append(output, commontypes.LLMLogMessage{Role: "assistant", Content: content})
	}
	return output
}

func (r *LLMLogRecorder) captureOutputItemMap(item map[string]any) {
	itemType, _ := item["type"].(string)
	if itemType == "function_call" || stringField(item, "name") != "" || stringField(item, "arguments") != "" {
		r.sawOutputToolCall = true
		itemID := stringField(item, "id")
		if itemID == "" {
			itemID = stringField(item, "call_id")
		}
		call := r.ensureToolCall(itemID)
		if name := stringField(item, "name"); name != "" {
			call.name = name
		}
		call.setArgumentsIfBetter(stringField(item, "arguments"))
	}
	if content, ok := item["content"].([]any); ok {
		for _, raw := range content {
			if part, ok := raw.(map[string]any); ok {
				r.captureContentPartMap(part)
			}
		}
	}
}

func (r *LLMLogRecorder) captureContentPartMap(part map[string]any) {
	switch stringField(part, "type") {
	case "output_text", "text":
		if r.text.Len() == 0 {
			r.text.WriteString(stringField(part, "text"))
		}
	case "refusal":
		if r.refusal.Len() == 0 {
			r.refusal.WriteString(stringField(part, "refusal"))
		}
	}
}

func (r *LLMLogRecorder) ensureToolCall(itemID string) *llmLogToolCall {
	if itemID == "" {
		itemID = fmt.Sprintf("tool_call_%d", len(r.toolCalls))
	}
	if call := r.toolCalls[itemID]; call != nil {
		return call
	}
	call := &llmLogToolCall{}
	r.toolCalls[itemID] = call
	r.toolOrder = append(r.toolOrder, itemID)
	return call
}

func (r *LLMLogRecorder) orderedToolCallKeys() []string {
	keys := append([]string(nil), r.toolOrder...)
	if len(keys) == 0 && len(r.toolCalls) > 0 {
		for key := range r.toolCalls {
			keys = append(keys, key)
		}
		sort.Strings(keys)
	}
	return keys
}

func decodeResponsesResponseMap(obj map[string]any) *types.ResponsesResponse {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var resp types.ResponsesResponse
	if err := json.Unmarshal(data, &resp); err != nil || resp.Object != "response" {
		return nil
	}
	return &resp
}

func llmLogUsageFromTokenUsage(usage *token.Usage) commontypes.LLMLogUsage {
	if usage == nil {
		return commontypes.LLMLogUsage{}
	}
	return commontypes.LLMLogUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func normalizeResponsesLogMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	normalized := make(map[string]any, len(metadata))
	for key, value := range metadata {
		normalized[key] = value
	}
	return normalized
}

func stringField(obj map[string]any, key string) string {
	value, _ := obj[key].(string)
	return value
}
