package handler

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model         string         `json:"model"`
	Messages      []ChatMessage  `json:"messages"`
	Temperature   float64        `json:"temperature,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ChatMessageHistoryResponse represents the chat message history response format
type ChatMessageHistoryResponse struct {
	Messages []ChatMessage `json:"messages"`
}

// EmbeddingRequest represents an embedding request structure
type EmbeddingRequest struct {
	Input          string `json:"input"`           // Input text content
	Model          string `json:"model"`           // Model name used (e.g., "text-embedding-ada-002")
	EncodingFormat string `json:"encoding_format"` // Encoding format (e.g., "float")
}
