package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/streamdecoder"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

type responsesAdapterStreamWriter struct {
	ginWriter          gin.ResponseWriter
	decoder            streamdecoder.Decoder
	respID             string
	model              string
	created            int64
	eventBuf           bytes.Buffer
	started            bool
	completed          bool
	failed             bool
	passthrough        bool
	textStarted        bool
	textDone           bool
	textOutputIdx      int
	textItemID         string
	text               strings.Builder
	refusalStarted     bool
	refusalDone        bool
	refusalOutputIdx   int
	refusalItemID      string
	refusal            strings.Builder
	reasoningStarted   bool
	reasoningDone      bool
	reasoningOutputIdx int
	reasoningItemID    string
	reasoning          strings.Builder
	nextOutputIdx      int
	responsesCounter   token.ResponsesTokenCounter
	usage              *types.ResponsesUsage
	toolCallItems      map[int]*responsesToolCallStreamState
	moderation         component.Moderation
	sessionID          string
	logCapture         *responsespkg.LLMLogRecorder
}

type responsesToolCallStreamState struct {
	OutputIndex int
	CallID      string
	Name        string
	Arguments   strings.Builder
	Done        bool
}

func newResponsesAdapterStreamWriter(w gin.ResponseWriter, model string, responsesCounter token.ResponsesTokenCounter, moderation component.Moderation, sessionID string, logCapture ...*responsespkg.LLMLogRecorder) *responsesAdapterStreamWriter {
	var recorder *responsespkg.LLMLogRecorder
	if len(logCapture) > 0 {
		recorder = logCapture[0]
	}
	return &responsesAdapterStreamWriter{
		ginWriter:        w,
		respID:           responsespkg.NewAdapterResponseID(),
		model:            model,
		created:          time.Now().Unix(),
		decoder:          streamdecoder.NewSSE(),
		responsesCounter: responsesCounter,
		toolCallItems:    map[int]*responsesToolCallStreamState{},
		moderation:       moderation,
		sessionID:        sessionID,
		logCapture:       recorder,
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
		w.cleanupStreamModeration()
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
			ctx, cancel := responsespkg.ModerationContext()
			w.finishStreamChecks(ctx)
			cancel()
			w.finishResponseStream()
			continue
		}
		var chunk types.ChatCompletionChunk
		if err := json.Unmarshal(event.Data, &chunk); err != nil {
			w.writeData(string(event.Data))
			continue
		}
		w.captureChatStreamUsage(chunk)
		ctx, cancel := responsespkg.ModerationContext()
		sensitive := w.checkStreamSensitive(ctx, chunk)
		cancel()
		if sensitive {
			return len(data), nil
		}
		w.ensureStarted()
		w.writeToolCallDeltas(chunk)
		for _, choice := range chunk.Choices {
			if reasoning := chatDeltaReasoning(choice.Delta); reasoning != "" {
				w.ensureReasoningItem()
				w.reasoning.WriteString(reasoning)
				w.writeResponsesEvent("response.reasoning_summary_text.delta", responsespkg.StreamReasoningSummaryDeltaEvent{
					Type:         "response.reasoning_summary_text.delta",
					ResponseID:   w.respID,
					ItemID:       w.reasoningItemID,
					OutputIndex:  w.reasoningOutputIdx,
					SummaryIndex: 0,
					Delta:        reasoning,
				})
			}
			if choice.Delta.Refusal != "" {
				w.ensureRefusalItem()
				w.refusal.WriteString(choice.Delta.Refusal)
				w.writeResponsesEvent("response.refusal.delta", responsespkg.StreamRefusalDeltaEvent{
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
				w.writeResponsesEvent("response.output_text.delta", responsespkg.StreamOutputTextDeltaEvent{
					Type:         "response.output_text.delta",
					ResponseID:   w.respID,
					ItemID:       w.textItemID,
					OutputIndex:  w.textOutputIdx,
					ContentIndex: 0,
					Delta:        choice.Delta.Content,
				})
			}
			if choice.FinishReason != "" {
				if w.logCapture != nil {
					w.logCapture.CaptureFinishReason(choice.FinishReason)
				}
				if choice.FinishReason == "tool_calls" {
					w.finishToolCallItems()
				} else {
					w.finishReasoningItem()
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
	w.finishReasoningItem()
	w.completed = true
	w.captureCompletedStreamLog()
	w.writeResponsesEvent("response.completed", responsespkg.StreamResponseEvent{
		Type:     "response.completed",
		Response: w.completedResponse(),
	})
	w.writeData("data: [DONE]")
}

func (w *responsesAdapterStreamWriter) captureCompletedStreamLog() {
	if w.logCapture == nil {
		return
	}
	w.logCapture.CaptureResponseID(w.respID)
	if w.textStarted && w.text.Len() > 0 {
		w.logCapture.CaptureOutputTextDone(w.text.String())
	}
	if w.refusalStarted && w.refusal.Len() > 0 {
		w.logCapture.CaptureRefusalDone(w.refusal.String())
	}
	for _, state := range w.orderedToolCallStates() {
		if state == nil {
			continue
		}
		w.logCapture.CaptureToolCallStart(state.CallID, state.Name, state.Arguments.String())
	}
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
			if w.logCapture != nil {
				w.logCapture.CaptureToolCallStart(state.CallID, state.Name, "")
			}
			if args := call.Function.Arguments; args != "" {
				state.Arguments.WriteString(args)
				if w.logCapture != nil {
					w.logCapture.CaptureToolCallArgumentsDelta(state.CallID, args)
				}
				w.writeResponsesEvent("response.function_call_arguments.delta", responsespkg.StreamFunctionCallArgumentsDeltaEvent{
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
	if w.logCapture != nil {
		w.logCapture.CaptureResponseID(w.respID)
		w.logCapture.CaptureToolCallStart(itemID, name, "")
	}
	w.writeResponsesEvent("response.output_item.added", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: state.OutputIndex,
		Item: responsespkg.StreamFunctionCallItem{
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
	w.writeResponsesEvent("response.created", responsespkg.StreamResponseEvent{
		Type:     "response.created",
		Response: w.responseWithStatus("in_progress"),
	})
	w.writeResponsesEvent("response.in_progress", responsespkg.StreamResponseEvent{
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
	w.writeResponsesEvent("response.output_item.added", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: w.textOutputIdx,
		Item: responsespkg.StreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "in_progress",
		},
	})
	w.writeResponsesEvent("response.content_part.added", responsespkg.StreamContentPartEvent{
		Type:         "response.content_part.added",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Part: responsespkg.StreamContentPart{
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
	w.writeResponsesEvent("response.output_item.added", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: w.refusalOutputIdx,
		Item: responsespkg.StreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "in_progress",
		},
	})
	w.writeResponsesEvent("response.content_part.added", responsespkg.StreamContentPartEvent{
		Type:         "response.content_part.added",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Part: responsespkg.StreamContentPart{
			Type:    "refusal",
			Refusal: "",
		},
	})
}

func (w *responsesAdapterStreamWriter) ensureReasoningItem() {
	if w.reasoningStarted {
		return
	}
	w.reasoningStarted = true
	w.reasoningOutputIdx = w.nextOutputIndex()
	w.reasoningItemID = fmt.Sprintf("rs_%d", w.reasoningOutputIdx)
	w.writeResponsesEvent("response.output_item.added", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.added",
		ResponseID:  w.respID,
		OutputIndex: w.reasoningOutputIdx,
		Item: responsespkg.StreamReasoningItem{
			ID:     w.reasoningItemID,
			Type:   "reasoning",
			Status: "in_progress",
		},
	})
}

func (w *responsesAdapterStreamWriter) finishTextItem() {
	if !w.textStarted || w.textDone {
		return
	}
	w.textDone = true
	if w.logCapture != nil {
		w.logCapture.CaptureResponseID(w.respID)
		w.logCapture.CaptureOutputTextDone(w.text.String())
	}
	w.writeResponsesEvent("response.output_text.done", responsespkg.StreamOutputTextDoneEvent{
		Type:         "response.output_text.done",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Text:         w.text.String(),
	})
	w.writeResponsesEvent("response.content_part.done", responsespkg.StreamContentPartEvent{
		Type:         "response.content_part.done",
		ResponseID:   w.respID,
		ItemID:       w.textItemID,
		OutputIndex:  w.textOutputIdx,
		ContentIndex: 0,
		Part: responsespkg.StreamContentPart{
			Type: "output_text",
			Text: w.text.String(),
		},
	})
	w.writeResponsesEvent("response.output_item.done", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.done",
		ResponseID:  w.respID,
		OutputIndex: w.textOutputIdx,
		Item: responsespkg.StreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsespkg.StreamContentPart{{
				Type: "output_text",
				Text: w.text.String(),
			}},
		},
	})
}

func (w *responsesAdapterStreamWriter) finishReasoningItem() {
	if !w.reasoningStarted || w.reasoningDone {
		return
	}
	w.reasoningDone = true
	w.writeResponsesEvent("response.reasoning_summary_text.done", responsespkg.StreamReasoningSummaryDoneEvent{
		Type:         "response.reasoning_summary_text.done",
		ResponseID:   w.respID,
		ItemID:       w.reasoningItemID,
		OutputIndex:  w.reasoningOutputIdx,
		SummaryIndex: 0,
		Part: responsespkg.StreamReasoningSummaryPart{
			Type: "summary_text",
			Text: w.reasoning.String(),
		},
	})
	w.writeResponsesEvent("response.output_item.done", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.done",
		ResponseID:  w.respID,
		OutputIndex: w.reasoningOutputIdx,
		Item:        w.reasoningOutput("completed"),
	})
}

func (w *responsesAdapterStreamWriter) finishRefusalItem() {
	if !w.refusalStarted || w.refusalDone {
		return
	}
	w.refusalDone = true
	if w.logCapture != nil {
		w.logCapture.CaptureResponseID(w.respID)
		w.logCapture.CaptureRefusalDone(w.refusal.String())
	}
	w.writeResponsesEvent("response.refusal.done", responsespkg.StreamRefusalDoneEvent{
		Type:         "response.refusal.done",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Refusal:      w.refusal.String(),
	})
	w.writeResponsesEvent("response.content_part.done", responsespkg.StreamContentPartEvent{
		Type:         "response.content_part.done",
		ResponseID:   w.respID,
		ItemID:       w.refusalItemID,
		OutputIndex:  w.refusalOutputIdx,
		ContentIndex: 0,
		Part: responsespkg.StreamContentPart{
			Type:    "refusal",
			Refusal: w.refusal.String(),
		},
	})
	w.writeResponsesEvent("response.output_item.done", responsespkg.StreamOutputItemEvent{
		Type:        "response.output_item.done",
		ResponseID:  w.respID,
		OutputIndex: w.refusalOutputIdx,
		Item: responsespkg.StreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsespkg.StreamContentPart{{
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
		if w.logCapture != nil {
			w.logCapture.CaptureResponseID(w.respID)
			w.logCapture.CaptureToolCallStart(state.CallID, state.Name, state.Arguments.String())
		}
		w.writeResponsesEvent("response.function_call_arguments.done", responsespkg.StreamFunctionCallArgumentsDoneEvent{
			Type:        "response.function_call_arguments.done",
			ResponseID:  w.respID,
			ItemID:      state.CallID,
			OutputIndex: state.OutputIndex,
		})
		w.writeResponsesEvent("response.output_item.done", responsespkg.StreamOutputItemEvent{
			Type:        "response.output_item.done",
			ResponseID:  w.respID,
			OutputIndex: state.OutputIndex,
			Item: responsespkg.StreamFunctionCallItem{
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

func (w *responsesAdapterStreamWriter) responseWithStatus(status string) responsespkg.StreamResponse {
	return responsespkg.StreamResponse{
		ID:        w.respID,
		Object:    "response",
		CreatedAt: w.created,
		Status:    status,
		Model:     w.model,
	}
}

func (w *responsesAdapterStreamWriter) completedResponse() responsespkg.StreamResponse {
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
		}{index: w.textOutputIdx, item: responsespkg.StreamMessageItem{
			ID:     w.textItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsespkg.StreamContentPart{{
				Type: "output_text",
				Text: w.text.String(),
			}},
		}})
	}
	if w.refusalStarted {
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: w.refusalOutputIdx, item: responsespkg.StreamMessageItem{
			ID:     w.refusalItemID,
			Type:   "message",
			Role:   "assistant",
			Status: "completed",
			Content: []responsespkg.StreamContentPart{{
				Type:    "refusal",
				Refusal: w.refusal.String(),
			}},
		}})
	}
	if w.reasoningStarted {
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: w.reasoningOutputIdx, item: w.reasoningOutput("completed")})
	}
	for _, state := range w.orderedToolCallStates() {
		if state == nil {
			continue
		}
		outputItems = append(outputItems, struct {
			index int
			item  any
		}{index: state.OutputIndex, item: responsespkg.StreamFunctionCallItem{
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

func (w *responsesAdapterStreamWriter) reasoningOutput(status string) responsespkg.StreamReasoningItem {
	return responsespkg.StreamReasoningItem{
		ID:     w.reasoningItemID,
		Type:   "reasoning",
		Status: status,
		Summary: []responsespkg.StreamReasoningSummaryPart{{
			Type: "summary_text",
			Text: w.reasoning.String(),
		}},
	}
}

func chatDeltaReasoning(delta types.ChatCompletionChunkChoiceDelta) string {
	if delta.ReasoningContent != "" {
		return delta.ReasoningContent
	}
	return delta.Reasoning
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

// checkStreamSensitive runs the per-chunk sensitive-content check on the
// delta content of the upstream chat chunk. Returns true if the writer should
// stop forwarding the rest of the stream (sensitive content detected) and
// false otherwise. Nil-safe — returns false when no moderation component is
// wired (NeedSensitiveCheck == false path).
func (w *responsesAdapterStreamWriter) checkStreamSensitive(ctx context.Context, chunk types.ChatCompletionChunk) bool {
	if w.moderation == nil || len(chunk.Choices) == 0 {
		return false
	}
	content := chatChunkModerationText(chunk)
	if strings.TrimSpace(content) == "" {
		return false
	}
	result, err := w.moderation.CheckText(ctx, types.TextModerationRequest{
		Content: content,
		Key:     w.sessionID,
		Phase:   types.TextModerationPhaseResponse,
		Mode:    types.TextModerationModeStream,
	})
	if err != nil || result == nil {
		return false
	}
	if result.IsSensitive {
		w.failStreamSensitive()
		return true
	}
	return false
}

func chatChunkModerationText(chunk types.ChatCompletionChunk) string {
	var b strings.Builder
	for _, choice := range chunk.Choices {
		writeModerationText(&b, choice.Delta.Content)
		writeModerationText(&b, choice.Delta.ReasoningContent)
		writeModerationText(&b, choice.Delta.Refusal)
		for _, call := range choice.Delta.ToolCalls {
			writeModerationText(&b, call.Function.Name)
			writeModerationText(&b, call.Function.Arguments)
		}
	}
	return b.String()
}

func writeModerationText(b *strings.Builder, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	b.WriteString(text)
}

// finishStreamChecks flushes the async per-session sensitive-content check
// buffer at stream end. The async checker (when configured) may surface a
// sensitive verdict on the buffered text; if so we transition the writer to
// the failed state so the final response is the canned blocked message.
func (w *responsesAdapterStreamWriter) finishStreamChecks(ctx context.Context) {
	if w.moderation == nil {
		return
	}
	result, err := w.moderation.CloseStreamCheck(ctx, w.sessionID)
	if err != nil || result == nil {
		return
	}
	if result.IsSensitive {
		w.failStreamSensitive()
	}
}

// failStreamSensitive terminates the stream in the failed state so the next
// event emitted is the canned blocked Responses response instead of further
// chat-shaped content. Mirrors failResponseStream but uses the canned
// response for Responses shape.
func (w *responsesAdapterStreamWriter) failStreamSensitive() {
	if w.completed || w.failed {
		return
	}
	w.failed = true
	responsespkg.WriteSensitiveStreamEvent(w.ginWriter)
}

// cleanupStreamModeration removes the async moderation session from the LRU
// cache on abnormal stream termination. This must be called whenever the
// stream ends without going through the normal [DONE] path (e.g. upstream
// error, passthrough, or mid-stream sensitive detection). Without this call
// the session state accumulates in the async checker's cache indefinitely.
func (w *responsesAdapterStreamWriter) cleanupStreamModeration() {
	if w.moderation == nil {
		return
	}
	ctx, cancel := responsespkg.ModerationContext()
	defer cancel()
	_, _ = w.moderation.CloseStreamCheck(ctx, w.sessionID)
}
