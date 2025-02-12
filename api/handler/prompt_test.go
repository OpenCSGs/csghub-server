package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type PromptTester struct {
	*testutil.GinTester
	handler *PromptHandler
	mocks   struct {
		prompt    *mock_component.MockPromptComponent
		sensitive *mock_component.MockSensitiveComponent
		repo      *mock_component.MockRepoComponent
	}
}

func NewPromptTester(t *testing.T) *PromptTester {
	tester := &PromptTester{GinTester: testutil.NewGinTester()}
	tester.mocks.prompt = mock_component.NewMockPromptComponent(t)
	tester.mocks.sensitive = mock_component.NewMockSensitiveComponent(t)
	tester.mocks.repo = mock_component.NewMockRepoComponent(t)
	tester.handler = &PromptHandler{
		prompt: tester.mocks.prompt, sensitive: tester.mocks.sensitive,
		repo: tester.mocks.repo,
	}
	tester.WithParam("name", "r")
	tester.WithParam("namespace", "u")
	return tester

}

func (t *PromptTester) WithHandleFunc(fn func(h *PromptHandler) gin.HandlerFunc) *PromptTester {
	t.Handler(fn(t.handler))
	return t

}

func TestPromptHandler_Index(t *testing.T) {
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

			tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
				return h.Index
			})

			if !c.error {
				tester.mocks.prompt.EXPECT().IndexPromptRepo(tester.Ctx(), &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1).Return([]types.PromptRes{
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
					"data":  []types.PromptRes{{Name: "cc"}},
					"total": 100,
				})
			}
		})
	}
}

func TestPromptHandler_ListPrompt(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.ListPrompt
	})

	tester.WithUser()
	tester.mocks.prompt.EXPECT().Show(tester.Ctx(), "u", "r", "u").Return(&types.PromptRes{Name: "p"}, nil)
	tester.mocks.prompt.EXPECT().ListPrompt(tester.Ctx(), types.PromptReq{
		Namespace: "u", Name: "r", CurrentUser: "u",
	}).Return([]types.PromptOutput{{FilePath: "fp"}}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{
		"detail":  &types.PromptRes{Name: "p"},
		"prompts": []types.PromptOutput{{FilePath: "fp"}},
	})
}

func TestPromptHandler_GetPrompt(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.GetPrompt
	})

	tester.WithUser().WithParam("file_path", "fp")
	tester.mocks.prompt.EXPECT().GetPrompt(tester.Ctx(), types.PromptReq{
		Namespace: "u", Name: "r", CurrentUser: "u", Path: "fp",
	}).Return(&types.PromptOutput{FilePath: "fp"}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.PromptOutput{FilePath: "fp"})
}

func TestPromptHandler_CreatePrompt(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.CreatePrompt
	})
	tester.RequireUser(t)

	req := &types.CreatePromptReq{Prompt: types.Prompt{
		Title: "t", Content: "c", Language: "l",
	}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.prompt.EXPECT().CreatePrompt(tester.Ctx(), types.PromptReq{
		Namespace: "u", Name: "r", CurrentUser: "u",
	}, req).Return(&types.Prompt{Title: "p"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Prompt{Title: "p"})
}

func TestPromptHandler_UpdatePrompt(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.UpdatePrompt
	})
	tester.RequireUser(t)

	req := &types.UpdatePromptReq{Prompt: types.Prompt{
		Title: "t", Content: "c", Language: "l",
	}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.prompt.EXPECT().UpdatePrompt(tester.Ctx(), types.PromptReq{
		Namespace: "u", Name: "r", CurrentUser: "u", Path: "fp",
	}, req).Return(&types.Prompt{Title: "p"}, nil)
	tester.WithParam("file_path", "fp").WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Prompt{Title: "p"})
}

func TestPromptHandler_DeletePrompt(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.DeletePrompt
	})
	tester.RequireUser(t)

	tester.WithUser().WithParam("file_path", "fp")
	tester.mocks.prompt.EXPECT().DeletePrompt(tester.Ctx(), types.PromptReq{
		Namespace: "u", Name: "r", CurrentUser: "u", Path: "fp",
	}).Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_Relations(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Relations
	})

	tester.WithUser()
	tester.mocks.prompt.EXPECT().Relations(tester.Ctx(), "u", "r", "u").Return(&types.Relations{}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Relations{})
}

