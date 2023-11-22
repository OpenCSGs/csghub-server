package httpbase

import (
	"context"
	"fmt"
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
)

// GracefulServer implements an HTTP server with graceful shutdown.
// Graceful shutdown is actually hard to implement correctly
// due to an API design flaw of the Go http package,
// ref: https://nanmu.me/zh-cn/posts/2021/go-http-server-shudown-done-right/
type GracefulServer struct {
	server *http.Server
	logger log.Logger
	closed chan struct{}
}

type GraceServerOpt struct {
	Logger log.Logger
	Port   int
}

// NewGracefulServer returns a server with graceful shutdown
func NewGracefulServer(opt GraceServerOpt, handler http.Handler) (server *GracefulServer) {
	server = &GracefulServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", opt.Port),
			Handler: handler,
		},
		logger: opt.Logger,
		closed: make(chan struct{}),
	}
	return
}

// Shutdown trys to gracefully shutdowns the server
// before the provided context expires or gets canceled.
func (s *GracefulServer) Shutdown(ctx context.Context) (err error) {
	defer close(s.closed)

	err = s.server.Shutdown(ctx)
	if err != nil {
		err = fmt.Errorf("server.Shutdown: %w", err)
		s.logger.Error("graceful shutdown failed.",
			log.ErrField(err),
			log.String("addr", s.server.Addr),
		)
		return
	}

	s.logger.Info("HTTP service exited successfully.",
		log.String("addr", s.server.Addr),
	)

	return
}

// ListenAndServe listens and handles requests on incoming connections.
// It blocks the current goroutine.
func (s *GracefulServer) ListenAndServe() (err error) {
	err = s.server.ListenAndServe()
	if err != http.ErrServerClosed {
		err = fmt.Errorf("server stopped unexpectedly: %w", err)
		return
	}

	// ListenAndServe always returns a non-nil error.
	// After Shutdown or Close, the returned error is ErrServerClosed.
	err = nil

	<-s.closed

	return
}
