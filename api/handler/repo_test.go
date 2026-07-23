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
	"go.temporal.io/sdk/client"
	temporal_mock "go.temporal.io/sdk/mocks"
	workflow_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type RepoTester struct {
	*testutil.GinTester
	handler *RepoHandler
	mocks   struct {
		repo     *mockcomponent.MockRepoComponent
		model    *mockcomponent.MockModelComponent
		dataset  *mockcomponent.MockDatasetComponent
		mirror   *mockcomponent.MockMirrorComponent
		workflow *workflow_mock.MockClient
	}
}

func NewRepoTester(t *testing.T) *RepoTester {
	tester := &RepoTester{GinTester: testutil.NewGinTester()}
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)
	tester.mocks.model = mockcomponent.NewMockModelComponent(t)
	tester.mocks.dataset = mockcomponent.NewMockDatasetComponent(t)
	tester.mocks.mirror = mockcomponent.NewMockMirrorComponent(t)
	tester.mocks.workflow = workflow_mock.NewMockClient(t)
	temporal.Assign(tester.mocks.workflow)
	tester.handler = &RepoHandler{
		c:        tester.mocks.repo,
		m:        tester.mocks.model,
		d:        tester.mocks.dataset,
		mirror:   tester.mocks.mirror,
		temporal: tester.mocks.workflow,
		config: &config.Config{
			MaxRepoBatchNum: 500,
		},
	}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (rt *RepoTester) WithHandleFunc(fn func(rp *RepoHandler) gin.HandlerFunc) *RepoTester {
	rt.Handler(fn(rt.handler))
	return rt
}

func TestRepoHandler_GetRepoSizeByBranch(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.GetRepoSizeByBranch
	})
	tester.WithUser()

	// Set up parameters
	tester.WithParam("branch", "main")
	tester.WithKV("repo_type", types.ModelRepo)

	// Set mock expectation
	expectedResp := types.RepoSizeResponse{TotalSize: 1024, LastCommitSize: 512}
	tester.mocks.repo.EXPECT().GetRepoSizeByBranch(tester.Ctx(), types.ModelRepo, "u", "r", "main", "u").Return(expectedResp, nil)

	// Execute request
	tester.Execute()

	// Verify response
	tester.ResponseEq(t, http.StatusOK, tester.OKText, expectedResp)
}

func TestRepoHandler_ScanIndustryTags(t *testing.T) {
	t.Run("dataset success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ScanIndustryTags
		})
		tester.WithParam("repo_type", "dataset")
		run := &temporal_mock.WorkflowRun{}
		tester.mocks.workflow.EXPECT().ExecuteWorkflow(
			tester.Ctx(),
			client.StartWorkflowOptions{
				TaskQueue: workflow.HandlePushQueueName,
				ID:        "repo-industry-scan-dataset-u-r",
			},
			mock.Anything,
			types.ScanRepoIndustryTagsReq{
				Namespace: "u",
				Name:      "r",
				RepoType:  types.DatasetRepo,
			},
		).Return(run, nil)

		tester.Execute()
		tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)
	})

	t.Run("model success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ScanIndustryTags
		})
		tester.WithParam("repo_type", "model")
		run := &temporal_mock.WorkflowRun{}
		tester.mocks.workflow.EXPECT().ExecuteWorkflow(
			tester.Ctx(),
			client.StartWorkflowOptions{
				TaskQueue: workflow.HandlePushQueueName,
				ID:        "repo-industry-scan-model-u-r",
			},
			mock.Anything,
			types.ScanRepoIndustryTagsReq{
				Namespace: "u",
				Name:      "r",
				RepoType:  types.ModelRepo,
			},
		).Return(run, nil)

		tester.Execute()
		tester.ResponseEq(t, http.StatusOK, tester.OKText, nil)
	})

	t.Run("invalid repo type", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ScanIndustryTags
		})
		tester.WithParam("repo_type", "space")

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusBadRequest)
	})

	t.Run("workflow error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ScanIndustryTags
		})
		tester.WithParam("repo_type", "dataset")
		tester.mocks.workflow.EXPECT().ExecuteWorkflow(
			tester.Ctx(),
			client.StartWorkflowOptions{
				TaskQueue: workflow.HandlePushQueueName,
				ID:        "repo-industry-scan-dataset-u-r",
			},
			mock.Anything,
			types.ScanRepoIndustryTagsReq{
				Namespace: "u",
				Name:      "r",
				RepoType:  types.DatasetRepo,
			},
		).Return(nil, errors.New("workflow failed"))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusInternalServerError)
	})
}

