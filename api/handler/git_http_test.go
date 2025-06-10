package handler

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type GitHTTPTester struct {
	*testutil.GinTester
	handler *GitHTTPHandler
	mocks   struct {
		gitHttp *mockcomponent.MockGitHTTPComponent
	}
}

func NewGitHTTPTester(t *testing.T) *GitHTTPTester {
	tester := &GitHTTPTester{GinTester: testutil.NewGinTester()}
	tester.mocks.gitHttp = mockcomponent.NewMockGitHTTPComponent(t)

	tester.handler = &GitHTTPHandler{
		gitHttp: tester.mocks.gitHttp,
	}
	tester.WithParam("repo", "testRepo")
	tester.WithParam("branch", "testBranch")
	return tester
}

func (t *GitHTTPTester) WithHandleFunc(fn func(h *GitHTTPHandler) gin.HandlerFunc) *GitHTTPTester {
	t.Handler(fn(t.handler))
	return t
}

func TestGitHTTPHandler_InfoRefs(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.InfoRefs
	})

	reader := io.NopCloser(bytes.NewBuffer([]byte("foo")))
	tester.mocks.gitHttp.EXPECT().InfoRefs(tester.Ctx(), types.InfoRefsReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		Rpc:         "git-upload-pack",
		GitProtocol: "ssh",
		CurrentUser: "u",
	}).Return(reader, nil)
	tester.WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
	tester.WithKV("namespace", "u").WithKV("name", "r")
	tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

	require.Equal(t, 200, tester.Response().Code)
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte("foo"))
	require.NoError(t, err)
	err = gz.Close()
	require.NoError(t, err)
	require.Equal(t, b.String(), tester.Response().Body.String())
	headers := tester.Response().Header()
	require.Equal(t, "application/x-git-upload-pack-advertisement", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
}

func TestGitHTTPHandler_GitUploadPack(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
			return h.GitUploadPack
		})

		tester.mocks.gitHttp.EXPECT().GitUploadPack(tester.Ctx(), types.GitUploadPackReq{
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			GitProtocol: "ssh",
			Request:     tester.Gctx().Request,
			Writer:      tester.Gctx().Writer,
			CurrentUser: "u",
		}).Return(nil)
		tester.SetPath("git").WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
		tester.WithKV("namespace", "u").WithKV("name", "r")
		tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

		require.Equal(t, 200, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/x-git-result", headers.Get("Content-Type"))
		require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	})

	t.Run("no permission", func(t *testing.T) {
		tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
			return h.GitUploadPack
		})
		tester.mocks.gitHttp.EXPECT().GitUploadPack(tester.Ctx(), types.GitUploadPackReq{
			Namespace:   "u-other",
			Name:        "r",
			RepoType:    types.ModelRepo,
			GitProtocol: "ssh",
			Request:     tester.Gctx().Request,
			Writer:      tester.Gctx().Writer,
			CurrentUser: "u",
		}).Return(errorx.ErrForbidden)
		tester.SetPath("git").WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
		tester.WithKV("namespace", "u-other").WithKV("name", "r")
		tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

		require.Equal(t, 403, tester.Response().Code)
	})
}

func TestGitHTTPHandler_GitReceivePack(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.GitReceivePack
	})

	tester.mocks.gitHttp.EXPECT().GitReceivePack(tester.Ctx(), types.GitUploadPackReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		GitProtocol: "ssh",
		Request:     tester.Gctx().Request,
		Writer:      tester.Gctx().Writer,
		CurrentUser: "u",
	}).Return(nil)
	tester.SetPath("git").WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
	tester.WithKV("namespace", "u").WithKV("name", "r")
	tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "application/x-git-result", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
}

func TestGitHTTPHandler_LfsBatch(t *testing.T) {

	for _, op := range []types.LFSBatchOperation{types.LFSBatchUpload, types.LFSBatchDownload} {
		t.Run("success/"+string(op), func(t *testing.T) {
			tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
				return h.LfsBatch
			})

			tester.mocks.gitHttp.EXPECT().LFSBatch(tester.Ctx(), types.BatchRequest{
				Namespace:     "u",
				Name:          "r",
				RepoType:      types.ModelRepo,
				CurrentUser:   "u",
				Authorization: "auth",
				Operation:     op,
			}).Return(&types.BatchResponse{Transfer: "t"}, nil)
			tester.SetPath("git").WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
			tester.WithKV("namespace", "u").WithKV("name", "r").WithHeader("Authorization", "auth")
			tester.WithBody(t, &types.BatchRequest{Operation: op})
			tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

			tester.ResponseEqSimple(t, 200, &types.BatchResponse{Transfer: "t"})
			headers := tester.Response().Header()
			require.Equal(t, types.LfsMediaType, headers.Get("Content-Type"))
		})
	}

	for _, c := range []struct {
		err        error
		statusCode int
	}{
		{errorx.ErrUnauthorized, 401},
		{errorx.ErrForbidden, 403},
		{&errorx.HTTPError{
			StatusCode: 499,
			Message:    "xxx",
		}, 499},
		{errors.New("500"), 500},
	} {
		t.Run("error/"+c.err.Error(), func(t *testing.T) {
			tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
				return h.LfsBatch
			})

			tester.mocks.gitHttp.EXPECT().LFSBatch(tester.Ctx(), types.BatchRequest{
				Namespace:     "u",
				Name:          "r",
				RepoType:      types.ModelRepo,
				CurrentUser:   "u",
				Authorization: "auth",
				Operation:     types.LFSBatchDownload,
			}).Return(nil, c.err)
			tester.SetPath("git").WithQuery("service", "git-upload-pack").WithHeader("Git-Protocol", "ssh")
			tester.WithKV("namespace", "u").WithKV("name", "r").WithHeader("Authorization", "auth")
			tester.WithBody(t, &types.BatchRequest{Operation: types.LFSBatchDownload})
			tester.WithKV("repo_type", "model").WithUser().WithHeader("Accept-Encoding", "gzip").Execute()

			require.Equal(t, c.statusCode, tester.Response().Code)
		})
	}
}

