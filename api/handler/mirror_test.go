package handler

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type MirrorTester struct {
	*testutil.GinTester
	handler *MirrorHandler
	mocks   struct {
		mirror *mockcomponent.MockMirrorComponent
	}
}

func NewMirrorTester(t *testing.T) *MirrorTester {
	tester := &MirrorTester{GinTester: testutil.NewGinTester()}
	tester.mocks.mirror = mockcomponent.NewMockMirrorComponent(t)

	tester.handler = &MirrorHandler{
		mirror: tester.mocks.mirror,
	}
	tester.WithParam("mirrorId", "testMirrorId")
	tester.WithParam("userId", "testUserId")
	return tester
}

func (t *MirrorTester) WithHandleFunc(fn func(h *MirrorHandler) gin.HandlerFunc) *MirrorTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMirrorHandler_CreateMirrorRepo(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser()
	private := false
	createTargetRepo := true

	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.MatchedBy(func(req types.CreateMirrorRepoReq) bool {
		return req.SourceNamespace == "ns" &&
			req.SourceName == "sn" &&
			req.CurrentUser == "u" &&
			req.MirrorSourceID == 1 &&
			req.RepoType == types.ModelRepo &&
			req.SourceGitCloneUrl == "url" &&
			req.ForkNamespace == "target-ns" &&
			req.ForkName == "target-name" &&
			req.Private != nil &&
			!*req.Private &&
			req.CreateTargetRepo != nil &&
			*req.CreateTargetRepo &&
			req.Priority == types.HighMirrorPriority
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "ns",
		SourceName:        "sn",
		MirrorSourceID:    1,
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "url",
		ForkNamespace:     "target-ns",
		ForkName:          "target-name",
		Private:           &private,
		CreateTargetRepo:  &createTargetRepo,
		Priority:          types.HighMirrorPriority,
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestMirrorHandler_CreateMirrorRepoRequiresForkTarget(t *testing.T) {
	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "legacy target fields are ignored",
			body: map[string]any{
				"source_namespace": "ns",
				"source_name":      "sn",
				"mirror_source_id": 1,
				"repo_type":        types.ModelRepo,
				"source_url":       "url",
				"target_namespace": "target-ns",
				"target_repo_path": "target-name",
			},
		},
		{
			name: "missing fork name",
			body: map[string]any{
				"source_namespace": "ns",
				"source_name":      "sn",
				"mirror_source_id": 1,
				"repo_type":        types.ModelRepo,
				"source_url":       "url",
				"fork_namespace":   "target-ns",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
				return h.CreateMirrorRepo
			})
			tester.WithUser().WithBody(t, tc.body).Execute()

			tester.ResponseEqSimple(t, 400, gin.H{"msg": "fork_namespace and fork_name are required"})
		})
	}
}

func TestMirrorHandler_SourceRepoAuthInvalid(t *testing.T) {
	authErr := errorx.MirrorSourceRepoAuthInvalid(errors.New("credentials are incomplete"), errorx.Ctx())
	want := gin.H{"code": "MIRROR-ERR-5", "msg": "MIRROR-ERR-5: credentials are incomplete"}

	t.Run("CreateMirrorRepo", func(t *testing.T) {
		tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
			return h.CreateMirrorRepo
		})
		tester.WithUser().WithBody(t, &types.CreateMirrorRepoReq{
			SourceNamespace:   "source-ns",
			SourceName:        "source-name",
			RepoType:          types.ModelRepo,
			SourceGitCloneUrl: "https://example.com/source/repo.git",
			ForkNamespace:     "target-ns",
			ForkName:          "target-name",
		})
		tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.Anything).Return(nil, authErr)

		tester.Execute()

		tester.ResponseEqSimple(t, 400, want)
	})

	t.Run("BatchCreate", func(t *testing.T) {
		tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
			return h.BatchCreate
		})
		tester.WithBody(t, &types.BatchCreateMirrorReq{})
		tester.mocks.mirror.EXPECT().BatchCreate(tester.Ctx(), mock.Anything).Return(authErr)

		tester.Execute()

		tester.ResponseEqSimple(t, 400, want)
	})
}