func TestRepoHandler_GetRepoSizeByBranch_Error(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.GetRepoSizeByBranch
	})
	tester.WithUser()

	// Set up parameters
	tester.WithParam("branch", "main")
	tester.WithKV("repo_type", types.ModelRepo)

	// Set mock expectation with error
	tester.mocks.repo.EXPECT().GetRepoSizeByBranch(tester.Ctx(), types.ModelRepo, "u", "r", "main", "u").Return(types.RepoSizeResponse{}, errors.New("failed to get repo size"))

	// Execute request
	tester.Execute()

	// Verify response
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestRepoHandler_CreateFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CreateFile
	})
	tester.WithUser()

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

func TestRepoHandler_ServerlessVersionLogs(t *testing.T) {
	// Test case: Invalid deploy ID format
	t.Run("invalid_deploy_id", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ServerlessVersionLogs
		})
		tester.WithUser()

		// Set invalid deploy ID
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "invalid")
		tester.WithParam("commit_id", "test-commit-id")

		// Execute request
		tester.Execute()

		// Verify response
		require.Equal(t, http.StatusBadRequest, tester.Response().Code)
		require.Contains(t, tester.Response().Body.String(), "Invalid deploy ID format")
	})

	// Test case: DeployInstanceLogs returns error
	t.Run("deploy_instance_logs_error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ServerlessVersionLogs
		})
		tester.WithUser()

		// Set valid parameters
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "123")
		tester.WithParam("commit_id", "test-commit-id")

		// Mock error response
		expectedDeployID := int64(123)
		tester.mocks.repo.EXPECT().DeployInstanceLogs(mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     expectedDeployID,
			DeployType:   types.ServerlessType,
			InstanceName: "",
			Since:        "",
			CommitID:     "test-commit-id",
		}).Return(nil, errors.New("internal server error"))

		// Execute request
		tester.Execute()

		// Verify response
		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})

	// Test case: DeployInstanceLogs returns forbidden error
	t.Run("deploy_instance_logs_forbidden", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ServerlessVersionLogs
		})
		tester.WithUser()

		// Set valid parameters
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "123")
		tester.WithParam("commit_id", "test-commit-id")

		// Mock forbidden error response
		expectedDeployID := int64(123)
		tester.mocks.repo.EXPECT().DeployInstanceLogs(mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     expectedDeployID,
			DeployType:   types.ServerlessType,
			InstanceName: "",
			Since:        "",
			CommitID:     "test-commit-id",
		}).Return(nil, errorx.ErrForbidden)

		// Execute request
		tester.Execute()

		// Verify response
		require.Equal(t, http.StatusForbidden, tester.Response().Code)
	})

	// Test case: No logs found
	t.Run("no_logs_found", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ServerlessVersionLogs
		})
		tester.WithUser()

		// Set valid parameters
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "123")
		tester.WithParam("commit_id", "test-commit-id")

		// Mock empty logs response
		expectedDeployID := int64(123)
		buildLogChan := make(chan string)
		close(buildLogChan)
		// Create a MultiLogReader with nil runLogs
		// Note: We're passing a nil channel directly
		var nilRunLogChan <-chan string
		multiLogReader := deploy.NewMultiLogReader(buildLogChan, nilRunLogChan)

		tester.mocks.repo.EXPECT().DeployInstanceLogs(mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     expectedDeployID,
			DeployType:   types.ServerlessType,
			InstanceName: "",
			Since:        "",
			CommitID:     "test-commit-id",
		}).Return(multiLogReader, nil)

		// Execute request
		tester.Execute()

		// Verify response
		require.Equal(t, http.StatusInternalServerError, tester.Response().Code)
		require.Contains(t, tester.Response().Body.String(), "don't find any deploy instance log")
	})

	// Test case: Normal case with all parameters
	t.Run("normal_case_all_params", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.ServerlessVersionLogs
		})
		tester.WithUser()

		// Set valid parameters
		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "123")
		tester.WithParam("commit_id", "test-commit-id")
		tester.WithQuery("instance_name", "instance-1")
		tester.WithQuery("since", "1h")

		// Mock response with a timeout
		expectedDeployID := int64(123)
		buildLogChan := make(chan string)
		runLogChan := make(chan string)

		// Create multi log reader
		multiLogReader := deploy.NewMultiLogReader(buildLogChan, runLogChan)

		// Mock the expected call
		tester.mocks.repo.EXPECT().DeployInstanceLogs(mock.Anything, types.DeployActReq{
			RepoType:     types.ModelRepo,
			Namespace:    "u",
			Name:         "r",
			CurrentUser:  "u",
			DeployID:     expectedDeployID,
			DeployType:   types.ServerlessType,
			InstanceName: "instance-1",
			Since:        "1h",
			CommitID:     "test-commit-id",
		}).Return(multiLogReader, nil)

		// Set a timeout for the request
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Execute request in a goroutine with timeout
		done := make(chan bool)
		go func() {
			tester.Execute()
			done <- true
		}()

		// Wait for either the request to complete or the timeout
		select {
		case <-done:
			// Request completed successfully
		case <-ctx.Done():
			// Timeout occurred, which is expected for SSE streaming
		}

		// Verify that the mock was called

		// Close channels to clean up
		close(buildLogChan)
		close(runLogChan)
	})
}

