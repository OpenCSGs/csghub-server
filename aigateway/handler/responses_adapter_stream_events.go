package handler

import "opencsg.com/csghub-server/aigateway/types"

type responsesStreamResponseEvent struct {
	Type     string                  `json:"type"`
	Response responsesStreamResponse `json:"response"`
}

type responsesStreamResponse struct {
	ID         string                `json:"id"`
	Object     string                `json:"object"`
	CreatedAt  int64                 `json:"created_at"`
	Status     string                `json:"status"`
	Model      string                `json:"model"`
	Output     []any                 `json:"output,omitempty"`
	OutputText string                `json:"output_text,omitempty"`
	Usage      *types.ResponsesUsage `json:"usage,omitempty"`
}

type responsesStreamOutputItemEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	OutputIndex int    `json:"output_index"`
	Item        any    `json:"item"`
}

type responsesStreamContentPartEvent struct {
	Type         string                     `json:"type"`
	ResponseID   string                     `json:"response_id"`
	ItemID       string                     `json:"item_id,omitempty"`
	OutputIndex  int                        `json:"output_index"`
	ContentIndex int                        `json:"content_index"`
	Part         responsesStreamContentPart `json:"part"`
}

type responsesStreamOutputTextDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type responsesStreamOutputTextDoneEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Text         string `json:"text"`
}

type responsesStreamRefusalDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

type responsesStreamRefusalDoneEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Refusal      string `json:"refusal"`
}

type responsesStreamReasoningSummaryDeltaEvent struct {
	Type         string `json:"type"`
	ResponseID   string `json:"response_id"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	SummaryIndex int    `json:"summary_index"`
	Delta        string `json:"delta"`
}

type responsesStreamReasoningSummaryDoneEvent struct {
	Type         string                              `json:"type"`
	ResponseID   string                              `json:"response_id"`
	ItemID       string                              `json:"item_id"`
	OutputIndex  int                                 `json:"output_index"`
	SummaryIndex int                                 `json:"summary_index"`
	Part         responsesStreamReasoningSummaryPart `json:"part"`
}

type responsesStreamFunctionCallArgumentsDeltaEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
	Delta       string `json:"delta"`
}

type responsesStreamFunctionCallArgumentsDoneEvent struct {
	Type        string `json:"type"`
	ResponseID  string `json:"response_id"`
	ItemID      string `json:"item_id"`
	OutputIndex int    `json:"output_index"`
}

type responsesStreamMessageItem struct {
	ID      string                       `json:"id,omitempty"`
	Type    string                       `json:"type"`
	Role    string                       `json:"role"`
	Status  string                       `json:"status"`
	Content []responsesStreamContentPart `json:"content,omitempty"`
}

type responsesStreamFunctionCallItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
	Status    string `json:"status"`
}

type responsesStreamReasoningItem struct {
	ID      string                                `json:"id,omitempty"`
	Type    string                                `json:"type"`
	Status  string                                `json:"status,omitempty"`
	Summary []responsesStreamReasoningSummaryPart `json:"summary,omitempty"`
}

type responsesStreamReasoningSummaryPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesStreamContentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}
