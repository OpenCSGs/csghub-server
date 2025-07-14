package handler

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type ModelTester struct {
	*testutil.GinTester
	handler *ModelHandler
	mocks   struct {
		model     *mockcomponent.MockModelComponent
		sensitive *mockcomponent.MockSensitiveComponent
		repo      *mockcomponent.MockRepoComponent
	}
}

func NewModelTester(t *testing.T) *ModelTester {
	tester := &ModelTester{GinTester: testutil.NewGinTester()}
	tester.mocks.model = mockcomponent.NewMockModelComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)

	tester.handler = &ModelHandler{
		model: tester.mocks.model, sensitive: tester.mocks.sensitive,
		repo: tester.mocks.repo,
	}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (t *ModelTester) WithHandleFunc(fn func(h *ModelHandler) gin.HandlerFunc) *ModelTester {
	t.Handler(fn(t.handler))
	return t

}

func TestModelHandler_Index(t *testing.T) {
	cases := []struct {
		sort   string
		source string
		error  bool
	}{
		{"most_download", "local", false},
		{"foo", "local", true},
		{"most_download", "bar", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
				return h.Index
			})

			if !c.error {
				tester.mocks.model.EXPECT().Index(tester.Ctx(), &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1, false).Return([]*types.Model{
					{Name: "cc"},
				}, 100, nil)
			}

			tester.AddPagination(1, 10).WithQuery("search", "foo").
				WithQuery("sort", c.sort).
				WithQuery("source", c.source).Execute()

			if c.error {
				require.Equal(t, 400, tester.Response().Code)
			} else {
				tester.ResponseEqSimple(t, 200, gin.H{
					"data":  []*types.Model{{Name: "cc"}},
					"total": 100,
					"msg":   "OK",
				})
			}
		})
	}
}

func TestModelHandler_Create(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	req := &types.CreateModelReq{CreateRepoReq: types.CreateRepoReq{Username: "u"}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.model.EXPECT().Create(tester.Ctx(), req).Return(&types.Model{Name: "m"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Model{Name: "m"})
}

func TestModelHandler_Update(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()

	req := &types.UpdateModelReq{UpdateRepoReq: types.UpdateRepoReq{}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.model.EXPECT().Update(tester.Ctx(), &types.UpdateModelReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Namespace: "u",
			Name:      "r",
			Username:  "u",
		},
	}).Return(&types.Model{Name: "m"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Model{Name: "m"})
}

func TestModelHandler_Delete(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.WithUser()

	tester.mocks.model.EXPECT().Delete(tester.Ctx(), "u", "r", "u").Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_Show(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.Show
	})

	tester.WithUser()
	tester.mocks.model.EXPECT().Show(tester.Ctx(), "u", "r", "u", false).Return(&types.Model{
		Name: "m",
	}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Model{Name: "m"})
}

func TestModelHandler_SDKModelInfo(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.SDKModelInfo
	})

	tester.WithUser()
	tester.mocks.model.EXPECT().SDKModelInfo(tester.Ctx(), "u", "r", "main", "u", false).Return(&types.SDKModelInfo{
		ID: "m",
	}, nil)
	tester.WithParam("ref", "main").Execute()

	tester.ResponseEqSimple(t, 200, &types.SDKModelInfo{ID: "m"})
}

func TestModelHandler_Relations(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.Relations
	})

	tester.WithUser()
	tester.mocks.model.EXPECT().Relations(tester.Ctx(), "u", "r", "u").Return(&types.Relations{
		Models: []*types.Model{{Name: "m1"}},
	}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Relations{
		Models: []*types.Model{{Name: "m1"}},
	})
}

