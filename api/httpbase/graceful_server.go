package httpbase

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// GracefulServer implements an HTTP server with graceful shutdown.
// Graceful shutdown is actually hard to implement correctly
// due to an API design flaw of the Go http package,
// ref: https://nanmu.me/zh-cn/posts/2021/go-http-server-shudown-done-right/
type GracefulServer struct {
	server *http.Server
}

type GraceServerOpt struct {
	Port int
}

// NewGracefulServer returns a server with graceful shutdown
func NewGracefulServer(opt GraceServerOpt, handler http.Handler) (server *GracefulServer) {
	server = &GracefulServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", opt.Port),
			Handler: handler,
		},
	}
	return
}

// Run start the http server and block
func (s *GracefulServer) Run() {
	q := make(chan os.Signal, 1)
	signal.Notify(q, syscall.SIGINT, syscall.SIGTERM)

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			//notify server to stop
			q <- syscall.SIGTERM

			slog.Error("listen failed", slog.Any("error", err))
		}
	}()

	// Listen for the interrupt signal.
	<-q

	slog.Info("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		slog.Error("Server faild to shutdown", slog.Any("error", err))
	}

	slog.Info("Server stopped")
}
