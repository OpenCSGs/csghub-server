package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

func TestReverseProxy_AcceptEncodingDefaultGzip(t *testing.T) {
	var downstreamAcceptEncoding string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "gzip", downstreamAcceptEncoding)
}

func TestReverseProxy_AcceptEncodingDisabled(t *testing.T) {
	var downstreamAcceptEncoding string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL, WithoutAcceptEncoding())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br")
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "identity", downstreamAcceptEncoding)
}

func TestReverseProxy_RemovesConfiguredRequestHeaders(t *testing.T) {
	var authorization, cookie, preserved string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
		preserved = r.Header.Get("X-Preserved")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL, WithoutRequestHeaders("Authorization", "Cookie"))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer user-token")
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("X-Preserved", "value")
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, http.StatusOK, resp.Code)
	require.Empty(t, authorization)
	require.Empty(t, cookie)
	require.Equal(t, "value", preserved)
}

func TestReverseProxy_AppliesResponseModifiersInOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Value", "initial")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL,
		WithResponseModifier(func(resp *http.Response) error {
			resp.Header.Set("X-Value", resp.Header.Get("X-Value")+"-first")
			return nil
		}),
		WithResponseModifier(func(resp *http.Response) error {
			resp.Header.Set("X-Value", resp.Header.Get("X-Value")+"-second")
			return nil
		}),
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, http.StatusAccepted, resp.Code)
	require.Equal(t, "initial-first-second", resp.Header().Get("X-Value"))
}

func TestReverseProxy_ResponseModifierErrorWritesBadGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL, WithResponseModifier(func(*http.Response) error {
		return errors.New("invalid upstream response")
	}))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, http.StatusBadGateway, resp.Code)
}

func TestReverseProxy_ContextCanceledWritesClientClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	rp, err := NewReverseProxy(server.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	resp := httptest.NewRecorder()
	rp.ServeHTTP(resp, req, "", "")

	require.Equal(t, commonutils.StatusClientClosedRequest, resp.Code)
}

// import (
// 	"bytes"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"

// 	"github.com/stretchr/testify/mock"
// 	"github.com/stretchr/testify/require"
// 	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
// 	"opencsg.com/csghub-server/builder/rpc"
// )

// func TestReverseProxy(t *testing.T) {
// 	// init a test http server for the backend service of reverse proxy
// 	hander := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write([]byte("server response"))
// 	})
// 	server := httptest.NewServer(hander)
// 	rp, err := NewReverseProxy(server.URL)
// 	if err != nil {
// 		t.Fatalf("failed to create reverse proxy: %v", err)
// 	}

// 	// http test request
// 	reqBody := bytes.NewBufferString("hello world")
// 	req := httptest.NewRequest(http.MethodGet, "/", reqBody)
// 	respWriter := httptest.NewRecorder()
// 	rp.ServeHTTP(respWriter, req, "")

// 	require.True(t, respWriter.Code == http.StatusOK)
// 	require.Equal(t, respWriter.Body.String(), "server response")

// }

// func TestReverseProxy_RequestModNotPass(t *testing.T) {
// 	// init a test http server for the backend service of reverse proxy
// 	hander := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// make sure we keep the requesty body after moderation
// 		requestBodyContent, err := io.ReadAll(r.Body)
// 		require.NoError(t, err)
// 		require.Equal(t, "sensitive content", string(requestBodyContent))

// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write([]byte("server response"))
// 	})
// 	server := httptest.NewServer(hander)
// 	rp, err := NewReverseProxy(server.URL)
// 	if err != nil {
// 		t.Fatalf("failed to create reverse proxy: %v", err)
// 	}
// 	mockModSvcClient := mockrpc.NewMockModerationSvcClient(t)
// 	mockModSvcClient.EXPECT().PassTextCheck(mock.Anything, "comment_detection", "sensitive content").Return(&rpc.CheckResult{
// 		IsSensitive: true,
// 		Reason:      "sensitive content detected",
// 	}, nil)
// 	//enable moderation
// 	rp.WithModeration(mockModSvcClient)

// 	reqBody := bytes.NewBufferString("sensitive content")
// 	// can't use httptest.NewRequest which dont support GetBody method
// 	req, _ := http.NewRequest(http.MethodGet, "/", reqBody)
// 	respWriter := httptest.NewRecorder()
// 	rp.ServeHTTP(respWriter, req, "")

// 	require.True(t, respWriter.Code == http.StatusBadRequest)
// 	require.Equal(t, respWriter.Body.String(), "sensitive content detected in request body:sensitive content detected")
// }

// func TestReverseProxy_ResponseModNotPass(t *testing.T) {
// 	// init a test http server for the backend service of reverse proxy
// 	hander := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// make sure we keep the requesty body after moderation
// 		requestBodyContent, err := io.ReadAll(r.Body)
// 		require.NoError(t, err)
// 		require.Equal(t, "normal request body", string(requestBodyContent))

// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write([]byte("sensitive content"))
// 	})
// 	server := httptest.NewServer(hander)
// 	rp, err := NewReverseProxy(server.URL)
// 	if err != nil {
// 		t.Fatalf("failed to create reverse proxy: %v", err)
// 	}
// 	mockModSvcClient := mockrpc.NewMockModerationSvcClient(t)
// 	mockModSvcClient.EXPECT().PassTextCheck(mock.Anything, "comment_detection", "sensitive content").Return(&rpc.CheckResult{
// 		IsSensitive: true,
// 		Reason:      "sensitive content detected",
// 	}, nil)
// 	mockModSvcClient.EXPECT().PassTextCheck(mock.Anything, "comment_detection", "normal request body").Return(&rpc.CheckResult{
// 		IsSensitive: false,
// 	}, nil)
// 	//enable moderation
// 	rp.WithModeration(mockModSvcClient)

// 	reqBody := bytes.NewBufferString("normal request body")
// 	// can't use httptest.NewRequest which dont support GetBody method
// 	req, _ := http.NewRequest(http.MethodGet, "/", reqBody)
// 	respWriter := httptest.NewRecorder()
// 	rp.ServeHTTP(respWriter, req, "")

// 	require.True(t, respWriter.Code == http.StatusBadRequest)
// 	require.Equal(t, respWriter.Body.String(), "sensitive content detected in response body:sensitive content detected")
// }

// func TestReverseProxy_isTextContent(t *testing.T) {
// 	// init a test http server for the backend service of reverse proxy
// 	hander := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// make sure we keep the requesty body after moderation
// 		requestBodyContent, err := io.ReadAll(r.Body)
// 		require.NoError(t, err)
// 		require.Equal(t, "normal request body", string(requestBodyContent))

// 		w.WriteHeader(http.StatusOK)
// 		_, _ = w.Write([]byte("sensitive content"))
// 	})
// 	server := httptest.NewServer(hander)
// 	rp, err := NewReverseProxy(server.URL)
// 	if err != nil {
// 		t.Fatalf("failed to create reverse proxy: %v", err)
// 	}
// 	header := make(http.Header)
// 	require.False(t, rp.isTextContent(header))

// 	header.Set("Content-Type", "application/json")
// 	require.True(t, rp.isTextContent(header))

// 	header.Set("Content-Type", "text/event-stream;charset=utf-8")
// 	require.False(t, rp.isTextContent(header))
// }
