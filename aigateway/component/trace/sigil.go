package trace

import (
	"context"
	"fmt"
	"strings"

	"github.com/grafana/sigil-sdk/go/sigil"
	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
)

type SigilConfig struct {
	ContentCapture string
}

type sigilTracer struct {
	client *sigil.Client
}

func NewSigilTracer(config SigilConfig) (LLMTracer, error) {
	cfg := sigil.DefaultConfig()
	cfg.GenerationExport.Protocol = sigil.GenerationExportProtocolNone
	cfg.ContentCapture = parseContentCapture(config.ContentCapture)

	return &sigilTracer{
		client: sigil.NewClient(cfg),
	}, nil
}

func (t *sigilTracer) StartGeneration(ctx context.Context, input aigatewaytypes.GenerationStart) (context.Context, GenerationRecorder) {
	if t == nil || t.client == nil {
		return ctx, nil
	}
	ctx, recorder := t.client.StartGeneration(ctx, toSigilGenerationStart(input, sigil.GenerationModeSync))
	return ctx, &sigilGenerationRecorder{recorder: recorder}
}

func (t *sigilTracer) StartStreamingGeneration(ctx context.Context, input aigatewaytypes.GenerationStart) (context.Context, GenerationRecorder) {
	if t == nil || t.client == nil {
		return ctx, nil
	}
	ctx, recorder := t.client.StartStreamingGeneration(ctx, toSigilGenerationStart(input, sigil.GenerationModeStream))
	return ctx, &sigilGenerationRecorder{recorder: recorder}
}

func (t *sigilTracer) Shutdown(ctx context.Context) error {
	if t == nil || t.client == nil {
		return nil
	}
	return t.client.Shutdown(ctx)
}

type sigilGenerationRecorder struct {
	recorder *sigil.GenerationRecorder
	usage    aigatewaytypes.TokenUsage
	response aigatewaytypes.GenerationResponse
	ended    bool
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
		Input:         toSigilMessages(r.response.Input),
		Output:        toSigilMessages(r.response.Output),
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
	r.recorder.End()
}

func toSigilGenerationStart(input aigatewaytypes.GenerationStart, mode sigil.GenerationMode) sigil.GenerationStart {
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
		ContentCapture:      sigil.ContentCaptureModeDefault,
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
		out = append(out, sigil.Part{
			Kind:       sigil.PartKind(part.Kind),
			Text:       part.Text,
			Thinking:   part.Thinking,
			ToolCall:   toSigilToolCall(part.ToolCall),
			ToolResult: toSigilToolResult(part.ToolResult),
			Metadata: sigil.PartMetadata{
				ProviderType: part.Metadata.ProviderType,
			},
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
