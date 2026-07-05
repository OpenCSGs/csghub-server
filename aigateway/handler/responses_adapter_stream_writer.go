package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type responsesAdapterStreamWriter struct {
	ginWriter        gin.ResponseWriter
	decoder          streamdecoder.Decoder
	respID           string
	model            string
	created          int64
	eventBuf         bytes.Buffer
	started          bool
	completed        bool
	failed           bool
	passthrough      bool
	textStarted      bool
	textDone         bool
	textOutputIdx    int
	textItemID       string
	text             strings.Builder
	refusalStarted   bool
	refusalDone      bool
	refusalOutputIdx int
	refusalItemID    string
	refusal          strings.Builder
	nextOutputIdx    int
	responsesCounter token.ResponsesTokenCounter
	usage            *types.ResponsesUsage
	toolCallItems    map[int]*responsesToolCallStreamState
}

type responsesToolCallStreamState struct {
	OutputIndex int
	CallID      string
	Name        string
	Arguments   strings.Builder
	Done        bool
}

func newResponsesAdapterStreamWriter(w gin.ResponseWriter, model string, responsesCounter token.ResponsesTokenCounter) *responsesAdapterStreamWriter {
	return &responsesAdapterStreamWriter{
		ginWriter:        w,
		respID:           newAdapterResponseID(),
		model:            model,
		created:          time.Now().Unix(),
		decoder:          streamdecoder.NewSSE(),
		responsesCounter: responsesCounter,
		toolCallItems:    map[int]*responsesToolCallStreamState{},
	}
}

func (w *responsesAdapterStreamWriter) Header() http.Header {
	return w.ginWriter.Header()
}

func (w *responsesAdapterStreamWriter) WriteHeader(code int) {
	if isUpstreamHTTPError(code) {
		w.passthrough = true
		w.ginWriter.Header().Del("Content-Length")
		w.ginWriter.WriteHeader(code)
		return
	}
	w.ginWriter.Header().Set("Content-Type", "text/event-stream")
	w.ginWriter.Header().Del("Content-Length")
	w.ginWriter.WriteHeader(code)
}

func (w *responsesAdapterStreamWriter) Flush() {
	w.ginWriter.Flush()
}

func (w *responsesAdapterStreamWriter) Finalize(statusCode int) error {
	if isUpstreamHTTPError(statusCode) || w.passthrough || w.failed {
		return nil
	}
	w.finishResponseStream()
	return nil
}

func (w *responsesAdapterStreamWriter) ClearBuffer() {
}

func (w *responsesAdapterStreamWriter) Write(data []byte) (int, error) {
	if w.failed {
		return len(data), nil
	}
	if w.passthrough {
		n, err := w.ginWriter.Write(data)
		w.ginWriter.Flush()
		return n, err
	}
	// Decoder errors are reserved for future checks such as buffer overflow.
	events, _ := w.decoder.Write(data)
	for _, event := range events {
		if event.Type == "error" {
			w.failResponseStream(event)
			return len(data), nil
		}
		if len(event.Data) == 0 {
			continue
		}
		if string(event.Data) == "[DONE]" {
			w.finishResponseStream()
			continue
		}
		var chunk types.ChatCompletionChunk
		if err := json.Unmarshal(event.Data, &chunk); err != nil {
			w.writeData(string(event.Data))
			continue
		}
		w.captureChatStreamUsage(chunk)
		w.ensureStarted()
		w.writeToolCallDeltas(chunk)
		for _, choice := range chunk.Choices {
			if choice.Delta.Refusal != "" {
				w.ensureRefusalItem()
				w.refusal.WriteString(choice.Delta.Refusal)
				w.writeResponsesEvent("response.refusal.delta", responsesStreamRefusalDeltaEvent{
					Type:         "response.refusal.delta",
					ResponseID:   w.respID,
					ItemID:       w.refusalItemID,
					OutputIndex:  w.refusalOutputIdx,
					ContentIndex: 0,
					Delta:        choice.Delta.Refusal,
				})
			} else if choice.Delta.Content != "" {
				w.ensureTextItem()
				w.text.WriteString(choice.Delta.Content)
				w.writeResponsesEvent("response.output_text.delta", responsesStreamOutputTextDeltaEvent{
					Type:         "response.output_text.delta",
					ResponseID:   w.respID,
					ItemID:       w.textItemID,
					OutputIndex:  w.textOutputIdx,
					ContentIndex: 0,
					Delta:        choice.Delta.Content,
				})
			}
			if choice.FinishReason != "" {
				if choice.FinishReason == "tool_calls" {
					w.finishToolCallItems()
				} else {
					w.finishTextItem()
					w.finishRefusalItem()
				}
			}
		}
	}
	return len(data), nil
}

