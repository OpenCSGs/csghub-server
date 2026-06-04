package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/sigil-sdk/go/sigil"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

type SigilConfig struct {
	ContentCapture       string
	MaxContentLength     int
	MaxInputUserMessages int
}

type sigilTracer struct {
	client               *sigil.Client
	contentCapture       sigil.ContentCaptureMode
	maxContentLength     int
	maxInputUserMessages int
}

func NewSigilTracer(config SigilConfig) (LLMTracer, error) {
	cfg := sigil.DefaultConfig()
	cfg.GenerationExport.Protocol = sigil.GenerationExportProtocolNone
	contentCapture := parseContentCapture(config.ContentCapture)
	cfg.ContentCapture = contentCapture

	return &sigilTracer{
		client:               sigil.NewClient(cfg),
		contentCapture:       contentCapture,
		maxContentLength:     config.MaxContentLength,
		maxInputUserMessages: config.MaxInputUserMessages,
	}, nil
}

func (t *sigilTracer) StartGeneration(ctx context.Context, input aigatewaytypes.GenerationStart) (context.Context, GenerationRecorder) {
	if t == nil || t.client == nil {
		return ctx, nil
	}
	startedAt := traceStartedAt(input.StartedAt)
	input.StartedAt = startedAt
	ctx, recorder := t.client.StartGeneration(ctx, toSigilGenerationStart(input, sigil.GenerationModeSync, t.contentCapture))
	span := oteltrace.SpanFromContext(ctx)
	setTraceMetadataAttributes(span, input.Metadata)
	return ctx, &sigilGenerationRecorder{recorder: recorder, span: span, contentCapture: t.contentCapture, maxContentLength: t.maxContentLength, maxInputUserMessages: t.maxInputUserMessages, mode: sigil.GenerationModeSync, startedAt: startedAt, tools: input.Tools}
}

func (t *sigilTracer) StartStreamingGeneration(ctx context.Context, input aigatewaytypes.GenerationStart) (context.Context, GenerationRecorder) {
	if t == nil || t.client == nil {
		return ctx, nil
	}
	startedAt := traceStartedAt(input.StartedAt)
	input.StartedAt = startedAt
	ctx, recorder := t.client.StartStreamingGeneration(ctx, toSigilGenerationStart(input, sigil.GenerationModeStream, t.contentCapture))
	span := oteltrace.SpanFromContext(ctx)
	setTraceMetadataAttributes(span, input.Metadata)
	return ctx, &sigilGenerationRecorder{recorder: recorder, span: span, contentCapture: t.contentCapture, maxContentLength: t.maxContentLength, maxInputUserMessages: t.maxInputUserMessages, mode: sigil.GenerationModeStream, startedAt: startedAt, tools: input.Tools}
}

func (t *sigilTracer) StartEmbedding(ctx context.Context, input aigatewaytypes.EmbeddingStart) (context.Context, EmbeddingRecorder) {
	if t == nil || t.client == nil {
		return ctx, nil
	}
	ctx, recorder := t.client.StartEmbedding(ctx, toSigilEmbeddingStart(input))
	return ctx, &sigilEmbeddingRecorder{recorder: recorder}
}

func (t *sigilTracer) Shutdown(ctx context.Context) error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Shutdown(ctx)
}

type sigilGenerationRecorder struct {
	recorder             *sigil.GenerationRecorder
	span                 oteltrace.Span
	usage                aigatewaytypes.TokenUsage
	response             aigatewaytypes.GenerationResponse
	contentCapture       sigil.ContentCaptureMode
	maxContentLength     int
	maxInputUserMessages int
	mode                 sigil.GenerationMode
	startedAt            time.Time
	firstChunkAt         time.Time
	tools                []aigatewaytypes.GenerationToolDefinition
	ended                bool
}

type sigilEmbeddingRecorder struct {
	recorder  *sigil.EmbeddingRecorder
	result    aigatewaytypes.EmbeddingResult
	hasResult bool
	ended     bool
}

func (r *sigilGenerationRecorder) SetUsage(usage aigatewaytypes.TokenUsage) {
	if r == nil {
		return
	}
	r.usage = usage
}

func (r *sigilGenerationRecorder) SetResponse(response aigatewaytypes.GenerationResponse) {
	if r == nil {
		return
	}
	r.response = response
}

func (r *sigilGenerationRecorder) SetFirstChunk(firstChunk aigatewaytypes.GenerationFirstChunk) {
	if r == nil || r.recorder == nil || firstChunk.At.IsZero() {
		return
	}
	r.firstChunkAt = firstChunk.At
	r.recorder.SetFirstTokenAt(firstChunk.At)
}

