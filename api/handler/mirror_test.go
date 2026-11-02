package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
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
	autoCreateOrganization := true

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
			req.Priority == types.HighMirrorPriority &&
			req.AutoCreateOrganization != nil &&
			*req.AutoCreateOrganization &&
			req.AllowAutoCreateOrganization
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:        "ns",
		SourceName:             "sn",
		MirrorSourceID:         1,
		RepoType:               types.ModelRepo,
		SourceGitCloneUrl:      "url",
		ForkNamespace:          "target-ns",
		ForkName:               "target-name",
		Private:                &private,
		CreateTargetRepo:       &createTargetRepo,
		Priority:               types.HighMirrorPriority,
		AutoCreateOrganization: &autoCreateOrganization,
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

// TestMirrorHandler_CreateMirrorRepoAutoCreateOrganizationDisabled verifies the
// admin checkbox value flows through to the component-level flag, so an
// unchecked box does not grant organization auto-creation.
func TestMirrorHandler_CreateMirrorRepoAutoCreateOrganizationDisabled(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser()
	autoCreateOrganization := false

	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.MatchedBy(func(req types.CreateMirrorRepoReq) bool {
		return req.CurrentUser == "u" &&
			req.AutoCreateOrganization != nil &&
			!*req.AutoCreateOrganization &&
			!req.AllowAutoCreateOrganization
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:        "ns",
		SourceName:             "sn",
		RepoType:               types.ModelRepo,
		SourceGitCloneUrl:      "url",
		ForkNamespace:          "target-ns",
		ForkName:               "target-name",
		AutoCreateOrganization: &autoCreateOrganization,
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

// TestMirrorHandler_CreateMirrorRepoAutoCreateOrganizationDefaultsEnabled verifies
// older clients that omit the field retain the legacy auto-create behavior.
func TestMirrorHandler_CreateMirrorRepoAutoCreateOrganizationDefaultsEnabled(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser()

	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.MatchedBy(func(req types.CreateMirrorRepoReq) bool {
		return req.CurrentUser == "u" &&
			req.AutoCreateOrganization == nil &&
			req.Private != nil && !*req.Private &&
			req.AllowAutoCreateOrganization
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "ns",
		SourceName:        "sn",
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "url",
		ForkNamespace:     "target-ns",
		ForkName:          "target-name",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

// TestMirrorHandler_CreateMirrorRepoVisibilityDefaults checks public defaults and explicit visibility for every repository type.
func TestMirrorHandler_CreateMirrorRepoVisibilityDefaults(t *testing.T) {
	for _, repoType := range []types.RepositoryType{
		types.ModelRepo, types.DatasetRepo, types.CodeRepo, types.SpaceRepo,
		types.PromptRepo, types.MCPServerRepo, types.SkillRepo, types.TemplateRepo,
	} {
		for _, tc := range []struct {
			name    string
			private *bool
		}{
			{name: "omitted"},
			{name: "explicit_private", private: new(true)},
			{name: "explicit_public", private: new(false)},
		} {
			t.Run(string(repoType)+"/"+tc.name, func(t *testing.T) {
				tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
					return h.CreateMirrorRepo
				})
				tester.WithUser()
				tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.MatchedBy(func(req types.CreateMirrorRepoReq) bool {
					if req.RepoType != repoType {
						return false
					}
					wantPrivate := tc.private != nil && *tc.private
					return req.Private != nil && *req.Private == wantPrivate
				})).Return(&database.Mirror{}, nil)
				tester.WithBody(t, &types.CreateMirrorRepoReq{
					SourceNamespace: "ns", SourceName: "source", RepoType: repoType,
					SourceGitCloneUrl: "https://opencsg.com/ns/source.git",
					ForkNamespace:     "target-ns", ForkName: "target-name", Private: tc.private,
				}).Execute()
				tester.ResponseEq(t, 200, tester.OKText, nil)
			})
		}
	}
}

// TestMirrorHandler_CreateMirrorRepoIgnoresMCPServerAttributes verifies callers cannot inject internal MCP metadata through JSON.
func TestMirrorHandler_CreateMirrorRepoIgnoresMCPServerAttributes(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser()

	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.MatchedBy(func(req types.CreateMirrorRepoReq) bool {
		attributes := req.MCPServerAttributes
		return req.RepoType == types.MCPServerRepo &&
			attributes.StarCount == 0 &&
			len(attributes.Tools) == 0 &&
			attributes.AvatarURL == "" &&
			attributes.Configuration.Type == "" &&
			len(attributes.Configuration.Properties) == 0 &&
			len(attributes.Configuration.Required) == 0
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, map[string]any{
		"source_namespace": "ns",
		"source_name":      "server",
		"repo_type":        types.MCPServerRepo,
		"source_url":       "https://opencsg.com/ns/server.git",
		"fork_namespace":   "target-ns",
		"fork_name":        "server",
		"mcp_server_attributes": map[string]any{
			"star_count": 99,
			"avatar_url": "https://example.com/avatar.png",
		},
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
				"repo_type":        types.DatasetRepo,
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

}

// TestMirrorHandler_CreateMirrorRepoPrioritizesSourceURL verifies malformed paths are reported even when derived source fields are missing.
func TestMirrorHandler_CreateMirrorRepoPrioritizesSourceURL(t *testing.T) {
	for _, tc := range []struct {
		sourceURL string
		message   string
	}{
		{"https://xx.com/name.git", "source git clone url must include a namespace and repository"},
		{"https://xx.com/ns/.git", "source git clone url must not contain empty namespace or repository path segments"},
		{"https://xx.com/ns//repo.git", "source git clone url must not contain empty namespace or repository path segments"},
	} {
		t.Run(tc.sourceURL, func(t *testing.T) {
			tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
				return h.CreateMirrorRepo
			})
			tester.WithUser().WithBody(t, map[string]any{
				"source_url":     tc.sourceURL,
				"repo_type":      types.ModelRepo,
				"fork_namespace": "target-ns",
				"fork_name":      "target-name",
			}).Execute()
			tester.ResponseEqSimple(t, 400, gin.H{
				"code": "MIRROR-ERR-8",
				"msg":  "MIRROR-ERR-8: " + tc.message,
			})
		})
	}
}

// TestMirrorHandler_CreateMirrorRepoPreservesBindingErrors verifies URL validation does not bypass required fields or mask JSON decoding errors.
func TestMirrorHandler_CreateMirrorRepoPreservesBindingErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    map[string]any
		message string
	}{
		{
			name:    "valid URL with missing source fields",
			body:    map[string]any{"source_url": "https://xx.com/ns/repo.git", "repo_type": types.ModelRepo},
			message: "CreateMirrorRepoReq.SourceNamespace",
		},
		{
			name:    "missing URL",
			body:    map[string]any{"source_namespace": "ns", "source_name": "repo", "repo_type": types.ModelRepo},
			message: "CreateMirrorRepoReq.SourceGitCloneUrl",
		},
		{
			name:    "wrong JSON field type with invalid URL",
			body:    map[string]any{"source_url": "https://xx.com/name.git", "source_namespace": 123},
			message: "cannot unmarshal number",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
				return h.CreateMirrorRepo
			})
			tester.WithUser().WithBody(t, tc.body).Execute()
			tester.ResponseEqCode(t, 400)
			require.Contains(t, tester.Response().Body.String(), tc.message)
		})
	}
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