func TestRepoHandler_ServerlessVersionLogs_SSE(t *testing.T) {
	// This is a more advanced test that would require capturing SSE events
	// For simplicity, we'll test the basic functionality in the main test function
}

func TestRepoHandler_UpdateFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateFile
	})
	tester.WithUser()

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

func TestRepoHandler_DeleteFile(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeleteFile
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeleteFile(tester.Ctx(), &types.DeleteFileReq{
		Message:     "foo",
		Branch:      "main",
		Username:    "u",
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		FilePath:    "foo",
		RepoType:    types.ModelRepo,
		OriginPath:  "",
	}).Return(&types.DeleteFileResp{}, nil)
	tester.WithParam("file_path", "foo")
	tester.WithKV("repo_type", types.ModelRepo)
	req := &types.DeleteFileReq{
		Message: "foo",
		Branch:  "main",
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.DeleteFileResp{})
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
	}).Return([]types.Commit{{ID: "c1", Message: `<img src=x onerror=alert(1)>`}}, &types.RepoPageOpts{Total: 100, PageCount: 1}, nil)
	tester.WithParam("file_path", "foo")
	tester.WithQuery("ref", "main")
	tester.AddPagination(1, 10)
	tester.WithKV("repo_type", types.ModelRepo)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, gin.H{
		"commits":    []types.Commit{{ID: "c1", Message: `&lt;img src=x onerror=alert(1)&gt;`}},
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
		tester.mocks.repo.EXPECT().LastCommit(mock.Anything, mock.Anything).Return(nil, errorx.ErrForbidden).Once()
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

func TestRepoHandler_DownloadCodeZip(t *testing.T) {
	t.Run("success with ref", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()
		tester.WithParam("ref", "feature-branch")

		expectedZip := []byte("zip-data")
		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "feature-branch",
		}, "u").Return(expectedZip, nil)

		tester.Execute()

		require.Equal(t, http.StatusOK, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/zip", headers.Get("Content-Type"))
		require.Equal(t, `attachment; filename=r-feature-branch.zip`, headers.Get("Content-Disposition"))
		require.Equal(t, "zip-data", tester.Response().Body.String())
	})

	t.Run("success with ref containing slash", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()
		tester.WithParam("ref", "feature/branch")

		expectedZip := []byte("zip-data")
		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "feature/branch",
		}, "u").Return(expectedZip, nil)

		tester.Execute()

		require.Equal(t, http.StatusOK, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/zip", headers.Get("Content-Type"))
		require.Equal(t, `attachment; filename=r-feature-branch.zip`, headers.Get("Content-Disposition"))
		require.Equal(t, "zip-data", tester.Response().Body.String())
	})

	t.Run("success without ref", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		expectedZip := []byte("zip-data")
		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(expectedZip, nil)

		tester.Execute()

		require.Equal(t, http.StatusOK, tester.Response().Code)
		headers := tester.Response().Header()
		require.Equal(t, "application/zip", headers.Get("Content-Type"))
		require.Equal(t, `attachment; filename=r.zip`, headers.Get("Content-Disposition"))
		require.Equal(t, "zip-data", tester.Response().Body.String())
	})

	t.Run("forbidden error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(nil, errorx.ErrForbiddenMsg("no permission"))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusForbidden)
	})

	t.Run("server error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(nil, errors.New("failed"))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusInternalServerError)
	})

	t.Run("not found error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(nil, errorx.RepoNotFound(errors.New("not found"), errorx.Ctx()))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusNotFound)
	})

	t.Run("no default branch error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(nil, errorx.RepoNoDefaultBranch(errorx.Ctx()))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusBadRequest)
	})

	t.Run("code zip download failed error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DownloadCodeZip
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().DownloadCodeZip(tester.Ctx(), types.DownloadCodeZipReq{
			Namespace: "u",
			Name:      "r",
			Revision:  "",
		}, "u").Return(nil, errorx.CodeZipDownloadFailed(errors.New("git error"), errorx.Ctx()))

		tester.Execute()
		tester.ResponseEqCode(t, http.StatusInternalServerError)
	})
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
	tester.WithUser()

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
		tester.mocks.repo.EXPECT().Tree(mock.Anything, mock.Anything).Return(nil, errorx.ErrForbidden).Once()
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
	tester.WithUser()

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
	).Return(&types.File{Size: 100, SHA: "def"}, &types.Commit{ID: "abc"}, nil)

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "100", headers.Get("Content-Length"))
	require.Equal(t, "abc", headers.Get("X-Repo-Commit"))
	require.Equal(t, "def", headers.Get("ETag"))
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
	).Return(&types.CommitResponse{Commit: &types.Commit{ID: "foo", Message: `<script>alert(1)</script>`}}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.CommitResponse{Commit: &types.Commit{ID: "foo", Message: `&lt;script&gt;alert(1)&lt;/script&gt;`}})
}