func (r *sigilGenerationRecorder) SetError(err error, code string) {
	if r == nil || r.recorder == nil || err == nil {
		return
	}
	if code != "" {
		err = fmt.Errorf("%s: %w", code, err)
	}
	r.recorder.SetCallError(err)
}

func (r *sigilGenerationRecorder) End() {
	if r == nil || r.recorder == nil || r.ended {
		return
	}
	r.ended = true
	setTraceMetadataAttributes(r.span, r.response.Metadata)
	r.recorder.SetResult(sigil.Generation{
		Model: sigil.ModelRef{
			Provider: r.response.Provider,
			Name:     r.response.Model,
		},
		TraceID:       r.response.TraceID,
		SpanID:        r.response.SpanID,
		ResponseID:    r.response.ResponseID,
		ResponseModel: r.response.ResponseModel,
		SystemPrompt:  r.response.SystemPrompt,
		Tools:         toSigilToolDefinitions(r.response.Tools),
		Usage: sigil.TokenUsage{
			InputTokens:           r.usage.InputTokens,
			OutputTokens:          r.usage.OutputTokens,
			TotalTokens:           r.usage.TotalTokens,
			CacheReadInputTokens:  r.usage.CacheReadInputTokens,
			CacheWriteInputTokens: r.usage.CacheWriteInputTokens,
			ReasoningTokens:       r.usage.ReasoningTokens,
		},
		StopReason:  firstNonEmpty(r.response.StopReason, firstString(r.response.FinishReasons)),
		CompletedAt: r.response.CompletedAt,
		Tags:        r.response.Tags,
		Metadata:    r.response.Metadata,
		Artifacts:   toSigilArtifacts(r.response.Artifacts),
		CallError:   r.response.CallError,
	}, nil)
	inputMessages, outputMessages := spanMessagesForContentCapture(r.response.Input, r.response.Output, r.contentCapture)
	inputMessages = limitInputMessages(inputMessages, r.maxInputUserMessages)
	if inputJSON := marshalMessagesToCompactJSON(inputMessages, r.maxContentLength); inputJSON != "" {
		r.span.SetAttributes(attribute.String("gen_ai.input.messages", inputJSON))
	}
	if outputJSON := marshalMessagesToCompactJSON(outputMessages, r.maxContentLength); outputJSON != "" {
		r.span.SetAttributes(attribute.String("gen_ai.output.messages", outputJSON))
	}
	if toolsJSON := marshalToolDefinitionsToCompactJSON(firstNonEmptyTools(r.response.Tools, r.tools), r.contentCapture, r.maxContentLength); toolsJSON != "" {
		r.span.SetAttributes(attribute.String("gen_ai.tool.definitions", toolsJSON))
	}
	if len(r.response.FinishReasons) > 0 {
		r.span.SetAttributes(attribute.StringSlice("gen_ai.response.finish_reasons", r.response.FinishReasons))
	}
	if r.mode == sigil.GenerationModeStream && !r.startedAt.IsZero() && !r.firstChunkAt.IsZero() {
		if ttfc := r.firstChunkAt.Sub(r.startedAt).Seconds(); ttfc >= 0 {
			r.span.SetAttributes(attribute.Float64("gen_ai.response.time_to_first_chunk", ttfc))
		}
	}
	r.recorder.End()
}

func (r *sigilEmbeddingRecorder) SetResult(result aigatewaytypes.EmbeddingResult) {
	if r == nil {
		return
	}
	r.result = result
	r.hasResult = true
}

func (r *sigilEmbeddingRecorder) SetError(err error, code string) {
	if r == nil || r.recorder == nil || err == nil {
		return
	}
	if code != "" {
		err = fmt.Errorf("%s: %w", code, err)
	}
	r.recorder.SetCallError(err)
}

func (r *sigilEmbeddingRecorder) End() {
	if r == nil || r.recorder == nil || r.ended {
		return
	}
	r.ended = true
	if r.hasResult {
		r.recorder.SetResult(sigil.EmbeddingResult{
			InputCount:    r.result.InputCount,
			InputTokens:   r.result.InputTokens,
			ResponseModel: r.result.ResponseModel,
			Dimensions:    r.result.Dimensions,
		})
	}
	r.recorder.End()
}

