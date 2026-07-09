package responses

import "opencsg.com/csghub-server/aigateway/types"

type StreamResponseEvent struct {
	Type     string         `json:"type"`
	Response StreamResponse `json:"response"`
}

type StreamResponse struct {
	ID         string                `json:"id"`
	Object     string                `json:"object"`
	CreatedAt  int64                 `json:"created_at"`
	Status     string                `json:"status"`
	Model      string                `json:"model"`
	Output     []any                 `json:"output,omitempty"`
	OutputText string                `json:"output_text,omitempty"`
	Usage      *types.ResponsesUsage `json:"usage,omitempty"`
}

type StreamOutputItemEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	Item        any    `json:"item"`
}

type StreamContentPartEvent struct {
	Type         string            `json:"type"`
	ResponseID   string            `json:"response_id"`
	ItemID       string            `json:"item_id,omitempty"`
	OutputIndex  int               `json:"output_index"`
	ContentIndex int               `json:"content_index"`
	Part         StreamContentPart `json:"part"`
}

type StreamOutputTextDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type StreamOutputTextDoneEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text"`
}

type StreamRefusalDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type StreamRefusalDoneEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Refusal      string `json:"refusal"`
}

type StreamReasoningSummaryDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	SummaryIndex int    `json:"summary_index"`
	Delta        string `json:"delta"`
}

type StreamReasoningSummaryDoneEvent struct {
	Type         string                     `json:"type"`
	ResponseID   string                     `json:"response_id"`
	ItemID       string                     `json:"item_id"`
	OutputIndex  int                        `json:"output_index"`
	SummaryIndex int                        `json:"summary_index"`
	Part         StreamReasoningSummaryPart `json:"part"`
}

type StreamFunctionCallArgumentsDeltaEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Delta       string `json:"delta"`
}

type StreamFunctionCallArgumentsDoneEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
}

type StreamMessageItem struct {
	ID      string              `json:"id,omitempty"`
	Type    string              `json:"type"`
	Role    string              `json:"role"`
	Status  string              `json:"status"`
	Content []StreamContentPart `json:"content,omitempty"`
}

type StreamFunctionCallItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
	Status    string `json:"status"`
}

type StreamReasoningItem struct {
	ID      string                       `json:"id,omitempty"`
	Type    string                       `json:"type"`
	Status  string                       `json:"status,omitempty"`
	Summary []StreamReasoningSummaryPart `json:"summary,omitempty"`
}

type StreamReasoningSummaryPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type StreamContentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}
