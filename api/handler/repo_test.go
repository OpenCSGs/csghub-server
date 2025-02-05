package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type RepoTester struct {
	*testutil.GinTester
	handler *RepoHandler
	mocks   struct {
		repo *mockcomponent.MockRepoComponent
	}
}

func NewRepoTester(t *testing.T) *RepoTester {
	tester := &RepoTester{GinTester: testutil.NewGinTester()}
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)
	tester.handler = &RepoHandler{tester.mocks.repo, 0}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (rt *RepoTester) WithHandleFunc(fn func(rp *RepoHandler) gin.HandlerFunc) *RepoTester {
	rt.Handler(fn(rt.handler))
	return rt
}

func TestRepoHandler_CreateFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CreateFile
	})
	tester.RequireUser(t)

	tester.mocks.repo.EXPECT().CreateFile(tester.Ctx(), &types.CreateFileReq{
		Message:     "foo",
		Branch:      "main",
		Content:     "bar",
		Username:    "u",
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		FilePath:    "foo",
	}).Return(&types.CreateFileResp{}, nil)
	tester.WithParam("file_path", "foo")
	req := &types.CreateFileReq{
		Message: "foo",
		Branch:  "main",
		Content: "bar",
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.CreateFileResp{})

}

func TestRepoHandler_UpdateFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateFile
	})
	tester.RequireUser(t)

	tester.mocks.repo.EXPECT().UpdateFile(tester.Ctx(), &types.UpdateFileReq{
		Message:     "foo",
		Branch:      "main",
		Content:     "bar",
		Username:    "u",
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		FilePath:    "foo",
	}).Return(&types.UpdateFileResp{}, nil)
	tester.WithParam("file_path", "foo")
	req := &types.CreateFileReq{
		Message: "foo",
		Branch:  "main",
		Content: "bar",
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.UpdateFileResp{})

}

func TestRepoHandler_Commits(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.Commits
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().Commits(tester.Ctx(), &types.GetCommitsReq{
		Namespace:   "u",
		Name:        "r",
		Ref:         "main",
		Page:        1,
		Per:         10,
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}).Return([]types.Commit{{ID: "c1"}}, &types.RepoPageOpts{Total: 100, PageCount: 1}, nil)
	tester.WithParam("file_path", "foo")
	tester.WithQuery("ref", "main")
	tester.AddPagination(1, 10)
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, gin.H{
		"commits":    []types.Commit{{ID: "c1"}},
		"total":      100,
		"page_count": 1,
	})

}

func TestRepoHandler_LastCommit(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.LastCommit
		})

		//user does not have permission to access repo
		tester.mocks.repo.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(nil, component.ErrForbidden).Once()
		tester.Execute()
		require.Equal(t, http.StatusForbidden, tester.Response().Code)
	})

	t.Run("server error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.LastCommit
		})
		commit := &types.Commit{}

		tester.mocks.repo.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(commit, errors.New("custome error")).Once()
		tester.Execute()
		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})

	t.Run("success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.LastCommit
		})

		commit := &types.Commit{}
		commit.AuthorName = "u"
		commit.ID = uuid.New().String()

		tester.mocks.repo.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(commit, nil).Once()
		tester.Execute()
		require.Equal(t, http.StatusOK, tester.Response().Code)

		var r = struct {
			Code int           `json:"code,omitempty"`
			Msg  string        `json:"msg"`
			Data *types.Commit `json:"data,omitempty"`
		}{}
		err := json.Unmarshal(tester.Response().Body.Bytes(), &r)
		require.Empty(t, err)
		require.Equal(t, commit.ID, r.Data.ID)
	})
}

func TestRepoHandler_FileRaw(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.FileRaw
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().FileRaw(tester.Ctx(), &types.GetFileReq{
		Namespace:   "u",
		Name:        "r",
		Ref:         "main",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Path:        "foo",
	}).Return("data", nil)
	tester.WithParam("file_path", "foo")
	tester.WithQuery("ref", "main")
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, "data")

}

func TestRepoHandler_FileInfo(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.FileInfo
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().FileInfo(tester.Ctx(), &types.GetFileReq{
		Namespace:   "u",
		Name:        "r",
		Ref:         "main",
		RepoType:    types.ModelRepo,
		Path:        "foo",
		CurrentUser: "u",
	}).Return(&types.File{Name: "foo.go"}, nil)
	tester.WithParam("file_path", "foo")
	tester.WithQuery("ref", "main")
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.File{Name: "foo.go"})

}

