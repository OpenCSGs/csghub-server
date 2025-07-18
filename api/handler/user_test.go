package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type UserTester struct {
	*testutil.GinTester
	handler *UserHandler
	mocks   struct {
		user *mockcomponent.MockUserComponent
	}
}

func NewUserTester(t *testing.T) *UserTester {
	tester := &UserTester{GinTester: testutil.NewGinTester()}
	tester.mocks.user = mockcomponent.NewMockUserComponent(t)

	tester.handler = &UserHandler{
		user: tester.mocks.user,
	}
	tester.WithParam("name", "u")
	tester.WithParam("namespace", "r")
	return tester
}

func (t *UserTester) WithHandleFunc(fn func(h *UserHandler) gin.HandlerFunc) *UserTester {
	t.Handler(fn(t.handler))
	return t
}

func TestUserHandler_Datasets(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.Datasets
	})

	tester.mocks.user.EXPECT().Datasets(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Dataset{{Name: "ds"}}, 100, nil)
	tester.AddPagination(1, 10).WithUser().WithParam("username", "go").Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Dataset{{Name: "ds"}},
		"total":   100,
	})
}

func TestUserHandler_Models(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.Models
	})

	tester.mocks.user.EXPECT().Models(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Model{{Name: "ds"}}, 100, nil)
	tester.AddPagination(1, 10).WithUser().WithParam("username", "go").Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Model{{Name: "ds"}},
		"total":   100,
	})
}

func TestUserHandler_Codes(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.Codes
	})

	tester.mocks.user.EXPECT().Codes(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Code{{Name: "ds"}}, 100, nil)
	tester.AddPagination(1, 10).WithUser().WithParam("username", "go").Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Code{{Name: "ds"}},
		"total":   100,
	})
}

func TestUserHandler_Spaces(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.Spaces
	})

	tester.mocks.user.EXPECT().Spaces(tester.Ctx(), &types.UserSpacesReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Space{{Name: "ds"}}, 100, nil)
	tester.AddPagination(1, 10).WithUser().WithParam("username", "go").Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Space{{Name: "ds"}},
		"total":   100,
	})
}

func TestUserHandler_LikesAdd(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesAdd
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().AddLikes(tester.Ctx(), &types.UserLikesRequest{
		Username:    "go",
		CurrentUser: "u",
		RepoID:      123,
	}).Return(nil)
	tester.WithParam("username", "go").WithParam("repo_id", "123").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestUserHandler_LikesCollections(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesCollections
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesCollection(tester.Ctx(), &types.UserCollectionReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Collection{{ID: 1}}, 100, nil)
	tester.WithParam("username", "go").AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"data":  []types.Collection{{ID: 1}},
		"total": 100,
	})
}

func TestUserHandler_UserCollections(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.UserCollections
	})

	tester.mocks.user.EXPECT().Collections(tester.Ctx(), &types.UserCollectionReq{
		Owner:       "go",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Collection{{ID: 1}}, 100, nil)
	tester.WithParam("username", "go").WithUser().AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"data":  []types.Collection{{ID: 1}},
		"total": 100,
	})
}

func TestUserHandler_LikeCollection(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikeCollection
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikeCollection(tester.Ctx(), &types.UserLikesRequest{
		CurrentUser:  "u",
		CollectionID: 123,
	}).Return(nil)
	tester.WithParam("id", "123").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestUserHandler_UnLikeCollection(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.UnLikeCollection
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().UnLikeCollection(tester.Ctx(), &types.UserLikesRequest{
		CurrentUser:  "u",
		CollectionID: 123,
	}).Return(nil)
	tester.WithParam("id", "123").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestUserHandler_LikesDelete(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesDelete
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().DeleteLikes(tester.Ctx(), &types.UserLikesRequest{
		CurrentUser: "u",
		RepoID:      123,
	}).Return(nil)
	tester.WithParam("repo_id", "123").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestUserHandler_LikesSpaces(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesSpaces
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesSpaces(tester.Ctx(), &types.UserSpacesReq{
		Owner:       "foo",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Space{{Name: "sp"}}, 100, nil)
	tester.WithParam("username", "foo").AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Space{{Name: "sp"}},
		"total":   100,
	})
}

func TestUserHandler_LikesCodes(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesCodes
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesCodes(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "foo",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Code{{Name: "sp"}}, 100, nil)
	tester.WithParam("username", "foo").AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Code{{Name: "sp"}},
		"total":   100,
	})
}