func TestRepoHandler_CreateMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CreateMirror
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.CreateMirrorReq{
		SourceUrl:      "https://foo.com",
		MirrorSourceID: 12,
	})
	tester.mocks.mirror.EXPECT().CreateMirror(
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

func TestRepoHandler_GetMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.GetMirror
	})
	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.mirror.EXPECT().GetMirror(
		tester.Ctx(), types.GetMirrorReq{
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			CurrentUser: "u",
		},
	).Return(&types.Mirror{ID: 11, SourceUrl: "test"}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.Mirror{ID: 11, SourceUrl: "test"})
}

func TestRepoHandler_UpdateMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.UpdateMirror
	})
	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, &types.UpdateMirrorReq{
		MirrorSourceID: 123,
		SourceUrl:      "foo",
	})
	tester.mocks.mirror.EXPECT().UpdateMirror(
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

func TestRepoHandler_MirrorSourceRepoAuthInvalid(t *testing.T) {
	authErr := errorx.MirrorSourceRepoAuthInvalid(errors.New("credentials are incomplete"), errorx.Ctx())
	want := httpbase.R{Code: "MIRROR-ERR-5", Msg: "MIRROR-ERR-5: credentials are incomplete"}

	t.Run("CreateMirror", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.CreateMirror
		})
		tester.WithUser().WithKV("repo_type", types.ModelRepo).WithBody(t, &types.CreateMirrorReq{
			SourceUrl: "https://example.com/source/repo.git",
		})
		tester.mocks.mirror.EXPECT().CreateMirror(tester.Ctx(), mock.Anything).Return(nil, authErr)

		tester.Execute()

		tester.ResponseEqSimple(t, 400, want)
	})

	t.Run("UpdateMirror", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.UpdateMirror
		})
		tester.WithUser().WithKV("repo_type", types.ModelRepo).WithBody(t, &types.UpdateMirrorReq{
			SourceUrl: "https://example.com/source/repo.git",
		})
		tester.mocks.mirror.EXPECT().UpdateMirror(tester.Ctx(), mock.Anything).Return(nil, authErr)

		tester.Execute()

		tester.ResponseEqSimple(t, 400, want)
	})
}

// TestRepoHandler_MirrorSourceBadRequest verifies malformed mirror source URLs return a bad request.
func TestRepoHandler_MirrorSourceBadRequest(t *testing.T) {
	badRequestErr := errorx.BadRequest(errors.New("invalid source git clone url"), errorx.Ctx())
	want := httpbase.R{Code: "REQ-ERR-0", Msg: "REQ-ERR-0: invalid source git clone url"}

	t.Run("CreateMirror", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.CreateMirror
		})
		tester.WithUser().WithKV("repo_type", types.ModelRepo).WithBody(t, &types.CreateMirrorReq{
			SourceUrl: "https://example.com/source/repo.git",
		})
		tester.mocks.mirror.EXPECT().CreateMirror(tester.Ctx(), mock.Anything).Return(nil, badRequestErr)

		tester.Execute()

		tester.ResponseEqSimple(t, http.StatusBadRequest, want)
	})

	t.Run("UpdateMirror", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.UpdateMirror
		})
		tester.WithUser().WithKV("repo_type", types.ModelRepo).WithBody(t, &types.UpdateMirrorReq{
			SourceUrl: "https://example.com/source/repo.git",
		})
		tester.mocks.mirror.EXPECT().UpdateMirror(tester.Ctx(), mock.Anything).Return(nil, badRequestErr)

		tester.Execute()

		tester.ResponseEqSimple(t, http.StatusBadRequest, want)
	})
}

func TestRepoHandler_DeleteMirror(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeleteMirror
	})

	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.mirror.EXPECT().DeleteMirror(
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
	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.mocks.repo.EXPECT().ListDeploy(
		tester.Ctx(), types.ModelRepo, "u", "r", "u",
	).Return([]types.DeployRequest{{DeployName: "n"}}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, []types.DeployRequest{{DeployName: "n"}})
}

func TestRepoHandler_DeployDetail(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployDetail
	})
	tester.WithUser()

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
	).Return(&types.DeployRequest{DeployName: "n"}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.DeployRequest{DeployName: "n"})
}