func TestRepoHandler_DownloadFile(t *testing.T) {

	t.Run("lfs file", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadFile
		})

		tester.WithUser()
		tester.mocks.repo.EXPECT().DownloadFile(tester.Ctx(), &types.GetFileReq{
			Namespace:   "u",
			Name:        "r",
			Ref:         "main",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
			Path:        "foo",
			Lfs:         true,
		}, "u").Return(nil, 100, "foo", nil)
		tester.WithParam("file_path", "foo")
		tester.WithQuery("ref", "main")
		tester.WithQuery("lfs", "true")
		tester.WithKV("repo_type", types.ModelRepo)

		tester.Execute()
		tester.ResponseEq(t, http.StatusOK, tester.OKText, "foo")
	})

	t.Run("normal file", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadFile
		})

		tester.WithUser()
		tester.mocks.repo.EXPECT().DownloadFile(tester.Ctx(), &types.GetFileReq{
			Namespace:   "u",
			Name:        "r",
			Ref:         "main",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
			Path:        "foo",
		}, "u").Return(
			io.NopCloser(bytes.NewBuffer([]byte("bar"))), 100, "foo", nil,
		)
		tester.WithParam("file_path", "foo")
		tester.WithQuery("ref", "main")
		tester.WithKV("repo_type", types.ModelRepo)

		tester.Execute()
		require.Equal(t, 200, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/octet-stream", headers.Get("Content-Type"))
		require.Equal(t, `attachment; filename="foo"`, headers.Get("Content-Disposition"))
		require.Equal(t, "100", headers.Get("Content-Length"))
		r := tester.Response().Body.String()
		require.Equal(t, "bar", r)
	})

}

func TestRepoHandler_Branches(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.Branches
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().Branches(tester.Ctx(), &types.GetBranchesReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Per:         10,
		Page:        1,
	}).Return([]types.Branch{{Name: "main"}}, nil)
	tester.WithKV("repo_type", types.ModelRepo)
	tester.AddPagination(1, 10)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, []types.Branch{{Name: "main"}})

}

func TestRepoHandler_Tags(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.Tags
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().Tags(tester.Ctx(), &types.GetTagsReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
	}).Return([]database.Tag{{Name: "main"}}, nil)
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, []database.Tag{{Name: "main"}})

}

func TestRepoHandler_UpdateTags(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateTags
	})
	tester.RequireUser(t)

	tester.mocks.repo.EXPECT().UpdateTags(
		tester.Ctx(), "u", "r", types.ModelRepo,
		"cat", "u", []string{"foo", "bar"},
	).Return(nil)
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("category", "cat")
	tester.WithBody(t, []string{"foo", "bar"})

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)

}

func TestRepoHandler_Tree(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.Tree
		})
		//user does not have permission to access repo
		tester.mocks.repo.EXPECT().Tree(mock.Anything, mock.Anything).Return(nil, component.ErrForbidden).Once()
		tester.Execute()
		require.Equal(t, http.StatusForbidden, tester.Response().Code)
	})

	t.Run("server error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.Tree
		})

		tester.mocks.repo.EXPECT().Tree(mock.Anything, mock.Anything).Return(nil, errors.New("custome error")).Once()
		tester.Execute()
		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})

	t.Run("success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.Tree
		})
		var tree []*types.File
		tester.mocks.repo.EXPECT().Tree(mock.Anything, mock.Anything).Return(tree, nil).Once()
		tester.Execute()
		require.Equal(t, http.StatusOK, tester.Response().Code)

		var r = struct {
			Code int           `json:"code,omitempty"`
			Msg  string        `json:"msg"`
			Data []*types.File `json:"data,omitempty"`
		}{}
		err := json.Unmarshal(tester.Response().Body.Bytes(), &r)
		require.Empty(t, err)
		require.Equal(t, tree, r.Data)
	})
}

func TestRepoHandler_UpdateDownloads(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateDownloads
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().UpdateDownloads(
		tester.Ctx(), &types.UpdateDownloadsReq{
			Namespace: "u",
			Name:      "r",
			RepoType:  types.ModelRepo,
			ReqDate:   "2002-02-01",
			Date:      time.Date(2002, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	).Return(nil)
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.UpdateDownloadsReq{ReqDate: "2002-02-01"})

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)

}

func TestRepoHandler_IncrDownloads(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.IncrDownloads
	})

	tester.WithUser()
	tester.mocks.repo.EXPECT().IncrDownloads(
		tester.Ctx(), types.ModelRepo, "u", "r",
	).Return(nil)
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.UpdateDownloadsReq{ReqDate: "2002-02-01"})

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)

}