// TestMirrorHandler_CreateMirrorRepoUnsupportedMetadataSource verifies an unconfigured non-OpenCSG source returns HTTP 400.
func TestMirrorHandler_CreateMirrorRepoUnsupportedMetadataSource(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.CreateMirrorRepo
	})
	tester.WithUser().WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "upstream",
		SourceName:        "reviewer",
		RepoType:          types.SkillRepo,
		SourceGitCloneUrl: "https://github.com/upstream/reviewer.git",
		ForkNamespace:     "target-ns",
		ForkName:          "reviewer",
	})
	sourceURLErr := errorx.MirrorSourceURLInvalid(
		errors.New(`source host "github.com" is neither a configured mirror source nor OpenCSG SaaS`),
		errorx.Ctx(),
	)
	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), mock.Anything).Return(nil, sourceURLErr)

	tester.Execute()

	tester.ResponseEqSimple(t, 400, gin.H{
		"code": "MIRROR-ERR-8",
		"msg":  `MIRROR-ERR-8: source host "github.com" is neither a configured mirror source nor OpenCSG SaaS`,
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

// TestParseMirrorMetricsQuery validates filters without losing large database IDs.
func TestParseMirrorMetricsQuery(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		q, err := parseMirrorMetricsQuery(url.Values{})
		require.NoError(t, err)
		require.Equal(t, types.MirrorMetricsQuery{Service: "all", Page: 1, Per: 20, InstancePage: 1, InstancePer: 20}, q)
	})
	t.Run("explicit filters", func(t *testing.T) {
		values, err := url.ParseQuery("service=lfs&instance_id=f6d030fa-7ef1-49db-b31f-45318c74f903&task_id=9007199254740993&page=2&per=100&instance_page=3&instance_per=1")
		require.NoError(t, err)
		q, err := parseMirrorMetricsQuery(values)
		require.NoError(t, err)
		require.Equal(t, "9007199254740993", q.TaskID)
		require.Equal(t, "lfs", q.Service)
		require.Equal(t, 2, q.Page)
		require.Equal(t, 100, q.Per)
		require.Equal(t, 3, q.InstancePage)
		require.Equal(t, 1, q.InstancePer)
	})
	for _, raw := range []string{
		"current_user=admin&current_user_uuid=f6d030fa-7ef1-49db-b31f-45318c74f903",
		"include_history=true&since_ms=10&task_state=claimed",
		"extra=&extra=ignored", "unknown_parameter=value",
	} {
		t.Run("ignores "+raw, func(t *testing.T) {
			values, err := url.ParseQuery(raw)
			require.NoError(t, err)
			values.Set("service", "lfs")
			values.Set("page", "2")
			q, err := parseMirrorMetricsQuery(values)
			require.NoError(t, err)
			require.Equal(t, types.MirrorMetricsQuery{Service: "lfs", Page: 2, Per: 20, InstancePage: 1, InstancePer: 20}, q)
		})
	}
	for _, raw := range []string{
		"service=other", "service=repo&service=lfs", "service=", "page=1&page=2",
		"page=0&current_user=admin&extra=value",
		"instance_id=worker-1", "task_id=0", "task_id=01", "task_id=-1", "task_id=9223372036854775808",
		"page=0", "per=101", "instance_page=-1", "instance_per=0", "page=1.5",
	} {
		t.Run(raw, func(t *testing.T) {
			values, err := url.ParseQuery(raw)
			require.NoError(t, err)
			_, err = parseMirrorMetricsQuery(values)
			require.Error(t, err)
		})
	}
}