func TestModelHandler_SetRelations(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.SetRelations
	})
	tester.WithUser()

	req := &types.RelationDatasets{
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
	}
	tester.mocks.model.EXPECT().SetRelationDatasets(tester.Ctx(), *req).Return(nil)
	tester.WithBody(t, &types.RelationDatasets{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_AddDatasetRelation(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.AddDatasetRelation
	})
	tester.WithUser()

	req := &types.RelationDataset{
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
	}
	tester.mocks.model.EXPECT().AddRelationDataset(tester.Ctx(), *req).Return(nil)
	tester.WithBody(t, &types.RelationDataset{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_DelDatasetRelation(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.DelDatasetRelation
	})
	tester.WithUser()

	req := &types.RelationDataset{
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
	}
	tester.mocks.model.EXPECT().DelRelationDataset(tester.Ctx(), *req).Return(nil)
	tester.WithBody(t, &types.RelationDataset{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_DeployDedicated(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.DeployDedicated
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.ModelRunReq{
			DeployName: "test",
			MinReplica: 1,
			MaxReplica: 2,
			Revision:   "main",
		}).Return(true, nil)
		tester.mocks.model.EXPECT().Deploy(tester.Ctx(), types.DeployActReq{
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployType:  types.InferenceType,
		}, types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 2, Revision: "main"}).Return(123, nil)

		tester.WithBody(t, &types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEq(t, 200, tester.OKText, types.DeployRepo{DeployID: 123})
	})
	t.Run("error_badrequest", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.DeployDedicated
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowReadAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.ModelRunReq{
			MinReplica: 1,
			MaxReplica: 2,
			Revision:   "main",
		}).Return(true, nil)

		tester.WithBody(t, &types.ModelRunReq{MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{
			Msg:     errorx.ErrBadRequest.Error(),
			Context: errorx.Ctx().Set("detail", "Length must be between 2 and 64 characters.").Set("name", ""),
		})
	})

}

func TestModelHandler_FinetuneCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.FinetuneCreate
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowAdminAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.mocks.model.EXPECT().Deploy(tester.Ctx(), types.DeployActReq{
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployType:  types.FinetuneType,
		}, types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 1, Revision: "main", SecureLevel: 2}).Return(123, nil)

		tester.WithBody(t, &types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEq(t, 200, tester.OKText, types.DeployRepo{DeployID: 123})
	})
	t.Run("error_badrequest", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.FinetuneCreate
		})
		tester.WithUser()

		tester.mocks.repo.EXPECT().AllowAdminAccess(tester.Ctx(), types.ModelRepo, "u", "r", "u").Return(true, nil)
		tester.WithBody(t, &types.ModelRunReq{MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{
			Msg:     errorx.ErrBadRequest.Error(),
			Context: errorx.Ctx().Set("detail", "Length must be between 2 and 64 characters.").Set("name", ""),
		})
	})

}

func TestModelHandler_DeployDelete(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.DeployDelete
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeleteDeploy(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.InferenceType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_FinetuneDelete(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.FinetuneDelete
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeleteDeploy(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.FinetuneType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_DeployStop(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.DeployStop
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStop(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.InferenceType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_DeployStart(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.DeployStart
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStart(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.InferenceType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_FinetuneStop(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.FinetuneStop
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStop(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.FinetuneType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_FinetuneStart(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.FinetuneStart
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStart(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.FinetuneType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_ListByRuntimeFrameworkID(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.ListByRuntimeFrameworkID
	})
	tester.WithUser()

	tester.WithQuery("deploy_type", "").AddPagination(1, 10).WithParam("id", "1")
	tester.mocks.model.EXPECT().ListModelsByRuntimeFrameworkID(
		tester.Ctx(), "u", 10, 1, int64(1), types.InferenceType,
	).Return([]types.Model{{Name: "foo"}}, 100, nil)
	tester.Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"data":  []types.Model{{Name: "foo"}},
		"total": 100,
	})
}

func TestModelHandler_ListAllRuntimeFramework(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.ListAllRuntimeFramework
	})
	tester.WithUser()

	tester.mocks.model.EXPECT().ListAllByRuntimeFramework(
		tester.Ctx(), "u",
	).Return([]database.RuntimeFramework{{FrameName: "foo"}}, nil)
	tester.Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"data": []database.RuntimeFramework{{FrameName: "foo"}},
	})
}

func TestModelHandler_UpdateModelRuntimeFramework(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.UpdateModelRuntimeFrameworks
	})

	tester.WithUser().WithQuery("deploy_type", "").AddPagination(1, 10).WithParam("id", "1")
	tester.mocks.model.EXPECT().SetRuntimeFrameworkModes(
		tester.Ctx(), types.InferenceType, int64(1), []string{"foo"},
	).Return([]string{"bar"}, nil)
	tester.WithBody(t, types.RuntimeFrameworkModels{
		Models: []string{"foo"},
	}).Execute()
	tester.ResponseEq(t, 200, tester.OKText, []string{"bar"})
}

func TestModelHandler_DeleteModelRuntimeFramework(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.DeleteModelRuntimeFrameworks
	})

	tester.WithUser().WithQuery("deploy_type", "").AddPagination(1, 10).WithParam("id", "1")
	tester.mocks.model.EXPECT().DeleteRuntimeFrameworkModes(
		tester.Ctx(), types.InferenceType, int64(1), []string{"foo"},
	).Return([]string{"bar"}, nil)
	tester.WithBody(t, types.RuntimeFrameworkModels{
		Models: []string{"foo"},
	}).Execute()
	tester.ResponseEq(t, 200, tester.OKText, []string{"bar"})
}