func TestRepoHandler_UploadFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UploadFile
	})
	tester.RequireUser(t)

	bodyBuffer := new(bytes.Buffer)
	mw := multipart.NewWriter(bodyBuffer)
	err := mw.WriteField("file_path", "foo")
	require.NoError(t, err)
	err = mw.WriteField("message", "msg")
	require.NoError(t, err)
	err = mw.WriteField("branch", "main")
	require.NoError(t, err)
	part, err := mw.CreateFormFile("file", "file")
	if err != nil {
		t.Fatal(err)
	}
	file := strings.NewReader(`data`)
	_, err = io.Copy(part, file)
	require.NoError(t, err)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", bodyBuffer)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	err = req.ParseMultipartForm(20)
	require.NoError(t, err)
	tester.WithMultipartForm(req.MultipartForm)

	tester.mocks.repo.EXPECT().UploadFile(
		tester.Ctx(), &types.CreateFileReq{
			Namespace:       "u",
			Name:            "r",
			RepoType:        types.ModelRepo,
			Content:         "ZGF0YQ==",
			OriginalContent: []byte("data"),
			CurrentUser:     "u",
			Message:         "msg",
			Branch:          "main",
			FilePath:        "foo",
			Username:        "u",
		},
	).Return(nil)
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)

}

func TestRepoHandler_SDKListFiles(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.SDKListFiles
	})

	tester.WithUser()
	tester.WithParam("ref", "main")
	tester.WithParam("branch_mapped", "main_main")
	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().SDKListFiles(
		tester.Ctx(), types.ModelRepo, "u", "r", "main_main", "u",
	).Return(&types.SDKFiles{ID: "f1"}, nil)

	tester.Execute()
	tester.ResponseEqSimple(t, http.StatusOK, &types.SDKFiles{ID: "f1"})
}

func TestRepoHandler_HandleDownload(t *testing.T) {

	t.Run("lfs file", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.SDKDownload
		})

		tester.WithUser()
		tester.WithParam("ref", "main")
		tester.WithParam("branch_mapped", "main_main")
		tester.WithParam("file_path", "foo")
		tester.WithKV("repo_type", types.ModelRepo)
		req := &types.GetFileReq{
			Namespace: "u",
			Name:      "r",
			Path:      "foo",
			Ref:       "main_main",
			Lfs:       false,
			SaveAs:    "foo",
			RepoType:  types.ModelRepo,
		}
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(true, 100, nil)
		reqnew := *req
		reqnew.Lfs = true
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), &reqnew, "u").Return(
			nil, 100, "url", nil,
		)

		tester.Execute()

		// redirected
		require.Equal(t, http.StatusOK, tester.Response().Code)
		resp := tester.Response().Result()
		defer resp.Body.Close()
		lc, err := resp.Location()
		require.NoError(t, err)
		require.Equal(t, "/url", lc.String())
	})

	t.Run("normal file", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.SDKDownload
		})

		tester.WithUser()
		tester.WithParam("ref", "main")
		tester.WithParam("branch_mapped", "main_main")
		tester.WithParam("file_path", "foo")
		tester.WithKV("repo_type", types.ModelRepo)
		req := &types.GetFileReq{
			Namespace: "u",
			Name:      "r",
			Path:      "foo",
			Ref:       "main_main",
			Lfs:       false,
			SaveAs:    "foo",
			RepoType:  types.ModelRepo,
		}
		tester.mocks.repo.EXPECT().IsLfs(tester.Ctx(), req).Return(false, 100, nil)
		tester.mocks.repo.EXPECT().SDKDownloadFile(tester.Ctx(), req, "u").Return(
			io.NopCloser(bytes.NewBuffer([]byte("bar"))), 100, "url", nil,
		)

		tester.Execute()
		require.Equal(t, 200, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/octet-stream", headers.Get("Content-Type"))
		require.Equal(t, `attachment; filename="foo"`, headers.Get("Content-Disposition"))
		require.Equal(t, "100", headers.Get("Content-Length"))
		r := tester.Response().Body.String()
		require.Equal(t, "bar", r)
	})
}

