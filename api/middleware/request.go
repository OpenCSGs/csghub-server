package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/utils/trace"
)

func Request() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("clientIP", c.ClientIP())
		reqCtx := context.WithValue(c.Request.Context(), "clientIP", c.ClientIP())
		c.Request = c.Request.WithContext(reqCtx)
		traceID := trace.GetOrGenTraceID(c)
		c.Writer.Header().Set(trace.HeaderRequestID, traceID)

		sessionID := c.GetHeader(trace.HeaderXetSessionID)
		if sessionID != "" {
			ctx := trace.SetSessionIDInContext(c.Request.Context(), sessionID)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}
