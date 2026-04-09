//go:build ee || saas

package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"opencsg.com/csghub-server/common/utils/trace"
)

type stubRegistry struct {
	store map[string]string
	err   error
}

func (s *stubRegistry) Register(_ context.Context, sessionID, addr string) error {
	if s.store == nil {
		s.store = make(map[string]string)
	}
	s.store[sessionID] = addr
	return nil
}
func (s *stubRegistry) Lookup(_ context.Context, sessionID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	addr, ok := s.store[sessionID]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return addr, nil
}
func (s *stubRegistry) Delete(_ context.Context, sessionID string) error {
	delete(s.store, sessionID)
	return nil
}
func (s *stubRegistry) DeleteByInstance(_ context.Context, addr string) error {
	for k, v := range s.store {
		if v == addr {
			delete(s.store, k)
		}
	}
	return nil
}

func TestMCPProxyAwareHandler_NoSessionID_CaptureAndRegister(t *testing.T) {
	t.Parallel()
	registry := &stubRegistry{store: make(map[string]string)}
	selfAddr := "127.0.0.1:8094"
	newSessionID := "new-session-abc"

	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(trace.HeaderMcpSessionID, newSessionID)
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if registry.store[newSessionID] != selfAddr {
		t.Fatalf("expected session %q to be registered to %q, got %q", newSessionID, selfAddr, registry.store[newSessionID])
	}
}

func TestMCPProxyAwareHandler_LocalSession(t *testing.T) {
	t.Parallel()
	selfAddr := "127.0.0.1:8094"
	sessionID := "local-session-1"
	registry := &stubRegistry{store: map[string]string{sessionID: selfAddr}}

	called := false
	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	req.Header.Set(trace.HeaderMcpSessionID, sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected local SDK handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMCPProxyAwareHandler_InternalProxy(t *testing.T) {
	t.Parallel()
	selfAddr := "127.0.0.1:8094"
	registry := &stubRegistry{store: make(map[string]string)}

	called := false
	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	req.Header.Set(trace.HeaderMcpSessionID, "some-session")
	req.Header.Set(headerInternalProxy, "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected SDK handler to be called for internal proxy request")
	}
}

func TestMCPProxyAwareHandler_RemoteProxyFallback(t *testing.T) {
	t.Parallel()
	selfAddr := "127.0.0.1:8094"
	sessionID := "remote-session-1"
	registry := &stubRegistry{store: map[string]string{sessionID: "192.168.1.99:9999"}}

	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	req.Header.Set(trace.HeaderMcpSessionID, sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Remote target is unreachable, so proxy should fail and return 404 after cleanup
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after failed proxy, got %d", rec.Code)
	}
	if _, ok := registry.store[sessionID]; ok {
		t.Fatal("expected stale session to be deleted from registry")
	}
}

func TestMCPProxyAwareHandler_RegistryLookupError(t *testing.T) {
	t.Parallel()
	selfAddr := "127.0.0.1:8094"
	sessionID := "err-session"
	registry := &stubRegistry{store: make(map[string]string), err: fmt.Errorf("redis down")}

	called := false
	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	req.Header.Set(trace.HeaderMcpSessionID, sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected local SDK handler to be called as fallback on registry error")
	}
}

func TestMCPProxyAwareHandler_RemoteProxySuccess(t *testing.T) {
	t.Parallel()
	selfAddr := "127.0.0.1:8094"
	sessionID := "remote-ok-session"

	// Start a test server to act as the remote instance
	remoteHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerInternalProxy) != "true" {
			t.Error("expected X-Internal-Proxy header on proxied request")
		}
		w.Header().Set("X-Test-Response", "from-remote")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("remote response"))
	})
	remoteServer := httptest.NewServer(remoteHandler)
	defer remoteServer.Close()

	remoteAddr := remoteServer.Listener.Addr().String()
	registry := &stubRegistry{store: map[string]string{sessionID: remoteAddr}}

	sdkHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local SDK handler should NOT be called for remote session")
	})

	handler := NewMCPProxyAwareHandler(sdkHandler, registry, selfAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/mcp", strings.NewReader(`{}`))
	req.Header.Set(trace.HeaderMcpSessionID, sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from remote proxy, got %d", rec.Code)
	}
	if rec.Body.String() != "remote response" {
		t.Fatalf("expected body %q, got %q", "remote response", rec.Body.String())
	}
}

func TestSessionCapturingWriter_Flush(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	cw := &sessionCapturingWriter{
		ResponseWriter:   rec,
		onSessionCreated: func(sid string) {},
	}
	cw.Flush()
}
