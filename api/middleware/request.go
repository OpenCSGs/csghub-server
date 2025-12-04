package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/utils/trace"
)

func Request() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set("clientIP", ctx.ClientIP())
		reqCtx := context.WithValue(ctx.Request.Context(), "clientIP", ctx.ClientIP())
		ctx.Request = ctx.Request.WithContext(reqCtx)
		traceID := trace.GetOrGenTraceID(ctx)
		ctx.Writer.Header().Set(trace.HeaderRequestID, traceID)
		ctx.Next()
	}
}
