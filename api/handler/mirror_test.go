package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
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
	tester.RequireUser(t)

	tester.mocks.mirror.EXPECT().CreateMirrorRepo(tester.Ctx(), types.CreateMirrorRepoReq{
		SourceNamespace:   "ns",
		SourceName:        "sn",
		CurrentUser:       "u",
		MirrorSourceID:    1,
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "url",
	}).Return(&database.Mirror{}, nil)
	tester.WithBody(t, &types.CreateMirrorRepoReq{
		SourceNamespace:   "ns",
		SourceName:        "sn",
		MirrorSourceID:    1,
		RepoType:          types.ModelRepo,
		SourceGitCloneUrl: "url",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestMirrorHandler_Repos(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Repos
	})
	tester.RequireUser(t)

	tester.mocks.mirror.EXPECT().Repos(tester.Ctx(), "u", 10, 1).Return(
		[]types.MirrorRepo{{Path: "p"}}, 100, nil,
	)
	tester.AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MirrorRepo{{Path: "p"}},
		"total": 100,
	})
}

func TestMirrorHandler_Index(t *testing.T) {
	tester := NewMirrorTester(t).WithHandleFunc(func(h *MirrorHandler) gin.HandlerFunc {
		return h.Index
	})
	tester.RequireUser(t)

	tester.mocks.mirror.EXPECT().Index(tester.Ctx(), "u", 10, 1, "foo").Return(
		[]types.Mirror{{SourceUrl: "p"}}, 100, nil,
	)
	tester.AddPagination(1, 10).WithQuery("search", "foo").Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.Mirror{{SourceUrl: "p"}},
		"total": 100,
	})
}