func TestUserHandler_LikesModels(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesModels
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesModels(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "foo",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Model{{Name: "sp"}}, 100, nil)
	tester.WithParam("username", "foo").AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Model{{Name: "sp"}},
		"total":   100,
	})
}

func TestUserHandler_LikesDatasets(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesDatasets
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesDatasets(tester.Ctx(), &types.UserDatasetsReq{
		Owner:       "foo",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.Dataset{{Name: "sp"}}, 100, nil)
	tester.WithParam("username", "foo").AddPagination(1, 10).Execute()
	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.Dataset{{Name: "sp"}},
		"total":   100,
	})
}

func TestUserHandler_UserPermission(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.UserPermission
	})
	tester.WithUser()

	tester.Execute()
	tester.ResponseEqSimple(t, 200, types.WhoamiResponse{
		Name: "u",
		Auth: types.Auth{
			AccessToken: types.AccessToken{
				DisplayName: "u",
				Role:        "write",
			},
			Type: "Bearer",
		},
	})
}

func TestUserHandler_GetRunDeploys(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.GetRunDeploys
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().ListDeploys(tester.Ctx(), types.ModelRepo, &types.DeployReq{
		CurrentUser: "u",
		RepoType:    types.ModelRepo,
		DeployType:  1,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.DeployRepo{{DeployID: 1}}, 100, nil)
	tester.WithParam("username", "u").WithQuery("deploy_type", "").AddPagination(1, 10)
	tester.WithParam("repo_type", "model").Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.DeployRepo{{DeployID: 1}},
		"total":   100,
	})
}

func TestUserHandler_GetFinetuneInstances(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.GetFinetuneInstances
	})

	tester.mocks.user.EXPECT().ListInstances(tester.Ctx(), &types.UserRepoReq{
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.DeployRepo{{DeployID: 1}}, 100, nil)
	tester.WithUser().WithParam("username", "u").WithQuery("deploy_type", "").AddPagination(1, 10)
	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.DeployRepo{{DeployID: 1}},
		"total":   100,
	})
}

func TestUserHandler_GetRunServerless(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.GetRunServerless
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().ListServerless(tester.Ctx(), types.DeployReq{
		CurrentUser: "u",
		RepoType:    types.ModelRepo,
		DeployType:  3,
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.DeployRepo{{DeployID: 1}}, 100, nil)
	tester.WithParam("username", "u").WithQuery("deploy_type", "").AddPagination(1, 10)
	tester.WithParam("repo_type", "model").Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.DeployRepo{{DeployID: 1}},
		"total":   100,
	})
}

func TestUserHandler_Prompts(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.Prompts
	})

	tester.mocks.user.EXPECT().Prompts(tester.Ctx(), &types.UserPromptsReq{
		CurrentUser: "u",
		Owner:       "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.PromptRes{{ID: 123}}, 100, nil)
	tester.WithUser().WithParam("username", "u").AddPagination(1, 10).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"message": "OK",
		"data":    []types.PromptRes{{ID: 123}},
		"total":   100,
	})
}

func TestUserHandler_GetEvaluations(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.GetEvaluations
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().Evaluations(tester.Ctx(), &types.UserEvaluationReq{
		CurrentUser: "u",
		Owner:       "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.ArgoWorkFlowRes{{ID: 123}}, 100, nil)
	tester.WithParam("username", "u").AddPagination(1, 10).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data":  []types.ArgoWorkFlowRes{{ID: 123}},
		"total": 100,
	})
}

func TestUserHandler_MCPServers(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.MCPServers
	})

	tester.mocks.user.EXPECT().MCPServers(tester.Ctx(), &types.UserMCPsReq{
		CurrentUser: "u",
		Owner:       "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.MCPServer{{ID: 123}}, 1, nil)

	tester.WithUser().WithParam("username", "u").AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MCPServer{{ID: 123}},
		"total": 1,
	})
}

func TestUserHandler_LikesMCPServers(t *testing.T) {
	tester := NewUserTester(t).WithHandleFunc(func(h *UserHandler) gin.HandlerFunc {
		return h.LikesMCPServers
	})
	tester.WithUser()

	tester.mocks.user.EXPECT().LikesMCPServers(tester.Ctx(), &types.UserMCPsReq{
		Owner:       "foo",
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}).Return([]types.MCPServer{{Name: "sp"}}, 100, nil)

	tester.WithParam("username", "foo").AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"data":  []types.MCPServer{{Name: "sp"}},
		"total": 100,
	})
}