func TestMirrorHandler_CreateMirrorRepoBadRequest(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser().WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "source-ns",
		SourceName:        "source-name",
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "invalid-url",
		ForkNamespace:     "target-ns",
		ForkName:          "target-name",
	})
	badRequestErr := errorx.BadRequest(errors.New("invalid source git clone url"), errorx.Ctx())
	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.Anything).Return(nil, badRequestErr)

	tester.Execute()

	tester.ResponseEqSimple(t, 400, gin.H{
		"code": "REQ-ERR-0",
		"msg":  "REQ-ERR-0: invalid source git clone url",
	})
}

// TestMirrorHandler_CreateMirrorRepoTargetNotFound verifies an existing-target request returns HTTP 404.
func TestMirrorHandler_CreateMirrorRepoTargetNotFound(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser().WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "source-ns",
		SourceName:        "source-name",
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "https://example.com/source/repo.git",
		ForkNamespace:     "target-ns",
		ForkName:          "target-name",
	})
	notFoundErr := errorx.RepoNotFound(errors.New("target repository does not exist"), errorx.Ctx())
	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.Anything).Return(nil, notFoundErr)

	tester.Execute()

	tester.ResponseEqSimple(t, 404, gin.H{
		"code": "REPO-ERR-3",
		"msg":  "REPO-ERR-3: target repository does not exist",
	})
}

// TestMirrorHandler_BatchMirrorSourceBadRequest verifies malformed batch source URLs return a bad request.
func TestMirrorHandler_BatchMirrorSourceBadRequest(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.BatchCreate
	})
	tester.WithBody(t, &types.BatchCreateMirrorReq{})
	badRequestErr := errorx.BadRequest(errors.New("invalid source git clone url"), errorx.Ctx())
	tester.mocks.mirror.EXPECT().BatchCreate(tester.Ctx(), mock.Anything).Return(badRequestErr)

	tester.Execute()

	tester.ResponseEqSimple(t, 400, gin.H{
		"code": "REQ-ERR-0",
		"msg":  "REQ-ERR-0: invalid source git clone url",
	})
}

func TestMirrorHandler_Repos(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Repos
	})
	tester.WithUser()

	tester.mocks.mirror.EXPECT().Repos(tester.Ctx(), 10, 1).Return(
		[]types.MirrorRepo{{Path: "p", TaskID: 123}}, 100, nil,
	)
	tester.AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MirrorRepo{{Path: "p", TaskID: 123}},
		"total": 100,
	})
}

// TestMirrorHandler_Index verifies public filters map to the unified mirror sync list query.
func TestMirrorHandler_Index(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantStatus types.MirrorSyncOverallStatus
	}{
		{name: "empty status"},
		{name: "all status", status: "all"},
		{name: "waiting status", status: "waiting", wantStatus: types.MirrorSyncOverallWaiting},
		{name: "running status", status: "running", wantStatus: types.MirrorSyncOverallRunning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
				return h.Index
			})
			tester.WithUser()
			result := &types.MirrorSyncListResponse{
				Items: []types.MirrorSyncSummary{{SourceURL: "p"}}, Total: 100, Page: 1, Per: 10,
			}
			tester.mocks.mirror.EXPECT().ListMirrorSyncs(tester.Ctx(), types.MirrorSyncListReq{
				Page: 1, Per: 10, Search: "foo", Status: tt.wantStatus,
			}).Return(result, nil)
			tester.AddPagination(1, 10).WithQuery("search", "foo")
			if tt.status != "" {
				tester.WithQuery("status", tt.status)
			}
			tester.Execute()

			tester.ResponseEq(t, 200, tester.OKText, gin.H{
				"data":  result.Items,
				"total": result.Total,
			})
		})
	}
}

// TestMirrorHandler_IndexRejectsInvalidStatus verifies unsupported public status filters fail before component access.
func TestMirrorHandler_IndexRejectsInvalidStatus(t *testing.T) {
	for _, status := range []string{"finished", "no_task"} {
		t.Run(status, func(t *testing.T) {
			tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
				return h.Index
			})
			tester.WithUser().WithQuery("status", status).Execute()

			tester.ResponseEq(t, 400, "status must be one of all, waiting, or running", nil)
		})
	}
}

func TestMirrorHandler_Statistics(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Statistics
	})
	tester.WithUser()

	tester.mocks.mirror.EXPECT().Statistics(tester.Ctx()).Return(
		[]types.MirrorStatusCount{{Count: 5}}, nil,
	)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, []types.MirrorStatusCount{{Count: 5}})
}
