package handler

import (
	"errors"
	"net/http"
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
		finetune   *mockcomponent.MockFinetuneComponent
		evaluation *mockcomponent.MockEvaluationComponent
		user       *mockcomponent.MockUserComponent
		skill      *mockcomponent.MockSkillComponent
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
	tester.mocks.finetune = mockcomponent.NewMockFinetuneComponent(t)
	tester.mocks.evaluation = mockcomponent.NewMockEvaluationComponent(t)
	tester.mocks.user = mockcomponent.NewMockUserComponent(t)
	tester.mocks.skill = mockcomponent.NewMockSkillComponent(t)

	tester.handler = &OrganizationHandler{
		space:      tester.mocks.space,
		code:       tester.mocks.code,
		model:      tester.mocks.model,
		dataset:    tester.mocks.dataset,
		collection: tester.mocks.collection,
		prompt:     tester.mocks.prompt,
		mcp:        tester.mocks.mcp,
		finetune:   tester.mocks.finetune,
		evaluation: tester.mocks.evaluation,
		user:       tester.mocks.user,
		skill:      tester.mocks.skill,
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

func TestOrganizationHandler_Finetunes(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Finetunes
	})

	tester.mocks.finetune.EXPECT().OrgFinetunes(tester.Ctx(), &types.OrgFinetunesReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.ArgoWorkFlowRes{{TaskName: "ft1"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.ArgoWorkFlowRes{{TaskName: "ft1"}},
		"total": 100,
	})
}

func TestOrganizationHandler_Evaluations(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Evaluations
	})

	tester.mocks.evaluation.EXPECT().OrgEvaluations(tester.Ctx(), &types.OrgEvaluationsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.ArgoWorkFlowRes{{TaskName: "ev1"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.ArgoWorkFlowRes{{TaskName: "ev1"}},
		"total": 100,
	})
}

func TestOrganizationHandler_RunDeploys(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "model")

	tester.mocks.user.EXPECT().ListDeploysByNamespace(tester.Ctx(), &types.OrgRunDeploysReq{
		Namespace:   "u",
		CurrentUser: "u",
		RepoType:    types.ModelRepo,
		DeployType:  types.InferenceType,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.DeployRequest{{DeployName: "d1"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.DeployRequest{{DeployName: "d1"}},
		"total": 100,
	})
}

func TestOrganizationHandler_Notebooks(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Notebooks
	})

	tester.mocks.user.EXPECT().ListNotebooksByNamespace(tester.Ctx(), &types.OrgNotebooksReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.NotebookRes{{DeployName: "nb1"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.NotebookRes{{DeployName: "nb1"}},
		"total": 100,
	})
}

func TestOrganizationHandler_Skills(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Skills
	})

	tester.mocks.skill.EXPECT().OrgSkills(tester.Ctx(), &types.OrgSkillsReq{
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
		Namespace:   "u",
		CurrentUser: "u",
	}).Return([]types.Skill{{Name: "m"}}, 100, nil)
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Skill{{Name: "m"}},
		"total":   100,
	})
}

// --- Error and edge-case tests for coverage ---

