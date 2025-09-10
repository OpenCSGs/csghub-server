package handler

import (
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type CollectionTester struct {
	*testutil.GinTester
	handler *CollectionHandler
	mocks   struct {
		collection *mockcomponent.MockCollectionComponent
		sensitive  *mockcomponent.MockSensitiveComponent
	}
}

func NewCollectionTester(t *testing.T) *CollectionTester {
	tester := &CollectionTester{GinTester: testutil.NewGinTester()}
	tester.mocks.collection = mockcomponent.NewMockCollectionComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)

	tester.handler = &CollectionHandler{
		collection: tester.mocks.collection,
		sensitive:  tester.mocks.sensitive,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *CollectionTester) WithHandleFunc(fn func(h *CollectionHandler) gin.HandlerFunc) *CollectionTester {
	t.Handler(fn(t.handler))
	return t
}

func TestCollectionHandler_Index(t *testing.T) {
	cases := []struct {
		sort  string
		error bool
	}{
		{"trending", false},
		{"foo", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
				return h.Index
			})

			if !c.error {
				tester.mocks.collection.EXPECT().GetCollections(tester.Ctx(), &types.CollectionFilter{
					Search: "foo",
					Sort:   c.sort,
				}, 10, 1).Return([]types.Collection{
					{Name: "cc"},
				}, 100, nil)
			}

			tester.AddPagination(1, 10).WithQuery("search", "foo").
				WithQuery("sort", c.sort).Execute()

			if c.error {
				require.Equal(t, 400, tester.Response().Code)
			} else {
				tester.ResponseEqSimple(t, 200, gin.H{
					"data":  []types.Collection{{Name: "cc"}},
					"total": 100,
				})
			}
		})
	}
}

func TestCollectionHandler_Create(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateCollectionReq{}).Return(true, nil)
	tester.mocks.collection.EXPECT().CreateCollection(tester.Ctx(), types.CreateCollectionReq{
		Username: "u",
	}).Return(&database.Collection{ID: 1}, nil)
	tester.WithBody(t, &types.CreateCollectionReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.Collection{ID: 1})

}

func TestCollectionHandler_GetCollection(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.GetCollection
	})

	tester.mocks.collection.EXPECT().GetCollection(
		tester.Ctx(), "u", int64(1),
	).Return(&types.Collection{ID: 1}, nil)
	tester.WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Collection{ID: 1})

}

func TestCollectionHandler_UpdateCollection(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.UpdateCollection
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateCollectionReq{}).Return(true, nil)
	tester.mocks.collection.EXPECT().UpdateCollection(tester.Ctx(), types.CreateCollectionReq{
		ID: 1,
	}).Return(&database.Collection{ID: 1}, nil)
	tester.WithParam("id", "1").WithBody(t, &types.CreateCollectionReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &database.Collection{ID: 1})

}

func TestCollectionHandler_DeleteCollection(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.DeleteCollection
	})
	tester.WithUser()

	tester.mocks.collection.EXPECT().DeleteCollection(
		tester.Ctx(), int64(1), "u",
	).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestCollectionHandler_AddRepoToCollection(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.AddRepoToCollection
	})
	tester.WithUser()

	tester.mocks.collection.EXPECT().AddReposToCollection(tester.Ctx(), types.UpdateCollectionReposReq{
		Username: "u",
		ID:       1,
	}).Return(nil)
	tester.WithParam("id", "1").WithBody(t, &types.UpdateCollectionReposReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestCollectionHandler_AddRepoToCollectionWithRemarks(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.AddRepoToCollection
	})
	tester.WithUser()
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.UpdateCollectionRepoReq{
		Remark: "test remark",
	}).Return(true, nil)
	tester.mocks.collection.EXPECT().AddReposToCollection(tester.Ctx(), types.UpdateCollectionReposReq{
		Username: "u",
		ID:       1,
		Remarks: map[int64]string{
			1: "test remark",
		},
	}).Return(nil)
	tester.WithParam("id", "1").WithBody(t, &types.UpdateCollectionReposReq{
		Remarks: map[int64]string{
			1: "test remark",
		},
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestCollectionHandler_RemoveRepoFromCollection(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.RemoveRepoFromCollection
	})
	tester.WithUser()

	tester.mocks.collection.EXPECT().RemoveReposFromCollection(tester.Ctx(), types.UpdateCollectionReposReq{
		Username: "u",
		ID:       1,
	}).Return(nil)
	tester.WithParam("id", "1").WithBody(t, &types.UpdateCollectionReposReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestCollectionHandler_UpdateCollectionRepo(t *testing.T) {
	tester := NewCollectionTester(t).WithHandleFunc(func(h *CollectionHandler) gin.HandlerFunc {
		return h.UpdateCollectionRepo
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.UpdateCollectionRepoReq{
		Remark: "test remark",
	}).Return(true, nil)
	tester.mocks.collection.EXPECT().UpdateCollectionRepo(tester.Ctx(), types.UpdateCollectionRepoReq{
		Username: "u",
		ID:       1,
		RepoID:   1,
		Remark:   "test remark",
	}).Return(nil)
	tester.WithParam("id", "1").WithParam("repo_id", "1").WithBody(t, &types.UpdateCollectionRepoReq{Remark: "test remark"}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