func toSigilGenerationStart(input aigatewaytypes.GenerationStart, mode sigil.GenerationMode, contentCapture sigil.ContentCaptureMode) sigil.GenerationStart {
	metadata := map[string]any{}
	for k, v := range input.Metadata {
		metadata[k] = v
	}
	if input.ToolCount > 0 {
		metadata["aigateway.tool_count"] = input.ToolCount
	}

	return sigil.GenerationStart{
		ID:                input.RequestID,
		ConversationID:    input.ConversationID,
		ConversationTitle: input.ConversationTitle,
		UserID:            input.UserID,
		AgentName:         input.AgentName,
		AgentVersion:      input.AgentVersion,
		Mode:              mode,
		OperationName:     input.OperationName,
		Model: sigil.ModelRef{
			Provider: input.Provider,
			Name:     input.ResolvedModel,
		},
		SystemPrompt:        input.SystemPrompt,
		Tools:               toSigilToolDefinitions(input.Tools),
		MaxTokens:           input.MaxTokens,
		Temperature:         input.Temperature,
		TopP:                input.TopP,
		ToolChoice:          input.ToolChoice,
		ThinkingEnabled:     input.ThinkingEnabled,
		ParentGenerationIDs: input.ParentGenerationIDs,
		EffectiveVersion:    input.EffectiveVersion,
		Tags:                input.Tags,
		Metadata:            metadata,
		StartedAt:           input.StartedAt,
		ContentCapture:      contentCapture,
	}
}

func toSigilEmbeddingStart(input aigatewaytypes.EmbeddingStart) sigil.EmbeddingStart {
	return sigil.EmbeddingStart{
		Model: sigil.ModelRef{
			Provider: input.Provider,
			Name:     input.ResolvedModel,
		},
		AgentName:      input.AgentName,
		AgentVersion:   input.AgentVersion,
		Dimensions:     input.Dimensions,
		EncodingFormat: input.EncodingFormat,
		Tags:           input.Tags,
		Metadata:       input.Metadata,
		StartedAt:      input.StartedAt,
	}
}

const (
	TraceMetadataKeyAIGatewayAPI         = "aigateway.api"
	TraceMetadataKeyAIGatewayModelID     = "aigateway.model.id"
	TraceMetadataKeyGenAIOutputType      = "gen_ai.output.type"
	TraceMetadataKeyImageSize            = "aigateway.image.size"
	TraceMetadataKeyImageQuality         = "aigateway.image.quality"
	TraceMetadataKeyImageResponseFormat  = "aigateway.image.response_format"
	TraceMetadataKeyImageOutputFormat    = "aigateway.image.output_format"
	TraceMetadataKeyImageN               = "aigateway.image.n"
	TraceMetadataKeyImageOutputCount     = "aigateway.image.output_count"
	TraceMetadataKeyAudioDurationSeconds = "aigateway.audio.duration_seconds"
	TraceMetadataKeyVideoID              = "aigateway.video.id"
	TraceMetadataKeyVideoStatus          = "aigateway.video.status"
	TraceMetadataKeyVideoSize            = "aigateway.video.size"
	TraceMetadataKeyVideoSeconds         = "aigateway.video.seconds"
)

var traceMetadataAttributeKeys = map[string]struct{}{
	TraceMetadataKeyAIGatewayAPI:         {},
	TraceMetadataKeyAIGatewayModelID:     {},
	TraceMetadataKeyGenAIOutputType:      {},
	TraceMetadataKeyImageSize:            {},
	TraceMetadataKeyImageQuality:         {},
	TraceMetadataKeyImageResponseFormat:  {},
	TraceMetadataKeyImageOutputFormat:    {},
	TraceMetadataKeyImageN:               {},
	TraceMetadataKeyImageOutputCount:     {},
	TraceMetadataKeyAudioDurationSeconds: {},
	TraceMetadataKeyVideoStatus:          {},
	TraceMetadataKeyVideoSize:            {},
	TraceMetadataKeyVideoSeconds:         {},
}

func setTraceMetadataAttributes(span oteltrace.Span, metadata map[string]any) {
	if span == nil || !span.SpanContext().IsValid() || len(metadata) == 0 {
		return
	}
	attrs := make([]attribute.KeyValue, 0, len(metadata))
	for key, value := range metadata {
		if _, ok := traceMetadataAttributeKeys[key]; !ok {
			continue
		}
		attr, ok := traceMetadataAttribute(key, value)
		if ok {
			attrs = append(attrs, attr)
		}
	}
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
}

func traceMetadataAttribute(key string, value any) (attribute.KeyValue, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return attribute.KeyValue{}, false
		}
		return attribute.String(key, v), true
	case bool:
		return attribute.Bool(key, v), true
	case int:
		return attribute.Int(key, v), true
	case int64:
		return attribute.Int64(key, v), true
	case float64:
		return attribute.Float64(key, v), true
	default:
		return attribute.KeyValue{}, false
	}
}