func (w *responsesAdapterStreamWriter) finishResponseStream() {
	if w.completed || w.failed {
		return
	}
	w.completed = true
	w.writeResponsesEvent("response.completed", responsesStreamResponseEvent{
		Type:     "response.completed",
		Response: w.completedResponse(),
	})
	w.writeData("data: [DONE]")
}

func (w *responsesAdapterStreamWriter) failResponseStream(event *streamdecoder.Event) {
	if w.completed || w.failed {
		return
	}
	w.failed = true
	w.writeRawEvent(event)
}

func (w *responsesAdapterStreamWriter) captureChatStreamUsage(chunk types.ChatCompletionChunk) {
	if chunk.Usage.PromptTokens == 0 && chunk.Usage.CompletionTokens == 0 && chunk.Usage.TotalTokens == 0 {
		return
	}
	w.usage = &types.ResponsesUsage{
		InputTokens:  int64(chunk.Usage.PromptTokens),
		OutputTokens: int64(chunk.Usage.CompletionTokens),
		TotalTokens:  int64(chunk.Usage.TotalTokens),
	}
	if w.responsesCounter != nil {
		w.responsesCounter.Response(&types.ResponsesResponse{Usage: w.usage})
	}
}

func (w *responsesAdapterStreamWriter) writeToolCallDeltas(chunk types.ChatCompletionChunk) {
	for _, choice := range chunk.Choices {
		for _, call := range choice.Delta.ToolCalls {
			index := int(call.Index)
			callID := call.ID
			name := call.Function.Name
			state := w.ensureToolCallItem(index, callID, name)
			if args := call.Function.Arguments; args != "" {
				state.Arguments.WriteString(args)
				w.writeResponsesEvent("response.function_call_arguments.delta", responsesStreamFunctionCallArgumentsDeltaEvent{
					Type:        "response.function_call_arguments.delta",
					ResponseID:  w.respID,
					ItemID:      state.CallID,
					OutputIndex: state.OutputIndex,
					Delta:       args,
				})
			}
		}
	}
}

func (w *responsesAdapterStreamWriter) ensureToolCallItem(index int, callID, name string) *responsesToolCallStreamState {
	if state := w.toolCallItems[index]; state != nil {
		if state.CallID == "" && callID != "" {
			state.CallID = callID
		}
		if state.Name == "" && name != "" {
			state.Name = name
		}
		return state
	}
	itemID := callID
	if itemID == "" {
		itemID = fmt.Sprintf("fc_%d", index)
	}
	state := &responsesToolCallStreamState{
		OutputIndex: w.nextOutputIndex(),
		CallID:      itemID,
		Name:        name,
	}
	w.toolCallItems[index] = state
	w.writeResponsesEvent("response.output_item.added", responsesStreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: state.OutputIndex,
		Item: responsesStreamFunctionCallItem{
			ID:        itemID,
			Type:      "function_call",
			CallID:    itemID,
			Name:      name,
			Arguments: "",
			Status:    "in_progress",
		},
	})
	return state
}

func (w *responsesAdapterStreamWriter) ensureStarted() {
	if w.started {
		return
	}
	w.started = true
	w.writeResponsesEvent("response.created", responsesStreamResponseEvent{
		Type:     "response.created",
		Response: w.responseWithStatus("in_progress"),
	})
	w.writeResponsesEvent("response.in_progress", responsesStreamResponseEvent{
		Type:     "response.in_progress",
		Response: w.responseWithStatus("in_progress"),
	})
}

