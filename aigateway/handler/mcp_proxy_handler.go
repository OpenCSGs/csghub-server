//go:build ee || saas

package handler

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"opencsg.com/csghub-server/common/utils/trace"
)

const (
	headerInternalProxy = "X-Internal-Proxy"
	proxyConnectTimeout = 3 * time.Second
)

// MCPProxyAwareHandler wraps the local SDK handler with Redis-based session
// routing. Requests for sessions owned by a remote instance are transparently
// proxied; new sessions are registered in Redis after the SDK assigns an ID.
type MCPProxyAwareHandler struct {
	sdkHandler http.Handler
	registry   MCPSessionRegistry
	selfAddr   string
}

func NewMCPProxyAwareHandler(sdkHandler http.Handler, registry MCPSessionRegistry, selfAddr string) *MCPProxyAwareHandler {
	return &MCPProxyAwareHandler{
		sdkHandler: sdkHandler,
		registry:   registry,
		selfAddr:   selfAddr,
	}
}

func (h *MCPProxyAwareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(trace.HeaderMcpSessionID)

	// No session ID: new session (initialize). Capture the session ID from the
	// response and register it in Redis.
	if sessionID == "" {
		cw := &sessionCapturingWriter{
			ResponseWriter: w,
			onSessionCreated: func(sid string) {
				if err := h.registry.Register(r.Context(), sid, h.selfAddr); err != nil {
					slog.ErrorContext(r.Context(), "failed to register new mcp session in redis", "session_id", sid, "error", err)
				} else {
					slog.InfoContext(r.Context(), "registered new mcp session", "session_id", sid, "instance", h.selfAddr)
				}
			},
		}
		h.sdkHandler.ServeHTTP(cw, r)
		return
	}

	// Internal proxy request: already routed by another instance, handle locally.
	if r.Header.Get(headerInternalProxy) == "true" {
		h.sdkHandler.ServeHTTP(w, r)
		return
	}

	// Existing session: look up which instance owns it.
	targetAddr, err := h.registry.Lookup(r.Context(), sessionID)
	if err != nil {
		slog.DebugContext(r.Context(), "mcp session not found in redis, trying local", "session_id", sessionID, "error", err)
		h.sdkHandler.ServeHTTP(w, r)
		return
	}

	// Session is local.
	if targetAddr == h.selfAddr {
		h.sdkHandler.ServeHTTP(w, r)
		return
	}

	// Session is on a remote instance: proxy the request.
	slog.InfoContext(r.Context(), "proxy to remote instance", "session_id", sessionID, "target", targetAddr)
	proxyErr := h.proxyToInstance(w, r, targetAddr)
	if proxyErr != nil {
		slog.WarnContext(r.Context(), "proxy to remote instance failed, cleaning stale session",
			"session_id", sessionID, "target", targetAddr, "error", proxyErr)
		_ = h.registry.Delete(r.Context(), sessionID)
		http.Error(w, "session not found", http.StatusNotFound)
	}
}

func (h *MCPProxyAwareHandler) proxyToInstance(w http.ResponseWriter, r *http.Request, targetAddr string) error {
	targetURL, err := url.Parse(fmt.Sprintf("http://%s", targetAddr))
	if err != nil {
		return fmt.Errorf("parse target url: %w", err)
	}

	var proxyErr atomic.Value
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.FlushInterval = -1 // immediate flush for SSE streaming
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{Timeout: proxyConnectTimeout}).DialContext,
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		proxyErr.Store(e)
	}

	r.Header.Set(headerInternalProxy, "true")
	proxy.ServeHTTP(w, r)

	if stored := proxyErr.Load(); stored != nil {
		return stored.(error)
	}
	return nil
}

// sessionCapturingWriter intercepts WriteHeader to capture the Mcp-Session-Id
// set by the SDK on initialize responses, then calls onSessionCreated.
type sessionCapturingWriter struct {
	http.ResponseWriter
	onSessionCreated func(sessionID string)
	captured         bool
}

func (w *sessionCapturingWriter) WriteHeader(code int) {
	if !w.captured {
		w.captured = true
		if sid := w.Header().Get(trace.HeaderMcpSessionID); sid != "" {
			w.onSessionCreated(sid)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionCapturingWriter) Write(b []byte) (int, error) {
	if !w.captured {
		w.captured = true
		if sid := w.Header().Get(trace.HeaderMcpSessionID); sid != "" {
			w.onSessionCreated(sid)
		}
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for SSE streaming compatibility.
func (w *sessionCapturingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap allows http.ResponseController to reach the underlying writer.
func (w *sessionCapturingWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