func toSigilMessages(messages []aigatewaytypes.GenerationMessage) []sigil.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]sigil.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, sigil.Message{
			Role:  sigil.Role(message.Role),
			Name:  message.Name,
			Parts: toSigilParts(message.Parts),
		})
	}
	return out
}

func toSigilParts(parts []aigatewaytypes.GenerationPart) []sigil.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]sigil.Part, 0, len(parts))
	for _, part := range parts {
		var metadata sigil.PartMetadata
		if part.Metadata != nil {
			metadata.ProviderType = part.Metadata.ProviderType
		}
		out = append(out, sigil.Part{
			Kind:       sigil.PartKind(part.Kind),
			Text:       part.Text,
			Thinking:   part.Thinking,
			ToolCall:   toSigilToolCall(part.ToolCall),
			ToolResult: toSigilToolResult(part.ToolResult),
			Metadata:   metadata,
		})
	}
	return out
}

func toSigilToolCall(toolCall *aigatewaytypes.GenerationToolCall) *sigil.ToolCall {
	if toolCall == nil {
		return nil
	}
	return &sigil.ToolCall{
		ID:        toolCall.ID,
		Name:      toolCall.Name,
		InputJSON: toolCall.InputJSON,
	}
}

func toSigilToolResult(toolResult *aigatewaytypes.GenerationToolResult) *sigil.ToolResult {
	if toolResult == nil {
		return nil
	}
	return &sigil.ToolResult{
		ToolCallID:  toolResult.ToolCallID,
		Name:        toolResult.Name,
		IsError:     toolResult.IsError,
		Content:     toolResult.Content,
		ContentJSON: toolResult.ContentJSON,
	}
}

func toSigilToolDefinitions(tools []aigatewaytypes.GenerationToolDefinition) []sigil.ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	out := make([]sigil.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		out = append(out, sigil.ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			Type:        tool.Type,
			InputSchema: tool.InputSchema,
			Deferred:    tool.Deferred,
		})
	}
	return out
}

func toSigilArtifacts(artifacts []aigatewaytypes.GenerationArtifact) []sigil.Artifact {
	if len(artifacts) == 0 {
		return nil
	}
	out := make([]sigil.Artifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, sigil.Artifact{
			Kind:        sigil.ArtifactKind(artifact.Kind),
			Name:        artifact.Name,
			ContentType: artifact.ContentType,
			Payload:     artifact.Payload,
			RecordID:    artifact.RecordID,
			URI:         artifact.URI,
		})
	}
	return out
}

func parseContentCapture(value string) sigil.ContentCaptureMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full":
		return sigil.ContentCaptureModeFull
	case "no_tool_content":
		return sigil.ContentCaptureModeNoToolContent
	default:
		return sigil.ContentCaptureModeMetadataOnly
	}
}

func firstString(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyTools(values ...[]aigatewaytypes.GenerationToolDefinition) []aigatewaytypes.GenerationToolDefinition {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func marshalMessagesToCompactJSON(messages []aigatewaytypes.GenerationMessage, maxContentLength int) string {
	if len(messages) == 0 {
		return ""
	}
	data, err := json.Marshal(truncateMessageTextFields(messages, maxContentLength))
	if err != nil {
		return ""
	}
	return string(data)
}

func limitInputMessages(messages []aigatewaytypes.GenerationMessage, maxInputUserMessages int) []aigatewaytypes.GenerationMessage {
	if len(messages) == 0 || maxInputUserMessages <= 0 {
		return messages
	}

	result := make([]aigatewaytypes.GenerationMessage, 0, maxInputUserMessages+1)
	startIndex := 0
	for i, msg := range messages {
		if msg.Role == "system" {
			result = append(result, msg)
			startIndex = i + 1
			break
		}
	}

	userMessages := make([]aigatewaytypes.GenerationMessage, 0, len(messages)-startIndex)
	for _, msg := range messages[startIndex:] {
		if msg.Role == "user" {
			userMessages = append(userMessages, msg)
		}
	}
	if len(userMessages) > maxInputUserMessages {
		userMessages = userMessages[len(userMessages)-maxInputUserMessages:]
	}
	result = append(result, userMessages...)
	return result
}

func marshalToolDefinitionsToCompactJSON(tools []aigatewaytypes.GenerationToolDefinition, mode sigil.ContentCaptureMode, maxContentLength int) string {
	if len(tools) == 0 || (mode != sigil.ContentCaptureModeFull && mode != sigil.ContentCaptureModeNoToolContent) {
		return ""
	}
	definitions := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		definition := map[string]any{
			"name": tool.Name,
		}
		if tool.Type != "" {
			definition["type"] = tool.Type
		}
		if len(tool.InputSchema) > 0 {
			if parameters, ok := compactToolParameters(tool.InputSchema, maxContentLength); ok {
				definition["parameters"] = parameters
			}
		}
		definitions = append(definitions, definition)
	}
	data, err := json.Marshal(definitions)
	if err != nil {
		return ""
	}
	return string(data)
}

func compactToolParameters(inputSchema json.RawMessage, maxContentLength int) (any, bool) {
	var parameters any
	if err := json.Unmarshal(inputSchema, &parameters); err != nil {
		return nil, false
	}
	if maxContentLength <= 0 {
		return parameters, true
	}
	return compactToolSchema(parameters, maxContentLength), true
}

func compactToolSchema(value any, maxContentLength int) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		if item, ok := v["type"]; ok {
			result["type"] = item
		}
		if item, ok := v["required"]; ok {
			result["required"] = item
		}
		if item, ok := v["properties"]; ok {
			result["properties"] = compactToolProperties(item, maxContentLength)
		}
		if item, ok := v["items"]; ok {
			result["items"] = compactToolSchema(item, maxContentLength)
		}
		if item, ok := v["additionalProperties"]; ok {
			result["additionalProperties"] = compactToolSchema(item, maxContentLength)
		}
		return result
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			result = append(result, compactToolSchema(item, maxContentLength))
		}
		return result
	default:
		return value
	}
}

