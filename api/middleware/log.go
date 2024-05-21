package middleware

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
)

func Log() gin.HandlerFunc {
	lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
	l := slog.New(lh)
	return func(ctx *gin.Context) {
		startTime := time.Now()

		ctx.Next()

		latency := time.Since(startTime).Milliseconds()
		l.InfoContext(ctx, "http request", slog.String("ip", ctx.ClientIP()),
			slog.String("method", ctx.Request.Method),
			slog.Int("latency(ms)", int(latency)),
			slog.Int("status", ctx.Writer.Status()),
			slog.String("current_user", httpbase.GetCurrentUser(ctx)),
			slog.Any("auth_type", httpbase.GetAuthType(ctx)),
			slog.String("url", ctx.Request.URL.RequestURI()),
		)
	}
}
