package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type SpaceTemplateTester struct {
	*testutil.GinTester
	handler *SpaceTemplateHandler
	mocks   struct {
		spaceTemplate *mockcomponent.MockSpaceTemplateComponent
	}
}

func NewSpaceTemplateTester(t *testing.T) *SpaceTemplateTester {
	tester := &SpaceTemplateTester{GinTester: testutil.NewGinTester()}
	tester.mocks.spaceTemplate = mockcomponent.NewMockSpaceTemplateComponent(t)

	tester.handler = &SpaceTemplateHandler{
		c: tester.mocks.spaceTemplate,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *SpaceTemplateTester) WithHandleFunc(fn func(h *SpaceTemplateHandler) gin.HandlerFunc) *SpaceTemplateTester {
	t.Handler(fn(t.handler))
	return t
}

func TestSpaceTemplateHandler_Index(t *testing.T) {
	tester := NewSpaceTemplateTester(t).WithHandleFunc(func(h *SpaceTemplateHandler) gin.HandlerFunc {
		return h.Index
	})

	tester.mocks.spaceTemplate.EXPECT().Index(tester.Ctx()).Return(
		[]database.SpaceTemplate{{Name: "sp", Type: "docker"}}, nil,
	)
	tester.WithUser().Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.SpaceTemplate{{Name: "sp", Type: "docker"}})
}

func TestSpaceTemplateHandler_Create(t *testing.T) {
	tester := NewSpaceTemplateTester(t).WithHandleFunc(func(h *SpaceTemplateHandler) gin.HandlerFunc {
		return h.Create
	})

	tester.mocks.spaceTemplate.EXPECT().Create(tester.Ctx(), &types.SpaceTemplateReq{
		Name:     "sp",
		Type:     "docker",
		ShowName: "n1",
		Path:     "chatui",
	}).Return(
		&database.SpaceTemplate{Name: "sp", Type: "docker", ShowName: "n1",
			Path: "chatui"}, nil,
	)
	tester.WithBody(t, &types.SpaceTemplateReq{
		Name:     "sp",
		Type:     "docker",
		ShowName: "n1",
		Path:     "chatui",
	}).WithUser().Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.SpaceTemplate{
		Name: "sp", Type: "docker", ShowName: "n1", Path: "chatui",
	})
}

func TestSpaceTemplateHandler_Update(t *testing.T) {
	tester := NewSpaceTemplateTester(t).WithHandleFunc(func(h *SpaceTemplateHandler) gin.HandlerFunc {
		return h.Update
	})
	name := "n"
	typestr := "d"
	tester.mocks.spaceTemplate.EXPECT().Update(tester.Ctx(), &types.UpdateSpaceTemplateReq{
		Name: &name,
		Type: &typestr,
		ID:   1,
	}).Return(
		&database.SpaceTemplate{ID: 1, Name: "n", Type: "d"}, nil,
	)
	tester.WithBody(t, &types.UpdateSpaceTemplateReq{
		Name: &name,
		Type: &typestr,
	}).WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.SpaceTemplate{ID: 1, Name: "n", Type: "d"})
}

func TestSpaceTemplateHandler_Delete(t *testing.T) {
	tester := NewSpaceTemplateTester(t).WithHandleFunc(func(h *SpaceTemplateHandler) gin.HandlerFunc {
		return h.Delete
	})

	tester.mocks.spaceTemplate.EXPECT().Delete(tester.Ctx(), int64(1)).Return(
		nil,
	)
	tester.WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSpaceTemplateHandler_List(t *testing.T) {
	tester := NewSpaceTemplateTester(t).WithHandleFunc(func(h *SpaceTemplateHandler) gin.HandlerFunc {
		return h.List
	})
	tester.mocks.spaceTemplate.EXPECT().FindAllByType(tester.Ctx(), "docker").Return(
		[]database.SpaceTemplate{{Name: "sp", Type: "docker"}}, nil,
	)
	tester.WithUser().WithParam("type", "docker").Execute()
	tester.ResponseEq(t, 200, tester.OKText, []database.SpaceTemplate{{Name: "sp", Type: "docker"}})
}
