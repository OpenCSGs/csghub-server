package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type OrganizationTester struct {
	*testutil.GinTester
	handler *OrganizationHandler
	mocks   struct {
		space      *mockcomponent.MockSpaceComponent
		code       *mockcomponent.MockCodeComponent
		model      *mockcomponent.MockModelComponent
		dataset    *mockcomponent.MockDatasetComponent
		collection *mockcomponent.MockCollectionComponent
		prompt     *mockcomponent.MockPromptComponent
		mcp        *mockcomponent.MockMCPServerComponent
	}
}

func NewOrganizationTester(t *testing.T) *OrganizationTester {
	tester := &OrganizationTester{GinTester: testutil.NewGinTester()}
	tester.mocks.space = mockcomponent.NewMockSpaceComponent(t)
	tester.mocks.code = mockcomponent.NewMockCodeComponent(t)
	tester.mocks.model = mockcomponent.NewMockModelComponent(t)
	tester.mocks.dataset = mockcomponent.NewMockDatasetComponent(t)
	tester.mocks.collection = mockcomponent.NewMockCollectionComponent(t)
	tester.mocks.prompt = mockcomponent.NewMockPromptComponent(t)
	tester.mocks.mcp = mockcomponent.NewMockMCPServerComponent(t)

	tester.handler = &OrganizationHandler{
		space:      tester.mocks.space,
		code:       tester.mocks.code,
		model:      tester.mocks.model,
		dataset:    tester.mocks.dataset,
		collection: tester.mocks.collection,
		prompt:     tester.mocks.prompt,
		mcp:        tester.mocks.mcp,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "n")
	return tester
}

func (t *OrganizationTester) WithHandleFunc(fn func(h *OrganizationHandler) gin.HandlerFunc) *OrganizationTester {
	t.Handler(fn(t.handler))
	return t
}

func TestOrganizationHandler_Models(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Models
	})

	tester.mocks.model.EXPECT().OrgModels(tester.Ctx(), &types.OrgModelsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Model{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Model{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_Datasets(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Datasets
	})

	tester.mocks.dataset.EXPECT().OrgDatasets(tester.Ctx(), &types.OrgDatasetsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Dataset{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Dataset{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_Codes(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Codes
	})

	tester.mocks.code.EXPECT().OrgCodes(tester.Ctx(), &types.OrgCodesReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Code{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Code{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_Spaces(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Spaces
	})

	tester.mocks.space.EXPECT().OrgSpaces(tester.Ctx(), &types.OrgSpacesReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Space{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Space{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_Collections(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Collections
	})

	tester.mocks.collection.EXPECT().OrgCollections(tester.Ctx(), &types.OrgCollectionsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Collection{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Collection{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_Prompts(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Prompts
	})

	tester.mocks.prompt.EXPECT().OrgPrompts(tester.Ctx(), &types.OrgPromptsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.PromptRes{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.PromptRes{{Name: "m"}},
		"total":   100,
	})
}

func TestOrganizationHandler_MCPServers(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.MCPServers
	})

	tester.mocks.mcp.EXPECT().OrgMCPServers(tester.Ctx(), &types.OrgMCPsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.MCPServer{{Name: "m"}}, 100, nil)

	tester.WithUser().AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MCPServer{{Name: "m"}},
		"total": 100,
	})
}