func TestRepoHandler_HeadSDKDownload(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.HeadSDKDownload
	})

	tester.WithUser()
	tester.WithParam("file_path", "foo")
	tester.WithParam("branch", "main")
	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().HeadDownloadFile(
		tester.Ctx(), &types.GetFileReq{
			Namespace: "u",
			Name:      "r",
			Path:      "foo",
			Ref:       "main",
			SaveAs:    "foo",
			RepoType:  types.ModelRepo,
		}, "u",
	).Return(&types.File{Size: 100, SHA: "def"}, nil)

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "100", headers.Get("Content-Length"))
	require.Equal(t, "def", headers.Get("X-Repo-Commit"))
}

func TestRepoHandler_CommitWithDiff(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CommitWithDiff
	})

	tester.WithUser()
	tester.WithParam("commit_id", "foo")
	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().GetCommitWithDiff(
		tester.Ctx(), &types.GetCommitsReq{
			Namespace:   "u",
			Name:        "r",
			Ref:         "foo",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
		},
	).Return(&types.CommitResponse{Commit: &types.Commit{ID: "foo"}}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.CommitResponse{Commit: &types.Commit{ID: "foo"}})
}

func TestRepoHandler_CreateMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CreateMirror
	})

	tester.RequireUser(t)
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.CreateMirrorReq{
		SourceUrl:      "https://foo.com",
		MirrorSourceID: 12,
	})
	tester.mocks.repo.EXPECT().CreateMirror(
		tester.Ctx(), types.CreateMirrorReq{
			Namespace:      "u",
			Name:           "r",
			RepoType:       types.ModelRepo,
			CurrentUser:    "u",
			SourceUrl:      "https://foo.com",
			MirrorSourceID: 12,
		},
	).Return(&database.Mirror{ID: 123}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &database.Mirror{ID: 123})
}

func TestRepoHandler_MirrorFromSaas(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaas
		})
		tester.RequireUser(t)

		tester.WithParam("namespace", types.OpenCSGPrefix+"repo")
		tester.WithKV("repo_type", types.ModelRepo)
		tester.mocks.repo.EXPECT().MirrorFromSaas(
			tester.Ctx(), "CSG_repo", "r", "u", types.ModelRepo,
		).Return(nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("invalid", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.MirrorFromSaas
		})
		tester.RequireUser(t)

		tester.WithKV("repo_type", types.ModelRepo)
		tester.Execute()
		tester.ResponseEq(t, 400, "Repo could not be mirrored", nil)
	})
}

func TestRepoHandler_GetMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.GetMirror
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().GetMirror(
		tester.Ctx(), types.GetMirrorReq{
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
		},
	).Return(&database.Mirror{ID: 11}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &database.Mirror{ID: 11})
}

func TestRepoHandler_UpdateMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateMirror
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.UpdateMirrorReq{
		MirrorSourceID: 123,
		SourceUrl:      "foo",
	})
	tester.mocks.repo.EXPECT().UpdateMirror(
		tester.Ctx(), types.UpdateMirrorReq{
			Namespace:      "u",
			Name:           "r",
			RepoType:       types.ModelRepo,
			CurrentUser:    "u",
			MirrorSourceID: 123,
			SourceUrl:      "foo",
			SourceRepoPath: "foo",
		},
	).Return(&database.Mirror{ID: 11}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &database.Mirror{ID: 11})
}

func TestRepoHandler_DeleteMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeleteMirror
	})

	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().DeleteMirror(
		tester.Ctx(), types.DeleteMirrorReq{
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
		},
	).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestRepoHandler_RuntimeFrameworkList(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RuntimeFrameworkList
	})

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithQuery("deploy_type", "1")
	tester.mocks.repo.EXPECT().ListRuntimeFramework(
		tester.Ctx(), types.ModelRepo, "u", "r", 1,
	).Return([]types.RuntimeFramework{{FrameName: "f1"}}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, []types.RuntimeFramework{{FrameName: "f1"}})
}

func TestRepoHandler_RuntimeFrameworkCreate(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RuntimeFrameworkCreate
	})

	tester.WithUser().WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.RuntimeFrameworkReq{
		FrameName:   "f1",
		CurrentUser: "u",
	})
	tester.mocks.repo.EXPECT().CreateRuntimeFramework(
		tester.Ctx(), &types.RuntimeFrameworkReq{FrameName: "f1", CurrentUser: "u"},
	).Return(&types.RuntimeFramework{FrameName: "f1"}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.RuntimeFramework{FrameName: "f1"})
}

