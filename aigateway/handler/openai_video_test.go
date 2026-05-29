package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamVideoDownloadURL_Success(t *testing.T) {
	videoContent := "fake-video-bytes"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(videoContent))
	}))
	defer upstream.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL:    &url.URL{},
		Header: http.Header{},
	}

	streamVideoDownloadURL(c, upstream.URL+"/path/to/video.mp4")

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, videoContent, w.Body.String())
	require.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
}

func TestStreamVideoDownloadURL_InvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL:    &url.URL{},
		Header: http.Header{},
	}

	streamVideoDownloadURL(c, "://invalid-url")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "internal_error")
}

func TestStreamVideoDownloadURL_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer upstream.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL:    &url.URL{},
		Header: http.Header{},
	}

	streamVideoDownloadURL(c, upstream.URL+"/path/to/missing-video.mp4")

	require.Equal(t, http.StatusNotFound, w.Code)
}
