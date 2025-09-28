package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type clusterTester struct {
	*testutil.GinTester
	handler *ClusterHandler
	mocks   struct {
		clusterComponent *mockcomponent.MockClusterComponent
	}
}

func newClusterTester(t *testing.T) *clusterTester {
	tester := &clusterTester{GinTester: testutil.NewGinTester()}
	tester.mocks.clusterComponent = mockcomponent.NewMockClusterComponent(t)
	tester.handler = &ClusterHandler{
		c: tester.mocks.clusterComponent,
	}
	return tester
}

func (ct *clusterTester) withHandlerFunc(fn func(clusterHandler *ClusterHandler) gin.HandlerFunc) *clusterTester {
	ct.Handler(fn(ct.handler))
	return ct
}

func Test_GetClusterByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
			return clusterHandler.GetClusterById
		})
		tester.mocks.clusterComponent.EXPECT().
			GetClusterWithResourceByID(context.Background(), "1").
			Once().
			Return(&types.ClusterRes{
				ResourceStatus: types.StatusUncertain,
			}, nil)
		tester.WithParam("id", "1").Execute()
		tester.ResponseEqSimple(t, 200, httpbase.R{
			Msg: "OK",
			Data: &types.ClusterRes{
				ResourceStatus: types.StatusUncertain,
			}})
	})
	t.Run("5xx failed", func(t *testing.T) {
		tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
			return clusterHandler.GetClusterById
		})
		tester.mocks.clusterComponent.EXPECT().
			GetClusterWithResourceByID(context.Background(), "1").
			Once().
			Return(nil, errorx.ErrInternalServerError)
		tester.WithParam("id", "1").Execute()
		tester.ResponseEqSimple(t, http.StatusInternalServerError, httpbase.R{
			Msg:  errorx.ErrInternalServerError.Error(),
			Code: errorx.ErrInternalServerError.(errorx.CustomError).Code()})
	})
}
