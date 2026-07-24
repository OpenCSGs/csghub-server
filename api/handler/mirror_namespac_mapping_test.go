package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type MirrorNamespaceMappingTester struct {
	*testutil.GinTester
	handler *MirrorNamespaceMappingHandler
	mocks   struct {
		mirrorNamespaceMapping *mockcomponent.MockMirrorNamespaceMappingComponent
	}
}

func NewMirrorNamespaceMappingTester(t *testing.T) *MirrorNamespaceMappingTester {
	tester := &MirrorNamespaceMappingTester{GinTester: testutil.NewGinTester()}
	tester.mocks.mirrorNamespaceMapping = mockcomponent.NewMockMirrorNamespaceMappingComponent(t)

	tester.handler = &MirrorNamespaceMappingHandler{
		mirrorNamespaceMapping: tester.mocks.mirrorNamespaceMapping,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *MirrorNamespaceMappingTester) WithHandleFunc(fn func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc) *MirrorNamespaceMappingTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMirrorNamespaceMappingHandler_Create(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	tester.mocks.mirrorNamespaceMapping.EXPECT().Create(tester.Ctx(), types.CreateMirrorNamespaceMappingReq{
		SourceNamespace: "SN",
		TargetNamespace: "U",
	}).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)
	tester.WithBody(t, &types.CreateMirrorNamespaceMappingReq{SourceNamespace: "SN", TargetNamespace: "U"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorNamespaceMapping{ID: 1})
}

func TestMirrorNamespaceMappingHandler_CreateConflict(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()
	req := types.CreateMirrorNamespaceMappingReq{
		SourceNamespace: "SourceTeam",
		TargetNamespace: "TargetTeam",
	}
	err := errorx.SourceNamespaceMappingExists(
		errors.New("source namespace mapping exists"),
		errorx.Ctx().Set("source_namespace", req.SourceNamespace),
	)
	tester.mocks.mirrorNamespaceMapping.EXPECT().Create(tester.Ctx(), req).Return(nil, err)

	tester.WithBody(t, &req).Execute()

	tester.ResponseEqCode(t, http.StatusConflict)
}

func TestMirrorNamespaceMappingHandler_Index(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Index
	})
	tester.WithUser()

	tester.mocks.mirrorNamespaceMapping.EXPECT().Index(tester.Ctx()).Return([]database.MirrorNamespaceMapping{{ID: 1}}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.MirrorNamespaceMapping{{ID: 1}})
}

func TestMirrorNamespaceMappingHandler_Update(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	var (
		sn = "sn"
		n  = "u"
	)

	tester.mocks.mirrorNamespaceMapping.EXPECT().Update(tester.Ctx(), types.UpdateMirrorNamespaceMappingReq{
		ID:              1,
		TargetNamespace: &n,
		SourceNamespace: &sn,
	}).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)
	tester.WithBody(t, &types.UpdateMirrorNamespaceMappingReq{
		SourceNamespace: &sn,
		TargetNamespace: &n,
	}).WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorNamespaceMapping{ID: 1})
}

func TestMirrorNamespaceMappingHandler_UpdateNotFound(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	sourceNamespace := "MissingSource"
	req := types.UpdateMirrorNamespaceMappingReq{ID: 1, SourceNamespace: &sourceNamespace}
	err := errorx.SourceNamespaceMappingNotFound(
		errors.New("source namespace mapping does not exist"),
		errorx.Ctx().Set("id", int64(1)).Set("source_namespace", sourceNamespace),
	)
	tester.mocks.mirrorNamespaceMapping.EXPECT().Update(tester.Ctx(), req).Return(nil, err)

	tester.WithBody(t, &types.UpdateMirrorNamespaceMappingReq{
		SourceNamespace: &sourceNamespace,
	}).WithParam("id", "1").Execute()

	tester.ResponseEqCode(t, http.StatusNotFound)
}

func TestMirrorNamespaceMappingHandler_Get(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Get
	})
	tester.WithUser()

	tester.mocks.mirrorNamespaceMapping.EXPECT().Get(tester.Ctx(), int64(1)).Return(&database.MirrorNamespaceMapping{ID: 1}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorNamespaceMapping{ID: 1})
}

func TestMirrorNamespaceMappingHandler_Delete(t *testing.T) {
	tester := NewMirrorNamespaceMappingTester(t).WithHandleFunc(func(h *MirrorNamespaceMappingHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.WithUser()

	tester.mocks.mirrorNamespaceMapping.EXPECT().Delete(tester.Ctx(), int64(1)).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
