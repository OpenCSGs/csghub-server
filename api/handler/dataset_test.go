package handler

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type DatasetTester struct {
	*testutil.GinTester
	handler *DatasetHandler
	mocks   struct {
		dataset   *mockcomponent.MockDatasetComponent
		sensitive *mockcomponent.MockSensitiveComponent
		repo      *mockcomponent.MockRepoComponent
	}
}

func NewDatasetTester(t *testing.T) *DatasetTester {
	tester := &DatasetTester{GinTester: testutil.NewGinTester()}
	tester.mocks.dataset = mockcomponent.NewMockDatasetComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)

	tester.handler = &DatasetHandler{
		dataset:   tester.mocks.dataset,
		sensitive: tester.mocks.sensitive,
		repo:      tester.mocks.repo,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *DatasetTester) WithHandleFunc(fn func(h *DatasetHandler) gin.HandlerFunc) *DatasetTester {
	t.Handler(fn(t.handler))
	return t
}

func TestDatasetHandler_Index(t *testing.T) {
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

			tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
				return h.Index
			})

			if !c.error {
				tester.mocks.dataset.EXPECT().Index(tester.Ctx(), &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1).Return([]types.Dataset{
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
					"data":  []types.Dataset{{Name: "cc"}},
					"total": 100,
					"msg":   "OK",
				})
			}
		})
	}
}

func TestDatasetHandler_Update(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Update
		})
		tester.RequireUser(t)

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.UpdateDatasetReq{}).Return(true, nil)
		tester.mocks.dataset.EXPECT().Update(tester.Ctx(), &types.UpdateDatasetReq{
			UpdateRepoReq: types.UpdateRepoReq{
				Username:  "u",
				Namespace: "u-other",
				Name:      "r",
			},
		}).Return(nil, errorx.ErrForbiddenMsg("user not allowed to update dataset"))
		tester.WithParam("namespace", "u-other").WithParam("name", "r").
			WithBody(t, &types.UpdateDatasetReq{
				UpdateRepoReq: types.UpdateRepoReq{Name: "r"},
			}).
			WithUser().
			Execute()

		require.Equal(t, 403, tester.Response().Code)
	})

	t.Run("normal", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Update
		})
		tester.RequireUser(t)

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.UpdateDatasetReq{}).Return(true, nil)
		tester.mocks.dataset.EXPECT().Update(tester.Ctx(), &types.UpdateDatasetReq{
			UpdateRepoReq: types.UpdateRepoReq{
				Username:  "u",
				Namespace: "u",
				Name:      "r",
			},
		}).Return(&types.Dataset{Name: "foo"}, nil)
		tester.WithBody(t, &types.UpdateDatasetReq{
			UpdateRepoReq: types.UpdateRepoReq{Name: "r"},
		}).Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.Dataset{Name: "foo"})
	})
}

func TestDatasetHandler_Delete(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Delete
		})
		tester.RequireUser(t)

		tester.mocks.dataset.EXPECT().Delete(tester.Ctx(), "u-other", "r", "u").Return(errorx.ErrForbidden)
		tester.WithParam("namespace", "u-other").WithParam("name", "r")
		tester.WithUser().Execute()

		require.Equal(t, 403, tester.Response().Code)
	})

	t.Run("normal", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Delete
		})
		tester.RequireUser(t)

		tester.mocks.dataset.EXPECT().Delete(tester.Ctx(), "u", "r", "u").Return(nil)
		tester.Execute()

		tester.ResponseEq(t, 200, tester.OKText, nil)
	})
}

func TestDatasetHandler_Show(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Show
		})

		tester.mocks.dataset.EXPECT().Show(tester.Ctx(), "u-other", "r", "u").Return(nil, errorx.ErrForbidden)
		tester.WithParam("namespace", "u-other").WithParam("name", "r")
		tester.WithUser().Execute()

		require.Equal(t, 403, tester.Response().Code)
	})

	t.Run("normal", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Show
		})

		tester.mocks.dataset.EXPECT().Show(tester.Ctx(), "u", "r", "u").Return(&types.Dataset{
			Name: "d",
		}, nil)
		tester.WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.Dataset{Name: "d"})
	})
}

func TestDatasetHandler_Relations(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Relations
		})

		tester.mocks.dataset.EXPECT().Relations(tester.Ctx(), "u-other", "r", "u").Return(nil, errorx.ErrForbidden)
		tester.WithParam("namespace", "u-other").WithParam("name", "r")
		tester.WithUser().Execute()

		require.Equal(t, 403, tester.Response().Code)

	})

	t.Run("normal", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.Relations
		})

		tester.mocks.dataset.EXPECT().Relations(tester.Ctx(), "u", "r", "u").Return(&types.Relations{
			Models: []*types.Model{{Name: "m"}},
		}, nil)
		tester.WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.Relations{
			Models: []*types.Model{{Name: "m"}},
		})
	})
}

func TestDatasetHandler_AllFiles(t *testing.T) {
	t.Run("forbidden", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.AllFiles
		})

		tester.mocks.repo.EXPECT().AllFiles(tester.Ctx(), types.GetAllFilesReq{
			Namespace:   "u-other",
			Name:        "r",
			RepoType:    types.DatasetRepo,
			CurrentUser: "u",
		}).Return(nil, errorx.ErrForbidden)
		tester.WithParam("namespace", "u-other").WithParam("name", "r")
		tester.WithUser().Execute()

		require.Equal(t, 403, tester.Response().Code)
	})

	t.Run("normal", func(t *testing.T) {
		tester := NewDatasetTester(t).WithHandleFunc(func(h *DatasetHandler) gin.HandlerFunc {
			return h.AllFiles
		})

		tester.mocks.repo.EXPECT().AllFiles(tester.Ctx(), types.GetAllFilesReq{
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.DatasetRepo,
			CurrentUser: "u",
		}).Return([]*types.File{{Name: "f"}}, nil)
		tester.WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, []*types.File{{Name: "f"}})
	})
}
