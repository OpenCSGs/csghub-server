package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	bld "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/common/utils/trace"
)

// Recovery returns a middleware that recovers from any panics and writes a 500 if there was one.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Increment the panic counter
				if bld.HttpPanicsTotal != nil {
					bld.HttpPanicsTotal.Inc()
				}
				// Get trace ID
				traceID := trace.GetTraceIDInGinContext(c)
				slog.ErrorContext(c.Request.Context(), "[Recovery from panic]",
					slog.Time("time", time.Now()),
					slog.String("trace_id", traceID),
					slog.String("method", c.Request.Method),
					slog.String("url", c.Request.URL.RequestURI()),
					slog.String("full_path", c.FullPath()),
					slog.Any("error", err),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
