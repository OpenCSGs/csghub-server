//go:build ee || saas

package handler

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

const testCorrelationID = "f83b035c-bb2f-4e7d-b2f1-e19da676135f"

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("stream failed") }
func (failingReadCloser) Close() error             { return nil }

func TestRepoHandlerDownloadAnalytics(t *testing.T) {
	t.Run("LFS URL issued uses authenticated user UUID", func(t *testing.T) {
		tester, publisher := newDownloadAnalyticsTester(t, true)
		tester.WithUser()
		tester.WithKV(httpbase.CurrentUserUUIDCtxVar, "server-user-uuid")
		tester.WithHeader(postHogDistinctIDHeader, "forged-client-id")
		tester.WithHeader(postHogSessionIDHeader, "session-1")

		tester.mocks.repo.EXPECT().DownloadFile(
			tester.Ctx(),
			downloadFileRequest(true, "u"),
			"u",
		).Return(nil, int64(1024), "https://download.test/file", nil)

		tester.Execute()

		require.Equal(t, 200, tester.Response().Code)
		require.Len(t, publisher.events, 1)
		event := publisher.events[0]
		require.Equal(t, modelFileDownloadURLIssued, event.Name)
		require.Equal(t, "server-user-uuid", event.DistinctID)
		require.Equal(t, "session-1", event.SessionID)
		require.Equal(t, testCorrelationID, event.CorrelationID)
		require.Equal(t, downloadChannelBrowser, event.Properties["download_channel"])
		require.Equal(t, deliveryTypeLFSURL, event.Properties["delivery_type"])
		require.Equal(t, int64(1024), event.Properties["file_size"])
	})

	t.Run("normal stream success uses anonymous distinct ID", func(t *testing.T) {
		tester, publisher := newDownloadAnalyticsTester(t, false)
		tester.WithHeader(postHogDistinctIDHeader, "anonymous-id")
		tester.mocks.repo.EXPECT().DownloadFile(
			tester.Ctx(),
			downloadFileRequest(false, ""),
			"",
		).Return(io.NopCloser(bytes.NewBufferString("content")), int64(7), "", nil)

		tester.Execute()

		require.Equal(t, 200, tester.Response().Code)
		require.Len(t, publisher.events, 1)
		require.Equal(t, modelFileDownloadSuccess, publisher.events[0].Name)
		require.Equal(t, "anonymous-id", publisher.events[0].DistinctID)
		require.Equal(t, downloadChannelBrowser, publisher.events[0].Properties["download_channel"])
		require.Equal(t, deliveryTypeDirectStream, publisher.events[0].Properties["delivery_type"])
	})

	t.Run("stream failure does not report success", func(t *testing.T) {
		tester, publisher := newDownloadAnalyticsTester(t, false)
		tester.WithHeader(postHogDistinctIDHeader, "anonymous-id")
		tester.mocks.repo.EXPECT().DownloadFile(
			tester.Ctx(),
			downloadFileRequest(false, ""),
			"",
		).Return(failingReadCloser{}, int64(7), "", nil)

		tester.Execute()

		require.Empty(t, publisher.events)
	})

	t.Run("invalid correlation ID is ignored", func(t *testing.T) {
		tester, publisher := newDownloadAnalyticsTester(t, true)
		tester.Gctx().Request.Header.Set(correlationIDHeader, "not-a-uuid")
		tester.WithHeader(postHogDistinctIDHeader, "anonymous-id")
		tester.mocks.repo.EXPECT().DownloadFile(
			tester.Ctx(),
			downloadFileRequest(true, ""),
			"",
		).Return(nil, int64(1024), "https://download.test/file", nil)

		tester.Execute()

		require.Empty(t, publisher.events)
	})

	t.Run("publisher failure does not alter response", func(t *testing.T) {
		tester, publisher := newDownloadAnalyticsTester(t, true)
		publisher.err = errors.New("PostHog unavailable")
		tester.WithHeader(postHogDistinctIDHeader, "anonymous-id")
		tester.mocks.repo.EXPECT().DownloadFile(
			tester.Ctx(),
			downloadFileRequest(true, ""),
			"",
		).Return(nil, int64(1024), "https://download.test/file", nil)

		tester.Execute()

		require.Equal(t, 200, tester.Response().Code)
		require.Len(t, publisher.events, 1)
	})
}

