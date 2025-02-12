package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type MirrorSourceTester struct {
	*testutil.GinTester
	handler *MirrorSourceHandler
	mocks   struct {
		mirrorSource *mockcomponent.MockMirrorSourceComponent
	}
}

func NewMirrorSourceTester(t *testing.T) *MirrorSourceTester {
	tester := &MirrorSourceTester{GinTester: testutil.NewGinTester()}
	tester.mocks.mirrorSource = mockcomponent.NewMockMirrorSourceComponent(t)

	tester.handler = &MirrorSourceHandler{
		mirrorSource: tester.mocks.mirrorSource,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *MirrorSourceTester) WithHandleFunc(fn func(h *MirrorSourceHandler) gin.HandlerFunc) *MirrorSourceTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMirrorSourceHandler_Create(t *testing.T) {
	tester := NewMirrorSourceTester(t).WithHandleFunc(func(h *MirrorSourceHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.RequireUser(t)

	tester.mocks.mirrorSource.EXPECT().Create(tester.Ctx(), types.CreateMirrorSourceReq{
		SourceName:  "sn",
		CurrentUser: "u",
	}).Return(&database.MirrorSource{ID: 1}, nil)
	tester.WithBody(t, &types.CreateMirrorSourceReq{SourceName: "sn"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorSource{ID: 1})
}

func TestMirrorSourceHandler_Index(t *testing.T) {
	tester := NewMirrorSourceTester(t).WithHandleFunc(func(h *MirrorSourceHandler) gin.HandlerFunc {
		return h.Index
	})
	tester.RequireUser(t)

	tester.mocks.mirrorSource.EXPECT().Index(tester.Ctx(), "u").Return([]database.MirrorSource{{ID: 1}}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.MirrorSource{{ID: 1}})
}

func TestMirrorSourceHandler_Update(t *testing.T) {
	tester := NewMirrorSourceTester(t).WithHandleFunc(func(h *MirrorSourceHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.RequireUser(t)

	tester.mocks.mirrorSource.EXPECT().Update(tester.Ctx(), types.UpdateMirrorSourceReq{
		ID:          1,
		CurrentUser: "u",
		SourceName:  "sn",
	}).Return(&database.MirrorSource{ID: 1}, nil)
	tester.WithBody(t, &types.UpdateMirrorSourceReq{
		SourceName: "sn",
	}).WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorSource{ID: 1})
}

func TestMirrorSourceHandler_Get(t *testing.T) {
	tester := NewMirrorSourceTester(t).WithHandleFunc(func(h *MirrorSourceHandler) gin.HandlerFunc {
		return h.Get
	})
	tester.RequireUser(t)

	tester.mocks.mirrorSource.EXPECT().Get(tester.Ctx(), int64(1), "u").Return(&database.MirrorSource{ID: 1}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.MirrorSource{ID: 1})
}

func TestMirrorSourceHandler_Delete(t *testing.T) {
	tester := NewMirrorSourceTester(t).WithHandleFunc(func(h *MirrorSourceHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.RequireUser(t)

	tester.mocks.mirrorSource.EXPECT().Delete(tester.Ctx(), int64(1), "u").Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
