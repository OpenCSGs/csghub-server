package responses

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

const BlockedMessage = "The prompt includes inappropriate content and has been blocked. We appreciate your understanding and cooperation."

const ModerationTimeout = 5 * time.Second

func ModerationContext() (context.Context, context.CancelFunc) {
	// Deliberately detached from the request so stream moderation cleanup can
	// finish after a client disconnect, bounded by responsesModerationTimeout.
	return context.WithTimeout(context.Background(), ModerationTimeout)
}

// HandleSensitiveResponse mirrors handleSensitiveResponse for the
// Responses API. It writes either a single SSE `response.completed` event
// (stream) or a 200 OK JSON ResponsesResponse (non-stream) carrying the
// canned blocked message, and never propagates the upstream.
func HandleSensitiveResponse(c *gin.Context, stream bool, checkResult *rpc.CheckResult) {
	var reason any
	if checkResult != nil {
		reason = checkResult.Reason
	}
	slog.DebugContext(
		c.Request.Context(),
		"sensitive content detected in responses request",
		slog.Any("reason", reason),
	)
	if stream {
		writeSensitiveStreamResponse(c)
		return
	}
	writeSensitiveJSONResponse(c)
}

func writeSensitiveStreamResponse(c *gin.Context) {
	resp := types.ResponsesResponse{
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: BlockedMessage,
			}},
		}},
	}
	body, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "marshal sensitive responses response", slog.Any("error", err))
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte("event: response.completed\n"))
	_, _ = c.Writer.Write([]byte("data: "))
	_, _ = c.Writer.Write(body)
	_, _ = c.Writer.Write([]byte("\n\n"))
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	c.Writer.Flush()
}

// WriteSensitiveStreamEvent is the same canned payload as
// writeSensitiveStreamResponse but writes to a raw gin.ResponseWriter
// mid-stream (no header/status mutation, no slog on marshal failure) so the
// stream writers can terminate an ongoing response when moderation flags
// content. The header has already been set by the writer's WriteHeader.
func WriteSensitiveStreamEvent(w gin.ResponseWriter) {
	resp := types.ResponsesResponse{
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: BlockedMessage,
			}},
		}},
	}
	body, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("event: response.completed\n"))
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n\n"))
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	w.Flush()
}

func writeSensitiveJSONResponse(c *gin.Context) {
	c.JSON(http.StatusOK, types.ResponsesResponse{
		Object: "response",
		Status: "completed",
		Output: []types.ResponsesOutputItem{{
			Type:   "message",
			Status: "completed",
			Role:   "assistant",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: BlockedMessage,
			}},
		}},
	})
}