func TestRepoHandler_RuntimeFrameworkUpdate(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RuntimeFrameworkUpdate
	})

	tester.WithUser().WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.RuntimeFrameworkReq{
		FrameName: "f1",
	})
	tester.WithParam("id", "1")
	tester.mocks.repo.EXPECT().UpdateRuntimeFramework(
		tester.Ctx(), int64(1), &types.RuntimeFrameworkReq{FrameName: "f1", CurrentUser: "u"},
	).Return(&types.RuntimeFramework{FrameName: "f1"}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.RuntimeFramework{FrameName: "f1"})
}

func TestRepoHandler_RuntimeFrameworkDelete(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RuntimeFrameworkDelete
	})
	tester.WithUser().WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.mocks.repo.EXPECT().DeleteRuntimeFramework(
		tester.Ctx(), "u", int64(1),
	).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestRepoHandler_DeployList(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployList
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().ListDeploy(
		tester.Ctx(), types.ModelRepo, "u", "r", "u",
	).Return([]types.DeployRepo{{DeployName: "n"}}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, []types.DeployRepo{{DeployName: "n"}})
}

func TestRepoHandler_DeployDetail(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployDetail
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.mocks.repo.EXPECT().DeployDetail(
		tester.Ctx(), types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.InferenceType,
		},
	).Return(&types.DeployRepo{DeployName: "n"}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.DeployRepo{DeployName: "n"})
}

func TestRepoHandler_DeployInstanceLogs(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployInstanceLogs
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.WithParam("instance", "ii")
	runlogChan := make(chan string)
	tester.mocks.repo.EXPECT().DeployInstanceLogs(
		mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     1,
			DeployType:   types.InferenceType,
			InstanceName: "ii",
		},
	).Return(deploy.NewMultiLogReader(nil, runlogChan), nil)
	cc, cancel := context.WithCancel(tester.Gctx().Request.Context())
	tester.Gctx().Request = tester.Gctx().Request.WithContext(cc)
	go func() {
		runlogChan <- "foo"
		runlogChan <- "bar"
		close(runlogChan)
		cancel()
	}()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:Container\ndata:foo\n\nevent:Container\ndata:bar\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_DeployStatus(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployStatus
	})
	tester.handler.deployStatusCheckInterval = 0
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	cc, cancel := context.WithCancel(tester.Gctx().Request.Context())
	tester.Gctx().Request = tester.Gctx().Request.WithContext(cc)
	tester.mocks.repo.EXPECT().AllowAccessDeploy(
		tester.Gctx().Request.Context(), types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.InferenceType,
		},
	).Return(true, nil)
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).Return("", "s1", []types.Instance{{Name: "i1"}}, nil).Once()
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).RunAndReturn(func(ctx context.Context, rt types.RepositoryType, s1, s2 string, i int64) (string, string, []types.Instance, error) {
		cancel()
		return "", "s3", nil, nil
	}).Once()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:status\ndata:{\"status\":\"s1\",\"details\":[{\"name\":\"i1\",\"status\":\"\"}]}\n\nevent:status\ndata:{\"status\":\"s3\",\"details\":null}\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_SyncMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.SyncMirror
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.mocks.repo.EXPECT().SyncMirror(
		tester.Ctx(), types.ModelRepo, "u", "r", "u",
	).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestRepoHandler_DeployUpdate(t *testing.T) {
	t.Run("not admin", func(t *testing.T) {

		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DeployUpdate
		})
		tester.RequireUser(t)

		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "1")
		tester.mocks.repo.EXPECT().AllowAdminAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(false, nil)
		tester.Execute()
		tester.ResponseEq(
			t, 403, "user not allowed to update deploy", nil,
		)
	})

	t.Run("admin", func(t *testing.T) {

		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DeployUpdate
		})
		tester.RequireUser(t)

		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "1")
		tester.mocks.repo.EXPECT().AllowAdminAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.WithBody(t, &types.DeployUpdateReq{
			MinReplica: tea.Int(1),
			MaxReplica: tea.Int(5),
		})
		tester.mocks.repo.EXPECT().DeployUpdate(tester.Ctx(), types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.InferenceType,
		}, &types.DeployUpdateReq{
			MinReplica: tea.Int(1),
			MaxReplica: tea.Int(5),
		}).Return(nil)
		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, nil)
	})
}

func TestRepoHandler_RuntimeFrameworkListWithType(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RuntimeFrameworkListWithType
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().ListRuntimeFrameworkWithType(
		tester.Ctx(), types.InferenceType,
	).Return([]types.RuntimeFramework{{FrameName: "f1"}}, nil)

	tester.Execute()
	tester.ResponseEq(
		t, 200, tester.OKText, []types.RuntimeFramework{{FrameName: "f1"}},
	)
}