func TestRepoHandler_DeployInstanceLogs(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.DeployInstanceLogs
	})
	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.WithParam("instance", "ii")
	runlogChan := make(chan string)
	tester.mocks.repo.EXPECT().DeployInstanceLogs(
		mock.Anything, types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.InferenceType,
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
	tester.WithUser()

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
	).Return(types.ModelStatusEventData{Details: []types.Instance{{Name: "i1"}},
		Status: "s1", Message: "", Reason: ""}, nil).Once()
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).RunAndReturn(func(ctx context.Context, rt types.RepositoryType, s1, s2 string, i int64) (types.ModelStatusEventData, error) {
		cancel()
		return types.ModelStatusEventData{
			Status: "s3", Message: "", Reason: ""}, nil
	}).Once()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:status\ndata:{\"status\":\"s1\",\"details\":[{\"name\":\"i1\",\"status\":\"\"}],\"message\":\"\",\"reason\":\"\"}\n\nevent:status\ndata:{\"status\":\"s3\",\"details\":null,\"message\":\"\",\"reason\":\"\"}\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_SyncMirror(t *testing.T) {
	for _, tc := range []struct {
		name   string
		urgent bool
		body   bool
	}{
		{name: "defaults to normal"},
		{name: "requests urgent", urgent: true, body: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
				return rp.SyncMirror
			})
			tester.WithUser()
			tester.WithKV("repo_type", types.ModelRepo)
			tester.WithParam("id", "1")
			if tc.body {
				tester.WithBody(t, &types.SyncMirrorParams{Urgent: tc.urgent})
			}
			tester.mocks.mirror.EXPECT().SyncMirror(tester.Ctx(), types.SyncMirrorReq{
				RepoType: types.ModelRepo, Namespace: "u", Name: "r", CurrentUser: "u", Urgent: tc.urgent,
			}).Return(nil).Once()

			tester.Execute()
			tester.ResponseEq(t, 200, tester.OKText, nil)
		})
	}

	t.Run("rejects invalid body", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.SyncMirror
		})
		tester.WithBody(t, "invalid")

		tester.Execute()

		require.Equal(t, http.StatusBadRequest, tester.Response().Code)
	})
}

func TestRepoHandler_CreateRepo(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *types.CreateRepoReq
		setupMocks     func(*RepoTester)
		expectedStatus int
		expectedError  string
		checkResponse  func(*testing.T, *RepoTester)
	}{
		{
			name: "successful model repo creation",
			setupRequest: func() *types.CreateRepoReq {
				return &types.CreateRepoReq{
					Name:        "test-model",
					Nickname:    "Test Model",
					Description: "A test model repository",
					Private:     false,
					RepoType:    types.ModelRepo,
					License:     "MIT",
				}
			},
			setupMocks: func(tester *RepoTester) {
				expectedModelReq := &types.CreateModelReq{
					CreateRepoReq: types.CreateRepoReq{
						Name:        "test-model",
						Nickname:    "Test Model",
						Description: "A test model repository",
						Private:     false,
						RepoType:    types.ModelRepo,
						License:     "MIT",
						Username:    "u",
						Namespace:   "u",
					},
				}
				expectedResponse := &types.Model{
					ID:          1,
					Name:        "test-model",
					Nickname:    "Test Model",
					Description: "A test model repository",
					Private:     false,
					Path:        "u/test-model",
				}
				tester.mocks.model.EXPECT().Create(
					common.GinContextToStdContext(tester.Gctx()), expectedModelReq,
				).Return(expectedResponse, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, tester *RepoTester) {
				var response struct {
					Code int    `json:"code"`
					URL  string `json:"url"`
				}
				err := json.Unmarshal(tester.Response().Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, 0, response.Code)
				require.NotNil(t, response.URL)
			},
		},
		{
			name: "successful dataset repo creation",
			setupRequest: func() *types.CreateRepoReq {
				return &types.CreateRepoReq{
					Name:        "test-dataset",
					Nickname:    "Test Dataset",
					Description: "A test dataset repository",
					Private:     true,
					RepoType:    types.DatasetRepo,
					License:     "Apache-2.0",
				}
			},
			setupMocks: func(tester *RepoTester) {
				expectedDatasetReq := &types.CreateDatasetReq{
					CreateRepoReq: types.CreateRepoReq{
						Name:        "test-dataset",
						Nickname:    "Test Dataset",
						Description: "A test dataset repository",
						Private:     true,
						RepoType:    types.DatasetRepo,
						License:     "Apache-2.0",
						Username:    "u",
						Namespace:   "u",
					},
				}
				expectedResponse := &types.Dataset{
					ID:          2,
					Name:        "test-dataset",
					Nickname:    "Test Dataset",
					Description: "A test dataset repository",
					Private:     true,
					Path:        "u/test-dataset",
				}
				tester.mocks.dataset.EXPECT().Create(
					common.GinContextToStdContext(tester.Gctx()), expectedDatasetReq,
				).Return(expectedResponse, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, tester *RepoTester) {
				var response struct {
					Code int    `json:"code"`
					URL  string `json:"url"`
				}
				err := json.Unmarshal(tester.Response().Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, 0, response.Code)
				require.NotNil(t, response.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
				return rp.CreateRepo
			})
			tester.WithUser()

			// Setup mocks
			tt.setupMocks(tester)

			// Setup request body
			req := tt.setupRequest()
			tester.WithBody(t, req)

			// Execute request
			tester.Execute()

			// Check status code
			require.Equal(t, tt.expectedStatus, tester.Response().Code)

			// Run additional response checks
			if tt.checkResponse != nil {
				tt.checkResponse(t, tester)
			}
		})
	}
}