func TestModelHandler_ListModelsOfRuntimeFrameworks(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.ListModelsOfRuntimeFrameworks
	})
	tester.WithUser()

	tester.WithQuery("deploy_type", "").AddPagination(1, 10).WithParam("id", "1")
	tester.mocks.model.EXPECT().ListModelsOfRuntimeFrameworks(
		tester.Ctx(), "u", "foo", "most_downloads", 10, 1, types.InferenceType,
	).Return([]types.Model{{Name: "foo"}}, 100, nil)
	tester.WithQuery("search", "foo").WithQuery("sort", "most_downloads").Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data":  []types.Model{{Name: "foo"}},
		"total": 100,
	})
}

func TestModelHandler_DeployServerless(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.DeployServerless
		})
		tester.WithUser()

		tester.mocks.model.EXPECT().Deploy(tester.Ctx(), types.DeployActReq{
			Namespace:   "u",
			Name:        "r",
			CurrentUser: "u",
			DeployType:  types.ServerlessType,
		}, types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 2, Revision: "main", SecureLevel: 1}).Return(123, nil)

		tester.WithBody(t, &types.ModelRunReq{DeployName: "test", MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEq(t, 200, tester.OKText, types.DeployRepo{DeployID: 123})
	})

	t.Run("error_badrequest", func(t *testing.T) {
		tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
			return h.DeployServerless
		})
		tester.WithUser()
		tester.WithBody(t, &types.ModelRunReq{MinReplica: 1, MaxReplica: 2, Revision: "main"}).Execute()

		tester.ResponseEqSimple(t, 400, gin.H{
			"msg":     errorx.ErrBadRequest.Error(),
			"context": errorx.Ctx().Set("detail", "Length must be between 2 and 64 characters.").Set("name", ""),
		})
	})
}

func TestModelHandler_ServerlessStop(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.ServerlessStop
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStop(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.ServerlessType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_ServerlessStart(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.ServerlessStart
	})
	tester.WithUser()

	tester.mocks.repo.EXPECT().DeployStart(tester.Ctx(), types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "u",
		Name:        "r",
		CurrentUser: "u",
		DeployID:    1,
		DeployType:  types.ServerlessType,
	}).Return(nil)

	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestModelHandler_GetDeployServerless(t *testing.T) {
	tester := NewModelTester(t).WithHandleFunc(func(h *ModelHandler) gin.HandlerFunc {
		return h.GetDeployServerless
	})

	tester.mocks.model.EXPECT().GetServerless(tester.Ctx(), "u", "r", "u").Return(&types.DeployRepo{
		DeployID: 1,
	}, nil)
	tester.WithUser().Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.DeployRepo{DeployID: 1})
}
