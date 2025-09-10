package proxy

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
