package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type SpaceResourceTester struct {
	*testutil.GinTester
	handler *SpaceResourceHandler
	mocks   struct {
		spaceResource *mockcomponent.MockSpaceResourceComponent
	}
}

func NewSpaceResourceTester(t *testing.T) *SpaceResourceTester {
	tester := &SpaceResourceTester{GinTester: testutil.NewGinTester()}
	tester.mocks.spaceResource = mockcomponent.NewMockSpaceResourceComponent(t)

	tester.handler = &SpaceResourceHandler{
		spaceResource: tester.mocks.spaceResource,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *SpaceResourceTester) WithHandleFunc(fn func(h *SpaceResourceHandler) gin.HandlerFunc) *SpaceResourceTester {
	t.Handler(fn(t.handler))
	return t
}

func TestSpaceResourceHandler_Index(t *testing.T) {
	tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
		return h.Index
	})

	req := &types.SpaceResourceIndexReq{
		ClusterID:   "c1",
		DeployType:  types.InferenceType,
		CurrentUser: "u",
		PageOpts: types.PageOpts{
			PageSize: 50,
			Page:     1,
		},
	}

	tester.mocks.spaceResource.EXPECT().Index(tester.Ctx(), req).Return(
		[]types.SpaceResource{{Name: "sp"}}, 0, nil,
	)
	tester.WithQuery("cluster_id", "c1").WithQuery("deploy_type", "").WithUser().Execute()

	tester.ResponseEq(t, 200, tester.OKText, []types.SpaceResource{{Name: "sp"}})
}

func TestSpaceResourceHandler_Create(t *testing.T) {
	tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
		return h.Create
	})

	tester.mocks.spaceResource.EXPECT().Create(tester.Ctx(), &types.CreateSpaceResourceReq{
		ClusterID: "c",
		Name:      "n",
		Resources: "r",
	}).Return(
		&types.SpaceResource{Name: "sp"}, nil,
	)
	tester.WithBody(t, &types.CreateSpaceResourceReq{
		ClusterID: "c", Name: "n", Resources: "r",
	}).WithUser().Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.SpaceResource{Name: "sp"})
}

func TestSpaceResourceHandler_Update(t *testing.T) {
	tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
		return h.Update
	})

	tester.mocks.spaceResource.EXPECT().Update(tester.Ctx(), &types.UpdateSpaceResourceReq{
		Name:      "n",
		Resources: "r",
		ID:        1,
	}).Return(
		&types.SpaceResource{Name: "sp"}, nil,
	)
	tester.WithBody(t, &types.UpdateSpaceResourceReq{
		Name: "n", Resources: "r",
	}).WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.SpaceResource{Name: "sp"})
}

func TestSpaceResourceHandler_Delete(t *testing.T) {
	tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
		return h.Delete
	})

	tester.mocks.spaceResource.EXPECT().Delete(tester.Ctx(), int64(1)).Return(
		nil,
	)
	tester.WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