func TestGitHTTPHandler_LfsUpload(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.LfsUpload
	})

	tester.mocks.gitHttp.EXPECT().LfsUpload(tester.Ctx(), tester.Gctx().Request.Body, types.UploadRequest{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Oid:         "o",
		Size:        100,
	}).Return(nil)
	tester.SetPath("git").WithParam("oid", "o").WithParam("size", "100")
	tester.WithKV("namespace", "u").WithKV("name", "r").WithHeader("Authorization", "auth")
	tester.WithKV("repo_type", "model").WithUser().Execute()

	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, types.LfsMediaType, headers.Get("Content-Type"))
}

func TestGitHTTPHandler_LfsDownload(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.LfsDownload
	})

	tester.mocks.gitHttp.EXPECT().LfsDownload(tester.Ctx(), types.DownloadRequest{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Oid:         "o",
		Size:        100,
	}).Return(&url.URL{Path: "url"}, nil)
	tester.SetPath("git").WithParam("oid", "o").WithParam("size", "100")
	tester.WithKV("namespace", "u").WithKV("name", "r").WithHeader("Authorization", "auth")
	tester.WithKV("repo_type", "model").WithUser().Execute()

	require.Equal(t, 200, tester.Response().Code)
	resp := tester.Response().Result()
	defer resp.Body.Close()
	lc, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, "url", lc.String())
}

func TestGitHTTPHandler_LfsVerify(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.LfsVerify
	})

	tester.mocks.gitHttp.EXPECT().LfsVerify(tester.Ctx(), types.VerifyRequest{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}, types.Pointer{Oid: "o"}).Return(nil)
	tester.WithKV("namespace", "u").WithKV("name", "r").WithHeader("Authorization", "auth")
	tester.WithKV("repo_type", "model").WithUser().WithBody(t, &types.Pointer{
		Oid: "o",
	}).Execute()

	tester.ResponseEqSimple(t, 200, nil)
}

func TestGitHTTPHandler_ListLocks(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.ListLocks
	})

	tester.mocks.gitHttp.EXPECT().ListLocks(tester.Ctx(), types.ListLFSLockReq{
		ID:          1,
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Cursor:      12,
		Path:        "p",
		Limit:       5,
	}).Return(&types.LFSLockList{Next: "n"}, nil)
	tester.WithKV("namespace", "u").WithKV("name", "r").WithQuery("path", "p").WithQuery("id", "1")
	tester.WithKV("repo_type", "model").WithUser().WithQuery("cursor", "12").WithQuery("limit", "5").Execute()

	tester.ResponseEqSimple(t, 200, &types.LFSLockList{Next: "n"})
}

func TestGitHTTPHandler_CreateLock(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.CreateLock
	})

	tester.mocks.gitHttp.EXPECT().CreateLock(tester.Ctx(), types.LfsLockReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}).Return(&database.LfsLock{
		ID:   1,
		Path: "p",
		User: database.User{Username: "u"},
	}, nil)
	tester.WithKV("namespace", "u").WithKV("name", "r")
	tester.WithKV("repo_type", "model").WithUser().WithBody(t, types.LfsLockReq{}).Execute()

	tester.ResponseEqSimple(t, 200, &types.LFSLockResponse{
		Lock: &types.LFSLock{
			ID:   "1",
			Path: "p",
			Owner: &types.LFSLockOwner{
				Name: "u",
			},
		},
	})
}

func TestGitHTTPHandler_VerifyLock(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.VerifyLock
	})

	tester.mocks.gitHttp.EXPECT().VerifyLock(tester.Ctx(), types.VerifyLFSLockReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}).Return(&types.LFSLockListVerify{
		Next: "n",
	}, nil)
	tester.WithKV("namespace", "u").WithKV("name", "r")
	tester.WithKV("repo_type", "model").WithUser().WithBody(t, types.VerifyLFSLockReq{}).Execute()

	tester.ResponseEqSimple(t, 200, &types.LFSLockListVerify{
		Next: "n",
	})
}

func TestGitHTTPHandler_UnLock(t *testing.T) {
	tester := NewGitHTTPTester(t).WithHandleFunc(func(h *GitHTTPHandler) gin.HandlerFunc {
		return h.UnLock
	})

	tester.mocks.gitHttp.EXPECT().UnLock(tester.Ctx(), types.UnlockLFSReq{
		ID:          1,
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}).Return(&database.LfsLock{
		ID:   1,
		Path: "p",
		User: database.User{Username: "u"},
	}, nil)
	tester.WithKV("namespace", "u").WithKV("name", "r").WithParam("lid", "1")
	tester.WithKV("repo_type", "model").WithUser().WithBody(t, types.UnlockLFSReq{}).Execute()

	tester.ResponseEqSimple(t, 200, &types.LFSLockResponse{
		Lock: &types.LFSLock{
			ID:   "1",
			Path: "p",
			Owner: &types.LFSLockOwner{
				Name: "u",
			},
		},
	})
}
