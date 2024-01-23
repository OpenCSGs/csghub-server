package middleware

import (
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

func Log() gin.HandlerFunc {
	lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
	l := slog.New(lh)
	return func(ctx *gin.Context) {
		l.InfoContext(ctx, "http request", slog.String("ip", ctx.ClientIP()),
			slog.String("method", ctx.Request.Method),
			slog.String("url", ctx.Request.URL.RequestURI()),
		)
	}
}
