package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestCaptureResponsesCounterEventForwardsStreamEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	var mu sync.Mutex
	var events []types.ResponsesStreamEvent
	counter := &streamCaptureCounter{
		onAppend: func(e types.ResponsesStreamEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		},
	}

	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", counter)
	w.captureResponsesCounterEvent(responsesStreamOutputTextDeltaEvent{
		Type:         "response.output_text.delta",
		ResponseID:   w.respID,
		ItemID:       "msg_1",
		OutputIndex:  0,
		ContentIndex: 0,
		Delta:        "hello",
	})

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 1)
	require.Equal(t, "response.output_text.delta", events[0].Type)
}

func TestCaptureResponsesCounterEventSkipsPayloadsWithoutType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	var called int
	var mu sync.Mutex
	counter := &streamCaptureCounter{
		onAppend: func(_ types.ResponsesStreamEvent) {
			mu.Lock()
			called++
			mu.Unlock()
		},
	}

	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", counter)
	w.captureResponsesCounterEvent(map[string]any{"not_a_type": true})

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 0, called)
}

func TestCaptureResponsesCounterEventHandlesNilCounter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
	require.NotPanics(t, func() {
		w.captureResponsesCounterEvent(responsesStreamOutputTextDeltaEvent{Type: "response.output_text.delta"})
	})
}

func TestCaptureResponsesCounterEventHandlesNilPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	counter := &streamCaptureCounter{}
	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", counter)
	require.NotPanics(t, func() {
		w.captureResponsesCounterEvent(nil)
	})
	require.Equal(t, 0, counter.count())
}

type streamCaptureCounter struct {
	mu       sync.Mutex
	events   []types.ResponsesStreamEvent
	onAppend func(types.ResponsesStreamEvent)
}

func (c *streamCaptureCounter) Request(_ *types.ResponsesRequest)   {}
func (c *streamCaptureCounter) Response(_ *types.ResponsesResponse) {}
func (c *streamCaptureCounter) AppendEvent(e types.ResponsesStreamEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	if c.onAppend != nil {
		c.onAppend(e)
	}
}
func (c *streamCaptureCounter) Usage(_ context.Context) (*token.Usage, error) {
	return nil, nil
}

func (c *streamCaptureCounter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func TestResponsesAdapterStreamWriterPassthroughUpstreamJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, err := w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
	require.NoError(t, err)
	require.NoError(t, w.Finalize(http.StatusTooManyRequests))

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), `"code":"rate_limit_exceeded"`)
	require.NotContains(t, rec.Body.String(), "response.completed")
	require.NotContains(t, rec.Body.String(), "[DONE]")
}

func TestResponsesAdapterStreamWriterFinalizeSkipsCompletionOnErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterStreamWriter(ctx.Writer, "public-model", nil)
	require.NoError(t, w.Finalize(http.StatusBadGateway))

	require.NotContains(t, rec.Body.String(), "response.completed")
	require.NotContains(t, rec.Body.String(), "[DONE]")
}