func compactToolProperties(value any, maxContentLength int) any {
	properties, ok := value.(map[string]any)
	if !ok {
		return compactToolSchema(value, maxContentLength)
	}
	result := make(map[string]any, len(properties))
	for name, schema := range properties {
		result[name] = compactToolSchema(schema, maxContentLength)
	}
	return result
}

func truncateMessageTextFields(messages []aigatewaytypes.GenerationMessage, maxContentLength int) []aigatewaytypes.GenerationMessage {
	if len(messages) == 0 || maxContentLength <= 0 {
		return messages
	}
	out := make([]aigatewaytypes.GenerationMessage, 0, len(messages))
	for _, msg := range messages {
		parts := make([]aigatewaytypes.GenerationPart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			part.Text = truncateContent(part.Text, maxContentLength)
			part.Thinking = truncateContent(part.Thinking, maxContentLength)
			if part.ToolResult != nil {
				toolResult := *part.ToolResult
				toolResult.Content = truncateContent(toolResult.Content, maxContentLength)
				part.ToolResult = &toolResult
			}
			parts = append(parts, part)
		}
		msg.Parts = parts
		out = append(out, msg)
	}
	return out
}

func truncateContent(value string, maxContentLength int) string {
	if value == "" || maxContentLength <= 0 {
		return value
	}
	return commonutils.TruncStringByRune(value, maxContentLength)
}

func traceStartedAt(startedAt time.Time) time.Time {
	if !startedAt.IsZero() {
		return startedAt
	}
	return time.Now()
}

func spanMessagesForContentCapture(input, output []aigatewaytypes.GenerationMessage, mode sigil.ContentCaptureMode) ([]aigatewaytypes.GenerationMessage, []aigatewaytypes.GenerationMessage) {
	switch mode {
	case sigil.ContentCaptureModeFull:
		return input, output
	case sigil.ContentCaptureModeNoToolContent:
		return stripToolContentFromMessages(input), stripToolContentFromMessages(output)
	default:
		return nil, nil
	}
}

func stripToolContentFromMessages(messages []aigatewaytypes.GenerationMessage) []aigatewaytypes.GenerationMessage {
	if len(messages) == 0 {
		return nil
	}
	result := make([]aigatewaytypes.GenerationMessage, 0, len(messages))
	for _, msg := range messages {
		parts := make([]aigatewaytypes.GenerationPart, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			part.ToolCall = stripToolCallContent(part.ToolCall)
			part.ToolResult = stripToolResultContent(part.ToolResult)
			parts = append(parts, part)
		}
		msg.Parts = parts
		result = append(result, msg)
	}
	return result
}

func stripToolCallContent(toolCall *aigatewaytypes.GenerationToolCall) *aigatewaytypes.GenerationToolCall {
	if toolCall == nil {
		return nil
	}
	return &aigatewaytypes.GenerationToolCall{
		ID:   toolCall.ID,
		Name: toolCall.Name,
	}
}

func stripToolResultContent(toolResult *aigatewaytypes.GenerationToolResult) *aigatewaytypes.GenerationToolResult {
	if toolResult == nil {
		return nil
	}
	return &aigatewaytypes.GenerationToolResult{
		ToolCallID: toolResult.ToolCallID,
		Name:       toolResult.Name,
		IsError:    toolResult.IsError,
	}
}