func TestRepoHandler_ServerlessDetail(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessDetail
	})
	tester.RequireUser(t)

	tester.WithParam("id", "1")
	tester.mocks.repo.EXPECT().DeployDetail(
		tester.Ctx(), types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.ServerlessType,
		},
	).Return(&types.DeployRepo{Name: "r"}, nil)

	tester.Execute()
	tester.ResponseEq(
		t, 200, tester.OKText, &types.DeployRepo{Name: "r"},
	)
}

func TestRepoHandler_ServerlessLogs(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessLogs
	})

	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.WithParam("instance", "ii")
	runlogChan := make(chan string)
	tester.mocks.repo.EXPECT().DeployInstanceLogs(
		mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     1,
			DeployType:   types.ServerlessType,
			InstanceName: "ii",
		},
	).Return(deploy.NewMultiLogReader(nil, runlogChan), nil)
	cc, cancel := context.WithCancel(tester.Gctx().Request.Context())
	tester.Gctx().Request = tester.Gctx().Request.WithContext(cc)
	go func() {
		runlogChan <- "foo"
		runlogChan <- "bar"
		close(runlogChan)
		cancel()
	}()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:Container\ndata:foo\n\nevent:Container\ndata:bar\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_ServerlessStatus(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessStatus
	})
	tester.handler.deployStatusCheckInterval = 0
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	cc, cancel := context.WithCancel(tester.Gctx().Request.Context())
	tester.Gctx().Request = tester.Gctx().Request.WithContext(cc)
	tester.mocks.repo.EXPECT().AllowAccessDeploy(
		tester.Gctx().Request.Context(), types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.ServerlessType,
		},
	).Return(true, nil)
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).Return("", "s1", []types.Instance{{Name: "i1"}}, nil).Once()
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).RunAndReturn(func(ctx context.Context, rt types.RepositoryType, s1, s2 string, i int64) (string, string, []types.Instance, error) {
		cancel()
		return "", "s3", nil, nil
	}).Once()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:status\ndata:{\"status\":\"s1\",\"details\":[{\"name\":\"i1\",\"status\":\"\"}]}\n\nevent:status\ndata:{\"status\":\"s3\",\"details\":null}\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_ServelessUpdate(t *testing.T) {

	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessUpdate
	})
	tester.RequireUser(t)

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.WithBody(t, &types.DeployUpdateReq{
		MinReplica: tea.Int(1),
		MaxReplica: tea.Int(5),
	})
	tester.mocks.repo.EXPECT().DeployUpdate(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.ServerlessType,
	}, &types.DeployUpdateReq{
		MinReplica: tea.Int(1),
		MaxReplica: tea.Int(5),
	}).Return(nil)
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestRepoHandler_TreeV2(t *testing.T) {

	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TreeV2
	})
	tester.mocks.repo.EXPECT().TreeV2(mock.Anything, &types.GetTreeRequest{
		Namespace:   "u",
		Name:        "r",
		Path:        "foo",
		Ref:         "main",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Limit:       5,
		Cursor:      "cc",
	}).Return(
		&types.GetRepoFileTreeResp{Files: []*types.File{{Name: "f1"}}}, nil,
	).Once()
	tester.WithParam("path", "foo").WithParam("ref", "main").WithQuery("limit", "5")
	tester.WithKV("repo_type", types.ModelRepo).WithUser().WithQuery("cursor", "cc").Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, &types.GetRepoFileTreeResp{Files: []*types.File{{Name: "f1"}}},
	)
}

func TestRepoHandler_LogsTree(t *testing.T) {

	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.LogsTree
	})
	tester.mocks.repo.EXPECT().LogsTree(mock.Anything, &types.GetLogsTreeRequest{
		Namespace:   "u",
		Name:        "r",
		Path:        "foo",
		Ref:         "main",
		RepoType:    types.ModelRepo,
		CurrentUser: "u",
		Limit:       5,
		Offset:      1,
	}).Return(
		&types.LogsTreeResp{Commits: []*types.CommitForTree{{Name: "c1"}}}, nil,
	).Once()
	tester.WithParam("path", "foo").WithParam("ref", "main").WithQuery("limit", "5")
	tester.WithKV("repo_type", types.ModelRepo).WithUser().WithQuery("offset", "1").Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, &types.LogsTreeResp{Commits: []*types.CommitForTree{{Name: "c1"}}},
	)
}