// TestMirrorMetricsHandlerResponses checks timeout propagation and safe failure responses.
func TestMirrorMetricsHandlerResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, query string
		readError   error
		wantStatus  int
		wantCalls   int
	}{
		{name: "snapshot", wantStatus: 200, wantCalls: 1},
		{name: "authentication and unrelated parameters", query: "?current_user=admin&current_user_uuid=user-id&extra=&extra=value", wantStatus: 200, wantCalls: 1},
		{name: "encoded unrelated values", query: "?extra=%25ZZ&extra=a%26b&current_user=admin", wantStatus: 200, wantCalls: 1},
		{name: "invalid value escape", query: "?per=%ZZ", wantStatus: 400},
		{name: "incomplete escape", query: "?per=%", wantStatus: 400},
		{name: "mixed duplicate with invalid escape", query: "?service=repo&service=%ZZ", wantStatus: 400},
		{name: "invalid parameter name escape", query: "?%ZZ=value", wantStatus: 400},
		{name: "invalid unrelated value escape", query: "?extra=%ZZ&service=lfs", wantStatus: 400},
		{name: "encoded duplicate parameter name", query: "?service=repo&%73ervice=lfs", wantStatus: 400},
		{name: "invalid query", query: "?per=101", wantStatus: 400},
		{name: "cache failure", readError: errors.New("redis://private:secret@host"), wantStatus: 503, wantCalls: 1},
		{name: "timeout", readError: context.DeadlineExceeded, wantStatus: 503, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			h := MirrorMetricsHandler{read: func(ctx context.Context, query types.MirrorMetricsQuery) (*types.MirrorMetricsResponse, error) {
				calls++
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
				return &types.MirrorMetricsResponse{SchemaVersion: 2, Status: "partial", ServerTimeMS: 1792022400000}, tc.readError
			}}
			engine := gin.New()
			engine.GET("/metrics", h.Get)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics"+tc.query, nil))
			require.Equal(t, tc.wantStatus, response.Code)
			require.Equal(t, tc.wantCalls, calls)
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
			require.NotContains(t, response.Body.String(), "secret")
			if tc.wantStatus == http.StatusOK {
				var body struct {
					Data types.MirrorMetricsResponse `json:"data"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, "partial", body.Data.Status)
				require.Equal(t, 2, body.Data.SchemaVersion)
			}
		})
	}
}

// TestMirrorMetricsHandlerInstanceUUIDForms matches equivalent UUID forms through the real snapshot filter.
func TestMirrorMetricsHandlerInstanceUUIDForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.Redis.Endpoint = server.Addr()
	cfg.Mirror.Metrics.Enabled = true
	cfg.Mirror.Metrics.Scope = "uuid-forms"
	store := cache.NewMirrorMetricsStoreFromConfig(cfg)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	ctx := context.Background()
	now := time.Now().UnixMilli()
	const id = "f6d030fa-7ef1-49db-b31f-45318c74f903"
	for i, instanceID := range []string{id, "9e1a1212-7488-4fe5-a271-21171970348c"} {
		require.NoError(t, store.PublishSnapshot(ctx, types.MirrorMetricsSnapshot{
			SchemaVersion: cache.MirrorMetricsSchemaVersion,
			Instance:      types.MirrorMetricsInstance{InstanceID: instanceID, Service: "lfs", RuntimeState: "ready", SampledAtMS: &now},
			Executions:    []types.MirrorMetricsTask{{TaskID: strconv.Itoa(i + 1)}},
		}))
	}
	require.NoError(t, store.PublishControl(ctx, &types.MirrorMetricsControl{SchemaVersion: cache.MirrorMetricsSchemaVersion, SampledAtMS: now}))
	engine := gin.New()
	engine.GET("/metrics", NewMirrorMetricsHandler(cfg).Get)
	for _, input := range []string{id, strings.ToUpper(id), strings.ReplaceAll(id, "-", ""), "urn:uuid:" + id, "{" + id + "}"} {
		t.Run(input, func(t *testing.T) {
			query := url.Values{"instance_id": {input}}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics?"+query.Encode(), nil))
			require.Equal(t, http.StatusOK, response.Code)
			var body struct {
				Data types.MirrorMetricsResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Equal(t, 1, body.Data.Instances.Total)
			require.Len(t, body.Data.Instances.Items, 1)
			require.Equal(t, id, body.Data.Instances.Items[0].InstanceID)
			require.Equal(t, 1, body.Data.Tasks.Total)
			require.Len(t, body.Data.Tasks.Items, 1)
			require.Equal(t, "1", body.Data.Tasks.Items[0].TaskID)
			require.Equal(t, id, *body.Data.Tasks.Items[0].InstanceID)
			require.Equal(t, 2, body.Data.Overview.OnlineInstances.LFS)
		})
	}
}
