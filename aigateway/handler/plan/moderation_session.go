package plan

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"opencsg.com/csghub-server/aigateway/types"
)

// ensureModerationSessionID generates the per-request moderation session id
// when the prompt content-safety check is about to run, and carries it on the
// request context. An asynchronous prompt moderation verdict is recorded
// under this id by the moderation component, so the response-side checks of
// the same request can gate on it without threading the id through
// signatures. Requests that never run the check get no id — there is no
// verdict to correlate.
func ensureModerationSessionID(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if types.ModerationSessionIDFromContext(ctx) == "" {
		ctx = types.WithModerationSessionID(ctx, uuid.NewString())
		c.Request = c.Request.WithContext(ctx)
	}
	return ctx
}