func TestRepoHandler_DeployUpdate(t *testing.T) {
	t.Run("not admin", func(t *testing.T) {

		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DeployUpdate
		})
		tester.WithUser()

		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "1")
		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(false, nil)
		tester.Execute()
		tester.ResponseEq(
			t, 403, "user is not authorized to read this repository for update deploy", nil,
		)
	})

	t.Run("admin", func(t *testing.T) {

		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.DeployUpdate
		})
		tester.WithUser()

		tester.WithKV("repo_type", types.ModelRepo)
		tester.WithParam("id", "1")
		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
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
	tester.WithUser()

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
	tester.WithUser()

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
	).Return(&types.DeployRequest{Name: "r"}, nil)

	tester.Execute()
	tester.ResponseEq(
		t, 200, tester.OKText, &types.DeployRequest{Name: "r"},
	)
}

func TestRepoHandler_ServerlessLogs(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessLogs
	})

	tester.WithUser()

	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithParam("id", "1")
	tester.WithParam("instance", "ii")
	runlogChan := make(chan string)
	tester.mocks.repo.EXPECT().DeployInstanceLogs(
		mock.Anything, types.DeployActReq{
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployID:    1,
			DeployType:  types.ServerlessType,
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
	tester.WithUser()

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
	).Return(types.ModelStatusEventData{Details: []types.Instance{{Name: "i1"}},
		Status: "s1"}, nil).Once()
	tester.mocks.repo.EXPECT().DeployStatus(
		mock.Anything, types.ModelRepo, "u", "r", int64(1),
	).RunAndReturn(func(ctx context.Context, rt types.RepositoryType, s1, s2 string, i int64) (types.ModelStatusEventData, error) {
		cancel()
		return types.ModelStatusEventData{
			Status: "s3"}, nil
	}).Once()

	tester.Execute()
	require.Equal(t, 200, tester.Response().Code)
	headers := tester.Response().Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:status\ndata:{\"status\":\"s1\",\"details\":[{\"name\":\"i1\",\"status\":\"\"}],\"message\":\"\",\"reason\":\"\"}\n\nevent:status\ndata:{\"status\":\"s3\",\"details\":null,\"message\":\"\",\"reason\":\"\"}\n\n",
		tester.Response().Body.String(),
	)

}

