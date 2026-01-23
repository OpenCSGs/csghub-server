package handler

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type SpaceResourceTester struct {
	*testutil.GinTester
	handler *SpaceResourceHandler
	mocks   struct {
		spaceResource *mockcomponent.MockSpaceResourceComponent
		cluster       *mockcomponent.MockClusterComponent
	}
}

func NewSpaceResourceTester(t *testing.T) *SpaceResourceTester {
	tester := &SpaceResourceTester{GinTester: testutil.NewGinTester()}
	tester.mocks.spaceResource = mockcomponent.NewMockSpaceResourceComponent(t)
	tester.mocks.cluster = mockcomponent.NewMockClusterComponent(t)

	tester.handler = &SpaceResourceHandler{
		spaceResource: tester.mocks.spaceResource,
		cluster:       tester.mocks.cluster,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	v, ok := binding.Validator.Engine().(*validator.Validate)
	assert.True(t, ok)
	err := v.RegisterValidation("resource_type", types.ResourceTypeValidator)
	assert.NoError(t, err)
	return tester
}

func (t *SpaceResourceTester) WithHandleFunc(fn func(h *SpaceResourceHandler) gin.HandlerFunc) *SpaceResourceTester {
	t.Handler(fn(t.handler))
	return t
}

func TestSpaceResourceHandler_Index(t *testing.T) {
	t.Run("200", func(t *testing.T) {
		tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
			return h.Index
		})
		deployType := types.InferenceType
		req := &types.SpaceResourceIndexReq{
			ClusterIDs:   []string{"c1"},
			DeployType:   deployType,
			CurrentUser:  "u",
			ResourceType: types.ResourceTypeGPU,
			HardwareType: "A10",
			Per:          50,
			Page:         1,
		}

		tester.mocks.spaceResource.EXPECT().Index(tester.Ctx(), req).Return(
			[]types.SpaceResource{{Name: "sp"}}, 0, nil,
		)
		tester.WithQuery("cluster_id", "c1").WithQuery("deploy_type", "1").
			WithQuery("resource_type", "gpu").WithQuery("hardware_type", "A10").WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, []types.SpaceResource{{Name: "sp"}})
	})
	t.Run("200 with no cluster", func(t *testing.T) {
		tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
			return h.Index
		})
		deployType := types.InferenceType
		req := &types.SpaceResourceIndexReq{
			ClusterIDs:   []string{"c1"},
			DeployType:   deployType,
			CurrentUser:  "u",
			ResourceType: types.ResourceTypeGPU,
			HardwareType: "A10",
			Per:          50,
			Page:         1,
		}
		tester.mocks.cluster.EXPECT().Index(mock.Anything).
			Return([]types.ClusterRes{
				{ClusterID: "c1"},
			}, nil)

		tester.mocks.spaceResource.EXPECT().Index(tester.Ctx(), req).Return(
			[]types.SpaceResource{{Name: "sp"}}, 0, nil,
		)
		tester.WithQuery("deploy_type", "1").
			WithQuery("resource_type", "gpu").WithQuery("hardware_type", "A10").WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, []types.SpaceResource{{Name: "sp"}})
	})
	t.Run("invalid resource type", func(t *testing.T) {
		tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
			return h.Index
		})

		tester.WithQuery("cluster_id", "c1").WithQuery("deploy_type", "").
			WithQuery("resource_type", "invalid").WithQuery("hardware_type", "intel").WithUser().Execute()

		tester.ResponseEqCode(t, 400)
	})
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

func TestSpaceResourceHandler_ListHardwareTypes(t *testing.T) {
	t.Run("200", func(t *testing.T) {
		tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
			return h.ListHardwareTypes
		})
		tester.mocks.spaceResource.EXPECT().ListHardwareTypes(tester.Ctx(), "c1").Return(
			[]string{"NVIDIA A100", "Intel Xeon"}, nil,
		)
		tester.WithQuery("cluster_id", "c1").WithUser().Execute()

		tester.ResponseEq(t, 200, tester.OKText, []string{"NVIDIA A100", "Intel Xeon"})
	})
	t.Run("error", func(t *testing.T) {
		tester := NewSpaceResourceTester(t).WithHandleFunc(func(h *SpaceResourceHandler) gin.HandlerFunc {
			return h.ListHardwareTypes
		})
		tester.mocks.spaceResource.EXPECT().ListHardwareTypes(tester.Ctx(), "c1").Return(
			nil, errors.New("database error"),
		)
		tester.WithQuery("cluster_id", "c1").WithUser().Execute()

		tester.ResponseEq(t, 500, "database error", nil)
	})
}