func (w *responsesAdapterStreamWriter) ensureTextItem() {
	if w.textStarted {
		return
	}
	w.textStarted = true
	w.textOutputIdx = w.nextOutputIndex()
	w.textItemID = fmt.Sprintf("msg_%d", w.textOutputIdx)
	w.writeResponsesEvent("response.output_item.added", responsesStreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: w.textOutputIdx,
		Item: responsesStreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "in_progress",
		},
	})
	w.writeResponsesEvent("response.content_part.added", responsesStreamContentPartEvent{
		Type:         "response.content_part.added",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Part: responsesStreamContentPart{
			Type: "output_text",
			Text: "",
		},
	})
}

func (w *responsesAdapterStreamWriter) ensureRefusalItem() {
	if w.refusalStarted {
		return
	}
	w.refusalStarted = true
	w.refusalOutputIdx = w.nextOutputIndex()
	w.refusalItemID = fmt.Sprintf("msg_%d", w.refusalOutputIdx)
	w.writeResponsesEvent("response.output_item.added", responsesStreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: w.refusalOutputIdx,
		Item: responsesStreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "in_progress",
		},
	})
	w.writeResponsesEvent("response.content_part.added", responsesStreamContentPartEvent{
		Type:         "response.content_part.added",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Part: responsesStreamContentPart{
			Type:    "refusal",
			Refusal: "",
		},
	})
}

func (w *responsesAdapterStreamWriter) finishTextItem() {
	if !w.textStarted || w.textDone {
		return
	}
	w.textDone = true
	w.writeResponsesEvent("response.output_text.done", responsesStreamOutputTextDoneEvent{
		Type:         "response.output_text.done",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Text:         w.text.String(),
	})
	w.writeResponsesEvent("response.content_part.done", responsesStreamContentPartEvent{
		Type:         "response.content_part.done",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Part: responsesStreamContentPart{
			Type: "output_text",
			Text: w.text.String(),
		},
	})
	w.writeResponsesEvent("response.output_item.done", responsesStreamOutputItemEvent{
		Type:        "response.output_item.done",
		ResponseID:  w.respID,
		OutputIndex: w.textOutputIdx,
		Item: responsesStreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsesStreamContentPart{{
				Type: "output_text",
				Text: w.text.String(),
			}},
		},
	})
}

func (w *responsesAdapterStreamWriter) finishRefusalItem() {
	if !w.refusalStarted || w.refusalDone {
		return
	}
	w.refusalDone = true
	w.writeResponsesEvent("response.refusal.done", responsesStreamRefusalDoneEvent{
		Type:         "response.refusal.done",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Refusal:      w.refusal.String(),
	})
	w.writeResponsesEvent("response.content_part.done", responsesStreamContentPartEvent{
		Type:         "response.content_part.done",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Part: responsesStreamContentPart{
			Type:    "refusal",
			Refusal: w.refusal.String(),
		},
	})
	w.writeResponsesEvent("response.output_item.done", responsesStreamOutputItemEvent{
		Type:        "response.output_item.done",
		ResponseID:  w.respID,
		OutputIndex: w.refusalOutputIdx,
		Item: responsesStreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsesStreamContentPart{{
				Type:    "refusal",
				Refusal: w.refusal.String(),
			}},
		},
	})
}

func (w *responsesAdapterStreamWriter) finishToolCallItems() {
	for _, state := range w.orderedToolCallStates() {
		if state == nil || state.Done {
			continue
		}
		state.Done = true
		w.writeResponsesEvent("response.function_call_arguments.done", responsesStreamFunctionCallArgumentsDoneEvent{
			Type:        "response.function_call_arguments.done",
			ResponseID:  w.respID,
			ItemID:      state.CallID,
			OutputIndex: state.OutputIndex,
		})
		w.writeResponsesEvent("response.output_item.done", responsesStreamOutputItemEvent{
			Type:        "response.output_item.done",
			ResponseID:  w.respID,
			OutputIndex: state.OutputIndex,
			Item: responsesStreamFunctionCallItem{
				ID:        state.CallID,
				Type:      "function_call",
				CallID:    state.CallID,
				Name:      state.Name,
				Arguments: state.Arguments.String(),
				Status:    "completed",
			},
		})
	}
}