func TestRepoHandler_ServelessUpdate(t *testing.T) {

	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.ServerlessUpdate
	})
	tester.WithUser()

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
	t.Run("returns tree without checking mirror status", func(t *testing.T) {
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
	})

	t.Run("returns accepted while root repository data is syncing", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.TreeV2
		})
		tester.mocks.repo.EXPECT().TreeV2(mock.Anything, mock.Anything).Return(nil, errorx.ErrGitFileNotFound)
		tester.mocks.mirror.EXPECT().MirrorFromSaasStatus(tester.Ctx(), types.MirrorFromSaasStatusReq{
			Namespace: "u", Name: "r", RepoType: types.ModelRepo, CurrentUser: "u",
		}).Return(&types.MirrorSyncStatusResponse{
			TaskID: 30, Status: types.MirrorRepoSyncStart, Phase: types.MirrorSyncPhaseRepo,
		}, nil)
		remoteTree := &types.GetRepoFileTreeResp{
			Files:  []*types.File{{Name: "config.json", Path: "config.json", Size: 42}},
			Cursor: "next",
		}
		tester.mocks.repo.EXPECT().RemoteTree(tester.Ctx(), mock.Anything).Return(remoteTree, nil)

		tester.WithParam("ref", "main").WithKV("repo_type", types.ModelRepo).WithUser().Execute()

		tester.ResponseEqSimple(t, http.StatusAccepted, httpbase.R{
			Code: "MIRROR-ERR-1",
			Msg:  "MIRROR-ERR-1: repository synchronization is in progress",
			Data: remoteTree,
			Context: map[string]interface{}{
				"task_id": int64(30), "status": types.MirrorRepoSyncStart,
			},
		})
	})

	t.Run("returns conflict after root repository sync terminates", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.TreeV2
		})
		tester.mocks.repo.EXPECT().TreeV2(mock.Anything, mock.Anything).Return(nil, errorx.ErrGitCommitNotFound)
		tester.mocks.mirror.EXPECT().MirrorFromSaasStatus(tester.Ctx(), types.MirrorFromSaasStatusReq{
			Namespace: "u", Name: "r", RepoType: types.ModelRepo, CurrentUser: "u",
		}).Return(&types.MirrorSyncStatusResponse{
			TaskID: 30, Status: types.MirrorRepoSyncFailed, Phase: types.MirrorSyncPhaseRepo,
			Terminal: true, FailureReason: types.MirrorSyncFailureRepoRetryExhausted,
		}, nil)

		tester.WithParam("ref", "main").WithKV("repo_type", types.ModelRepo).WithUser().Execute()

		tester.ResponseEqSimple(t, http.StatusConflict, httpbase.R{
			Code: "MIRROR-ERR-2",
			Msg:  "MIRROR-ERR-2: repository synchronization failed",
			Context: map[string]interface{}{
				"task_id": int64(30), "status": types.MirrorRepoSyncFailed,
			},
		})
	})

	t.Run("reports cancellation when root repository sync is cancelled", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.TreeV2
		})
		tester.mocks.repo.EXPECT().TreeV2(mock.Anything, mock.Anything).Return(nil, errorx.ErrGitCommitNotFound)
		tester.mocks.mirror.EXPECT().MirrorFromSaasStatus(tester.Ctx(), types.MirrorFromSaasStatusReq{
			Namespace: "u", Name: "r", RepoType: types.ModelRepo, CurrentUser: "u",
		}).Return(&types.MirrorSyncStatusResponse{
			TaskID: 31, Status: types.MirrorCanceled, Phase: types.MirrorSyncPhaseDone,
			Terminal: true, FailureReason: types.MirrorSyncFailureCanceled,
		}, nil)

		tester.WithParam("ref", "main").WithKV("repo_type", types.ModelRepo).WithUser().Execute()

		tester.ResponseEqSimple(t, http.StatusConflict, httpbase.R{
			Code: "MIRROR-ERR-4",
			Msg:  "MIRROR-ERR-4: repository synchronization was canceled",
			Context: map[string]interface{}{
				"task_id": int64(31), "status": types.MirrorCanceled,
				"failure_reason": types.MirrorSyncFailureCanceled,
			},
		})
	})

	t.Run("preserves nested file not found behavior", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.TreeV2
		})
		tester.mocks.repo.EXPECT().TreeV2(mock.Anything, mock.Anything).Return(nil, errorx.ErrGitFileNotFound)

		tester.WithParam("path", "missing").WithParam("ref", "main").WithKV("repo_type", types.ModelRepo).WithUser().Execute()

		tester.ResponseEqCode(t, http.StatusOK)
	})
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

func TestRepoHandler_RemoteTree(t *testing.T) {

	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RemoteTree
	})
	tester.mocks.repo.EXPECT().RemoteTree(mock.Anything, &types.GetTreeRequest{
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

func TestRepoHandler_RemoteDiff(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.RemoteDiff
	})
	tester.mocks.repo.EXPECT().RemoteDiff(mock.Anything, types.GetDiffBetweenCommitsReq{
		Namespace:     "u",
		Name:          "r",
		RepoType:      types.ModelRepo,
		LeftCommitID:  "left",
		RightCommitID: "",
		CurrentUser:   "u",
	}).Return(
		[]types.RemoteDiffs{
			{
				Added:    []string{"file1"},
				Removed:  []string{"file2"},
				Modified: []string{"file3"},
			},
		}, nil,
	).Once()
	tester.WithParam("path", "CSG_u/r").WithKV("repo_type", types.ModelRepo).WithQuery("left_commit_id", "left").WithUser().Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, []types.RemoteDiffs{
			{
				Added:    []string{"file1"},
				Removed:  []string{"file2"},
				Modified: []string{"file3"},
			},
		},
	)
}

func TestRepoHandler_Preupload(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.Preupload
	})
	req := types.PreuploadReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		Revision:    "main",
		CurrentUser: "u",
		Files: []types.PreuploadFile{
			{
				Path:   "file1",
				Sample: "",
				Size:   1000,
			},
		},
	}
	tester.mocks.repo.EXPECT().Preupload(mock.Anything, req).Return(
		&types.PreuploadResp{
			Files: []types.PreuploadRespFile{
				{
					OID:          "oid1",
					Path:         "file1",
					UploadMode:   "lfs",
					ShouldIgnore: false,
				},
			},
		}, nil,
	).Once()
	tester.WithParam("path", "CSG_u/r").WithKV("repo_type", types.ModelRepo).WithParam("revision", "main").WithBody(t, req).WithUser().Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, &types.PreuploadResp{
			Files: []types.PreuploadRespFile{
				{
					OID:          "oid1",
					Path:         "file1",
					UploadMode:   "lfs",
					ShouldIgnore: false,
				},
			},
		},
	)
}