func TestPromptHandler_SetRelations(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.SetRelations
	})
	tester.RequireUser(t)

	req := types.RelationModels{Namespace: "u", Name: "r", CurrentUser: "u"}
	tester.mocks.prompt.EXPECT().SetRelationModels(tester.Ctx(), req).Return(nil)
	tester.WithBody(t, types.RelationModels{Name: "rm"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_AddModelRelation(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.AddModelRelation
	})
	tester.RequireUser(t)

	req := types.RelationModel{Namespace: "u", Name: "r", CurrentUser: "u"}
	tester.mocks.prompt.EXPECT().AddRelationModel(tester.Ctx(), req).Return(nil)
	tester.WithBody(t, types.RelationModels{Name: "rm"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_DeleteModelRelation(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.AddModelRelation
	})
	tester.RequireUser(t)

	req := types.RelationModel{Namespace: "u", Name: "r", CurrentUser: "u"}
	tester.mocks.prompt.EXPECT().AddRelationModel(tester.Ctx(), req).Return(nil)
	tester.WithBody(t, types.RelationModels{Name: "rm"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_CreatePromptRepo(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.RequireUser(t)

	req := &types.CreatePromptRepoReq{CreateRepoReq: types.CreateRepoReq{}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	reqn := *req
	reqn.Username = "u"
	tester.mocks.prompt.EXPECT().CreatePromptRepo(tester.Ctx(), &reqn).Return(
		&types.PromptRes{Name: "p"}, nil,
	)
	tester.WithBody(t, req).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"data": &types.PromptRes{Name: "p"},
	})
}

func TestPromptHandler_UpdatePromptRepo(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.RequireUser(t)

	req := &types.UpdatePromptRepoReq{UpdateRepoReq: types.UpdateRepoReq{}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	reqn := *req
	reqn.Namespace = "u"
	reqn.Name = "r"
	reqn.Username = "u"
	tester.mocks.prompt.EXPECT().UpdatePromptRepo(tester.Ctx(), &reqn).Return(
		&types.PromptRes{Name: "p"}, nil,
	)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.PromptRes{Name: "p"})
}

func TestPromptHandler_DeletePromptRepo(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.RequireUser(t)

	tester.mocks.prompt.EXPECT().RemoveRepo(tester.Ctx(), "u", "r", "u").Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_Branches(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Branches
	})

	tester.mocks.repo.EXPECT().Branches(tester.Ctx(), &types.GetBranchesReq{
		Namespace:   "u",
		Name:        "r",
		Per:         10,
		Page:        1,
		RepoType:    types.PromptRepo,
		CurrentUser: "u",
	}).Return([]types.Branch{{Name: "main"}}, nil)
	tester.WithUser().AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, []types.Branch{{Name: "main"}})
}

func TestPromptHandler_Tags(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.Tags
	})

	tester.mocks.repo.EXPECT().Tags(tester.Ctx(), &types.GetTagsReq{
		Namespace:   "u",
		Name:        "r",
		RepoType:    types.PromptRepo,
		CurrentUser: "u",
	}).Return([]database.Tag{{Name: "main"}}, nil)
	tester.WithUser().AddPagination(1, 10).Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.Tag{{Name: "main"}})
}

func TestPromptHandler_UpdateTags(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.UpdateTags
	})
	tester.RequireUser(t)

	req := []string{"a", "b"}
	tester.mocks.repo.EXPECT().UpdateTags(tester.Ctx(), "u", "r", types.PromptRepo, "cat", "u", req).Return(nil)
	tester.WithBody(t, req).WithParam("category", "cat").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestPromptHandler_UpdateDownloads(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.UpdateDownloads
	})

	tester.mocks.repo.EXPECT().UpdateDownloads(tester.Ctx(), &types.UpdateDownloadsReq{
		Namespace: "u",
		Name:      "r",
		RepoType:  types.PromptRepo,
		Date:      time.Date(2012, 12, 12, 0, 0, 0, 0, time.UTC),
		ReqDate:   "2012-12-12",
	}).Return(nil)
	tester.WithUser().WithBody(t, &types.UpdateDownloadsReq{
		ReqDate: time.Date(2012, 12, 12, 0, 0, 0, 0, time.UTC).Format("2006-01-02"),
	}).WithParam("category", "cat").Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}