func TestOrganizationHandler_Models_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Models
	})
	tester.WithUser().WithQuery("per", "invalid").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Models_BadPage(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Models
	})
	tester.WithUser().WithQuery("per", "10").WithQuery("page", "0").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Datasets_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Datasets
	})
	tester.WithUser().WithQuery("per", "x").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Spaces_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Spaces
	})
	tester.WithUser().WithQuery("per", "0").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Codes_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Codes
	})
	tester.WithUser().WithQuery("per", "101").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Collections_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Collections
	})
	tester.WithUser().WithQuery("per", "invalid").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Prompts_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Prompts
	})
	tester.WithUser().WithQuery("per", "-1").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_MCPServers_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.MCPServers
	})
	tester.WithUser().WithQuery("per", "x").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Finetunes_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Finetunes
	})
	tester.WithUser().WithQuery("per", "0").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Evaluations_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Evaluations
	})
	tester.WithUser().WithQuery("per", "invalid").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Notebooks_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Notebooks
	})
	tester.WithUser().WithQuery("per", "x").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_RunDeploys_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "model")
	tester.WithUser().WithQuery("deploy_type", "1").WithQuery("per", "invalid").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Codes_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Codes
	})
	tester.mocks.code.EXPECT().OrgCodes(tester.Ctx(), &types.OrgCodesReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Spaces_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Spaces
	})
	tester.mocks.space.EXPECT().OrgSpaces(tester.Ctx(), &types.OrgSpacesReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Collections_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Collections
	})
	tester.mocks.collection.EXPECT().OrgCollections(tester.Ctx(), &types.OrgCollectionsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Prompts_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Prompts
	})
	tester.mocks.prompt.EXPECT().OrgPrompts(tester.Ctx(), &types.OrgPromptsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_MCPServers_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.MCPServers
	})
	tester.mocks.mcp.EXPECT().OrgMCPServers(tester.Ctx(), &types.OrgMCPsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Models_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Models
	})
	tester.mocks.model.EXPECT().OrgModels(tester.Ctx(), &types.OrgModelsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Datasets_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Datasets
	})
	tester.mocks.dataset.EXPECT().OrgDatasets(tester.Ctx(), &types.OrgDatasetsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Finetunes_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Finetunes
	})
	tester.mocks.finetune.EXPECT().OrgFinetunes(tester.Ctx(), &types.OrgFinetunesReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("backend error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Evaluations_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Evaluations
	})
	tester.mocks.evaluation.EXPECT().OrgEvaluations(tester.Ctx(), &types.OrgEvaluationsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("backend error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Notebooks_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Notebooks
	})
	tester.mocks.user.EXPECT().ListNotebooksByNamespace(tester.Ctx(), &types.OrgNotebooksReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("backend error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_RunDeploys_InvalidDeployType(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "model")
	tester.WithUser().WithQuery("deploy_type", "invalid").WithQuery("per", "10").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_RunDeploys_InvalidRepoType(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "dataset")
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_RunDeploys_RepoTypeSpace(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "space")
	tester.mocks.user.EXPECT().ListDeploysByNamespace(tester.Ctx(), &types.OrgRunDeploysReq{
		Namespace: "u", CurrentUser: "u",
		RepoType: types.SpaceRepo, DeployType: types.SpaceType,
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
	}).Return([]types.DeployRequest{{DeployName: "s1"}}, 1, nil)
	tester.WithUser().WithQuery("deploy_type", "0").AddPagination(1, 10).Execute()
	tester.ResponseEq(t, 200, tester.OKText, gin.H{"data": []types.DeployRequest{{DeployName: "s1"}}, "total": 1})
}

func TestOrganizationHandler_RunDeploys_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.RunDeploys
	})
	tester.WithParam("repo_type", "model")
	tester.mocks.user.EXPECT().ListDeploysByNamespace(tester.Ctx(), &types.OrgRunDeploysReq{
		Namespace: "u", CurrentUser: "u",
		RepoType: types.ModelRepo, DeployType: types.InferenceType,
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
	}).Return(nil, 0, errors.New("backend error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}

func TestOrganizationHandler_Skills_BadPagination(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Skills
	})
	tester.WithUser().WithQuery("per", "invalid").WithQuery("page", "1").Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestOrganizationHandler_Skills_ComponentError(t *testing.T) {
	tester := NewOrganizationTester(t).WithHandleFunc(func(h *OrganizationHandler) gin.HandlerFunc {
		return h.Skills
	})
	tester.mocks.skill.EXPECT().OrgSkills(tester.Ctx(), &types.OrgSkillsReq{
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
		Namespace: "u", CurrentUser: "u",
	}).Return(nil, 0, errors.New("db error"))
	tester.WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqCode(t, http.StatusInternalServerError)
}