func (w *responsesAdapterStreamWriter) nextOutputIndex() int {
	index := w.nextOutputIdx
	w.nextOutputIdx++
	return index
}

func (w *responsesAdapterStreamWriter) responseWithStatus(status string) responsesStreamResponse {
	return responsesStreamResponse{
		ID:        w.respID,
		Object:    "response",
		CreatedAt: w.created,
		Status:    status,
		Model:     w.model,
	}
}

func (w *responsesAdapterStreamWriter) completedResponse() responsesStreamResponse {
	response := w.responseWithStatus("completed")
	var outputItems []struct {
		index int
		item  any
	}
	if w.textStarted {
		response.OutputText = w.text.String()
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: w.textOutputIdx, item: responsesStreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsesStreamContentPart{{
				Type: "output_text",
				Text: w.text.String(),
			}},
		}})
	}
	if w.refusalStarted {
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: w.refusalOutputIdx, item: responsesStreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsesStreamContentPart{{
				Type:    "refusal",
				Refusal: w.refusal.String(),
			}},
		}})
	}
	for _, state := range w.orderedToolCallStates() {
		if state == nil {
			continue
		}
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: state.OutputIndex, item: responsesStreamFunctionCallItem{
			ID:        state.CallID,
			Type:      "function_call",
			CallID:    state.CallID,
			Name:      state.Name,
			Arguments: state.Arguments.String(),
			Status:    "completed",
		}})
	}
	if len(outputItems) > 0 {
		sort.Slice(outputItems, func(i, j int) bool {
			return outputItems[i].index < outputItems[j].index
		})
		output := make([]any, 0, len(outputItems))
		for _, outputItem := range outputItems {
			output = append(output, outputItem.item)
		}
		response.Output = output
	}
	if w.usage != nil {
		response.Usage = w.usage
	}
	return response
}

func (w *responsesAdapterStreamWriter) orderedToolCallStates() []*responsesToolCallStreamState {
	states := make([]*responsesToolCallStreamState, 0, len(w.toolCallItems))
	for _, state := range w.toolCallItems {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].OutputIndex < states[j].OutputIndex
	})
	return states
}

func (w *responsesAdapterStreamWriter) writeResponsesEvent(event string, payload any) {
	w.captureResponsesCounterEvent(payload)
	w.eventBuf.Reset()
	w.eventBuf.WriteString("event: ")
	w.eventBuf.WriteString(event)
	w.eventBuf.WriteString("\n")
	w.eventBuf.WriteString("data: ")
	encoder := json.NewEncoder(&w.eventBuf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return
	}
	w.eventBuf.WriteByte('\n')
	_, _ = w.ginWriter.Write(w.eventBuf.Bytes())
	w.ginWriter.Flush()
}

func (w *responsesAdapterStreamWriter) captureResponsesCounterEvent(payload any) {
	if w.responsesCounter == nil || payload == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var event types.ResponsesStreamEvent
	if err := json.Unmarshal(data, &event); err == nil && event.Type != "" {
		w.responsesCounter.AppendEvent(event)
	}
}

func (w *responsesAdapterStreamWriter) writeData(data string) {
	if !strings.HasSuffix(data, "\n\n") {
		data += "\n\n"
	}
	_, _ = w.ginWriter.Write([]byte(data))
	w.ginWriter.Flush()
}

func (w *responsesAdapterStreamWriter) writeRawEvent(event *streamdecoder.Event) {
	if event == nil || len(event.Raw) == 0 {
		return
	}
	_, _ = w.ginWriter.Write(event.Raw)
	w.ginWriter.Flush()
}
