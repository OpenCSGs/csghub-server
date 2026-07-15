package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
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
			req.RejectExistingRepo
	})).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:    "ns",
		SourceName:         "sn",
		MirrorSourceID:     1,
		RepoType:           types.ModelRepo,
		SourceGitCloneUrl:  "url",
		ForkNamespace:      "target-ns",
		ForkName:           "target-name",
		Private:            &private,
		RejectExistingRepo: true,
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

func TestMirrorHandler_Repos(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Repos
	})
	tester.WithUser()

	tester.mocks.mirror.EXPECT().Repos(tester.Ctx(), 10, 1).Return(
		[]types.MirrorRepo{{Path: "p"}}, 100, nil,
	)
	tester.AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MirrorRepo{{Path: "p"}},
		"total": 100,
	})
}

// TestMirrorHandler_Index verifies mirror list filters are forwarded to the component.
func TestMirrorHandler_Index(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Index
	})
	tester.WithUser()
	status := types.MirrorRepoSyncFailed
	filter := types.MirrorFilter{Search: "foo", Status: &status}

	tester.mocks.mirror.EXPECT().Index(tester.Ctx(), 10, 1, filter).Return(
		[]types.Mirror{{SourceUrl: "p"}}, 100, nil,
	)
	tester.AddPagination(1, 10).
		WithQuery("search", "foo").
		WithQuery("status", string(status)).
		Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.Mirror{{SourceUrl: "p"}},
		"total": 100,
	})
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