func TestRepoHandlerSDKDownloadAnalytics(t *testing.T) {
	t.Run("normal file reports an authenticated SDK-compatible download", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(handler *RepoHandler) gin.HandlerFunc {
			return handler.SDKDownload
		})
		publisher := &recordingPublisher{}
		tester.handler.analytics = publisher
		tester.WithUser()
		tester.WithKV(httpbase.CurrentUserUUIDCtxVar, "server-user-uuid")
		tester.WithParam("branch", "main").WithParam("file_path", "weights.bin")
		tester.WithKV("repo_type", types.ModelRepo)
		tester.Gctx().Request.Method = "GET"
		req := sdkDownloadRequest(false)
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(false, int64(7), nil)
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), req, "u").Return(
			io.NopCloser(bytes.NewBufferString("content")), int64(7), "", nil,
		)

		tester.Execute()

		require.Equal(t, 200, tester.Response().Code)
		require.Len(t, publisher.events, 1)
		event := publisher.events[0]
		require.Equal(t, modelFileDownloadSuccess, event.Name)
		require.Equal(t, "server-user-uuid", event.DistinctID)
		require.NotEmpty(t, event.InsertID)
		require.Equal(t, downloadChannelSDKCompatible, event.Properties["download_channel"])
		require.Equal(t, deliveryTypeDirectStream, event.Properties["delivery_type"])
		require.Equal(t, "authenticated", event.Properties["identity_type"])
	})

	t.Run("LFS URL reports an anonymous request without a person profile", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(handler *RepoHandler) gin.HandlerFunc {
			return handler.SDKDownload
		})
		publisher := &recordingPublisher{}
		tester.handler.analytics = publisher
		tester.WithParam("branch", "main").WithParam("file_path", "weights.bin")
		tester.WithKV("repo_type", types.ModelRepo)
		tester.Gctx().Request.Method = "GET"
		req := sdkDownloadRequest(false)
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(true, int64(1024), nil)
		lfsReq := *req
		lfsReq.Lfs = true
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), &lfsReq, "").Return(
			nil, int64(1024), "https://download.test/file", nil,
		)

		tester.Execute()

		require.Equal(t, 302, tester.Response().Code)
		require.Len(t, publisher.events, 1)
		event := publisher.events[0]
		require.Equal(t, modelFileDownloadURLIssued, event.Name)
		require.Contains(t, event.DistinctID, "anonymous-request:")
		require.Equal(t, false, event.Properties["$process_person_profile"])
		require.Equal(t, "anonymous_request", event.Properties["identity_type"])
		require.Equal(t, deliveryTypeLFSURL, event.Properties["delivery_type"])
	})

	t.Run("subsequent ranges do not report another download", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(handler *RepoHandler) gin.HandlerFunc {
			return handler.SDKDownload
		})
		publisher := &recordingPublisher{}
		tester.handler.analytics = publisher
		tester.WithParam("branch", "main").WithParam("file_path", "weights.bin")
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithHeader("Range", "bytes=10-19")
		req := sdkDownloadRequest(false)
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(false, int64(100), nil)
		downloadReq := *req
		downloadReq.Limit = 20
		downloadReq.CountDownload = false
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), &downloadReq, "").Return(
			io.NopCloser(bytes.NewBuffer(make([]byte, 100))), int64(100), "", nil,
		)

		tester.Execute()

		require.Equal(t, 206, tester.Response().Code)
		require.Empty(t, publisher.events)
	})

	t.Run("failed downloads do not report a success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(handler *RepoHandler) gin.HandlerFunc {
			return handler.SDKDownload
		})
		publisher := &recordingPublisher{}
		tester.handler.analytics = publisher
		tester.WithParam("branch", "main").WithParam("file_path", "weights.bin")
		tester.WithKV("repo_type", types.ModelRepo)
		req := sdkDownloadRequest(false)
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(false, int64(7), nil)
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), req, "").Return(
			nil, int64(0), "", errors.New("download failed"),
		)

		tester.Execute()

		require.Equal(t, 500, tester.Response().Code)
		require.Empty(t, publisher.events)
	})
}

func sdkDownloadRequest(lfs bool) *types.GetFileReq {
	return &types.GetFileReq{
		Namespace:     "u",
		Name:          "r",
		Path:          "weights.bin",
		Ref:           "main",
		Lfs:           lfs,
		SaveAs:        "weights.bin",
		RepoType:      types.ModelRepo,
		CountDownload: true,
	}
}

func newDownloadAnalyticsTester(t *testing.T, lfs bool) (*RepoTester, *recordingPublisher) {
	tester := NewRepoTester(t).WithHandleFunc(func(handler *RepoHandler) gin.HandlerFunc {
		return handler.DownloadFile
	})
	publisher := &recordingPublisher{}
	tester.handler.analytics = publisher
	tester.WithParam("file_path", "weights.bin")
	tester.WithQuery("ref", "main")
	if lfs {
		tester.WithQuery("lfs", "true")
	}
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithHeader(correlationIDHeader, testCorrelationID)
	return tester, publisher
}

func downloadFileRequest(lfs bool, currentUser string) *types.GetFileReq {
	return &types.GetFileReq{
		Namespace:   "u",
		Name:        "r",
		Path:        "weights.bin",
		Ref:         "main",
		Lfs:         lfs,
		RepoType:    types.ModelRepo,
		CurrentUser: currentUser,
	}
}
