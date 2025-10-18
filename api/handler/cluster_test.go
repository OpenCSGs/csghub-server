package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func Test_GetDeploysReport(t *testing.T) {
	tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
		return clusterHandler.GetDeploysReport
	})

	rows := []types.DeployRes{
		{
			ClusterID:       "c1",
			ClusterRegion:   "us-west-1",
			DeployName:      "d1",
			User:            types.User{Username: "alice"},
			Resource:        "cpu=2",
			Status:          "Running",
			TotalTimeInMin:  30,
			TotalFeeInCents: 100,
		},
		{
			ClusterID:       "c2",
			ClusterRegion:   "us-east-1",
			DeployName:      "d2",
			User:            types.User{Username: "bob"},
			Resource:        "cpu=4",
			Status:          "Stopped",
			TotalTimeInMin:  10,
			TotalFeeInCents: 50,
		},
	}

	tester.mocks.clusterComponent.EXPECT().
		GetDeploys(context.Background(), mock.Anything).
		Once().
		Return(rows, len(rows), nil)

	tester.Execute()

	// assert response headers and body
	resp := tester.Response()
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/csv")
	require.Contains(t, resp.Header().Get("Content-Disposition"), "deploys_report.csv")
	body := resp.Body.String()
	require.Contains(t, body, "ClusterID,ClusterRegion,DeployName,Username,Resource,CreateTime,Status,TotalTimeInMin,TotalFeeInCents")
	require.Contains(t, body, "alice")
	require.Contains(t, body, "bob")
}
