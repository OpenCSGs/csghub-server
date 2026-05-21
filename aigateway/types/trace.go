package types

import (
	"encoding/json"
	"time"
)

type GenerationMode string

const (
	GenerationModeSync   GenerationMode = "sync"
	GenerationModeStream GenerationMode = "stream"
)

// Trace error codes classify why a generation failed.
const (
	TraceErrInsufficientBalance = "insufficient_balance"
	TraceErrSensitivePrompt     = "sensitive_prompt"
	TraceErrUpstreamUnavailable = "upstream_unavailable"
	TraceErrUpstreamError       = "upstream_error"
)

type GenerationStart struct {
	RequestID           string
	ConversationID      string
	ConversationTitle   string
	UserID              string
	AgentName           string
	AgentVersion        string
	Provider            string
	RequestModel        string
	ResolvedModel       string
	Mode                GenerationMode
	OperationName       string
	SystemPrompt        string
	Input               []GenerationMessage
	Tools               []GenerationToolDefinition
	ToolCount           int
	MaxTokens           *int64
	Temperature         *float64
	TopP                *float64
	ToolChoice          *string
	ThinkingEnabled     *bool
	ParentGenerationIDs []string
	EffectiveVersion    string
	Tags                map[string]string
	Metadata            map[string]any
	StartedAt           time.Time
}

type GenerationMessage struct {
	Role  string           `json:"role"`
	Name  string           `json:"name,omitempty"`
	Parts []GenerationPart `json:"parts"`
}

type GenerationPart struct {
	Kind       string                 `json:"kind"`
	Text       string                 `json:"text,omitempty"`
	Thinking   string                 `json:"thinking,omitempty"`
	ToolCall   *GenerationToolCall    `json:"tool_call,omitempty"`
	ToolResult *GenerationToolResult  `json:"tool_result,omitempty"`
	Metadata   GenerationPartMetadata `json:"metadata,omitempty"`
}

type GenerationPartMetadata struct {
	ProviderType string `json:"provider_type,omitempty"`
}

type GenerationToolCall struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name"`
	InputJSON json.RawMessage `json:"input_json,omitempty"`
}

type GenerationToolResult struct {
	ToolCallID  string          `json:"tool_call_id,omitempty"`
	Name        string          `json:"name,omitempty"`
	IsError     bool            `json:"is_error,omitempty"`
	Content     string          `json:"content,omitempty"`
	ContentJSON json.RawMessage `json:"content_json,omitempty"`
}

type GenerationToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	Deferred    bool            `json:"deferred,omitempty"`
}

type GenerationArtifact struct {
	Kind        string `json:"kind"`
	Name        string `json:"name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Payload     []byte `json:"payload,omitempty"`
	RecordID    string `json:"record_id,omitempty"`
	URI         string `json:"uri,omitempty"`
}

type TokenUsage struct {
	InputTokens           int64
	OutputTokens          int64
	TotalTokens           int64
	CacheReadInputTokens  int64
	CacheWriteInputTokens int64
	ReasoningTokens       int64
}

type GenerationFirstChunk struct {
	At time.Time
}

type GenerationResponse struct {
	Provider      string
	Model         string
	TraceID       string
	SpanID        string
	ResponseID    string
	ResponseModel string
	SystemPrompt  string
	Input         []GenerationMessage
	Output        []GenerationMessage
	Tools         []GenerationToolDefinition
	StopReason    string
	FinishReasons []string
	CompletedAt   time.Time
	Tags          map[string]string
	Metadata      map[string]any
	Artifacts     []GenerationArtifact
	CallError     string
}