func TestRepoHandler_CommitFiles(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.CommitFiles
	})
	req := types.CommitFilesReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.ModelRepo,
		Revision:    "main",
		CurrentUser: "u",
		Message:     "msg",
		Files: []types.CommitFileReq{
			{
				Path:    "file1",
				Action:  types.CommitActionCreate,
				Content: "",
			},
		},
	}
	tester.mocks.repo.EXPECT().CommitFiles(mock.Anything, req).Return(nil).Once()
	tester.WithParam("path", "CSG_u/r").WithKV("repo_type", types.ModelRepo).WithParam("revision", "main").WithBody(t, req).WithUser().Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, nil,
	)
}

func TestRepoHandler_GetRepos(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.GetRepos
	})
	tester.mocks.repo.EXPECT().GetRepos(mock.Anything, "search", "u", types.ModelRepo).Return([]string{}, nil).Once()
	tester.WithQuery("type", "model").WithQuery("search", "search").WithUser().Execute()

	tester.ResponseEq(
		t, 200, tester.OKText, []string{},
	)
}

func TestRepoHandler_BatchGetRepoExtra(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.BatchGetRepoExtra
		})
		tester.WithBody(t, types.BatchRepoExtraReq{RepoIDs: []int64{1, 2}}).WithUser()

		tester.mocks.repo.EXPECT().BatchGetRepoExtra(tester.Ctx(), []int64{1, 2}, "u").Return([]types.RepoExtraItem{
			{RepoID: 1, Size: 100, LastCommitSize: 50},
			{RepoID: 2, Size: 200, LastCommitSize: 80},
		}, nil)

		tester.Execute()

		tester.ResponseEq(t, http.StatusOK, tester.OKText, []types.RepoExtraItem{
			{RepoID: 1, Size: 100, LastCommitSize: 50},
			{RepoID: 2, Size: 200, LastCommitSize: 80},
		})
	})

	t.Run("too many IDs", func(t *testing.T) {
		ids := make([]int64, 501)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.BatchGetRepoExtra
		})
		tester.WithBody(t, types.BatchRepoExtraReq{RepoIDs: ids}).WithUser()

		tester.Execute()

		tester.ResponseEqCode(t, http.StatusBadRequest)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.BatchGetRepoExtra
		})
		tester.WithBody(t, "not a valid json").WithUser()

		tester.Execute()

		tester.ResponseEqCode(t, http.StatusBadRequest)
	})

	t.Run("server error", func(t *testing.T) {
		tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
			return rp.BatchGetRepoExtra
		})
		tester.WithBody(t, types.BatchRepoExtraReq{RepoIDs: []int64{1}}).WithUser()

		tester.mocks.repo.EXPECT().BatchGetRepoExtra(tester.Ctx(), []int64{1}, "u").Return(nil, errors.New("db error"))

		tester.Execute()

		tester.ResponseEqCode(t, http.StatusInternalServerError)
	})
}

func TestRepoHandler_TransferOwnership_Success(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), types.TransferRepoReq{
		RepoType:     types.ModelRepo,
		Namespace:    "u",
		Name:         "r",
		NewNamespace: "targetns",
		CurrentUser:  "u",
	}).Return(nil)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusOK)
}

func TestRepoHandler_TransferOwnership_BadRequest(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrBadRequest)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestRepoHandler_TransferOwnership_Forbidden(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrForbiddenMsg("users do not have permission"))

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusForbidden)
}

func TestRepoHandler_TransferOwnership_ServerError(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errors.New("internal error"))

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestRepoHandler_TransferOwnership_NoSourcePermission(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrNoSourceTransferPermission)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusForbidden)
}

func TestRepoHandler_TransferOwnership_SameNamespace(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "same"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrTransferSameNamespace)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestRepoHandler_TransferOwnership_TargetExists(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrTransferTargetExists)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestRepoHandler_TransferOwnership_NotSupported(t *testing.T) {
	tester := NewRepoTester(t).WithHandleFunc(func(rp *RepoHandler) gin.HandlerFunc {
		return rp.TransferOwnership
	})

	tester.WithUser()
	tester.WithKV("repo_type", types.ModelRepo)
	tester.WithBody(t, types.TransferRepoReq{NewNamespace: "targetns"})

	tester.mocks.repo.EXPECT().TransferOwnership(tester.Ctx(), mock.Anything).
		Return(errorx.ErrTransferNotSupported)

	tester.Execute()

	tester.ResponseEqCode(t, http.StatusBadRequest)
}
