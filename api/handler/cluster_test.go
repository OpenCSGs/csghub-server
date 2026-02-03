package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

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

	start := "2024-01-01 00:00:00"
	end := "2024-01-31"
	expectedStart, err := time.ParseInLocation(time.DateTime, start, time.UTC)
	require.NoError(t, err)
	endDate, err := time.ParseInLocation("2006-01-02", end, time.UTC)
	require.NoError(t, err)
	expectedEnd := endDate.Add(24*time.Hour - time.Nanosecond)
	tester.mocks.clusterComponent.EXPECT().
		GetDeploys(context.Background(), mock.Anything).
		Run(func(_ context.Context, req types.DeployReq) {
			require.NotNil(t, req.StartTime)
			require.True(t, req.StartTime.Equal(expectedStart))
			require.NotNil(t, req.EndTime)
			require.True(t, req.EndTime.Equal(expectedEnd))
		}).
		Once().
		Return(rows, len(rows), nil)

	tester.WithQuery("start_time", start).WithQuery("end_time", end).Execute()

	// assert response headers and body
	resp := tester.Response()
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/csv")
	require.Contains(t, resp.Header().Get("Content-Disposition"), "deploys_report.csv")
	require.Contains(t, resp.Header().Get("Cache-Control"), "no-cache")
	require.Contains(t, resp.Header().Get("Connection"), "keep-alive")
	body := resp.Body.String()
	require.Contains(t, body, "ClusterID,ClusterRegion,DeployName,Username,Resource,CreateTime,Status,TotalTimeInMin,TotalFeeInCents")
	require.Contains(t, body, "alice")
	require.Contains(t, body, "bob")
}

func Test_GetDeploysReport_InvalidDateRange(t *testing.T) {
	tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
		return clusterHandler.GetDeploysReport
	})

	tester.WithQuery("start_time", "2024-01-01").Execute()
	tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{Msg: "start_time and end_time must be provided together"})
}

func Test_GetDeploysReport_InvalidFormat(t *testing.T) {
	tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
		return clusterHandler.GetDeploysReport
	})

	tester.WithQuery("start_time", "invalid").WithQuery("end_time", "2024-01-01").Execute()
	tester.ResponseEqSimple(t, http.StatusBadRequest, httpbase.R{Msg: "invalid datetime format, use '2006-01-02 15:04:05' or '2006-01-02'"})
}

func Test_GetClusterPublic(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
			return clusterHandler.GetClusterPublic
		})
		
		expectedResult := types.PublicClusterRes{
			Hardware: []types.HardwareInfo{
				{GPUVendor: "NVIDIA", XPUModel: "A100", XPUMem: 40, Region: "us-west-1"},
			},
			Regions: []string{"us-west-1"},
			GPUVendors: []string{"NVIDIA"},
		}
		
		tester.mocks.clusterComponent.EXPECT().
			IndexPublic(context.Background()).
			Once().
			Return(expectedResult, nil)
		
		tester.Execute()
		tester.ResponseEqSimple(t, 200, httpbase.R{
			Msg: "OK",
			Data: expectedResult,
		})
	})
	
	t.Run("error", func(t *testing.T) {
		tester := newClusterTester(t).withHandlerFunc(func(clusterHandler *ClusterHandler) gin.HandlerFunc {
			return clusterHandler.GetClusterPublic
		})
		
		tester.mocks.clusterComponent.EXPECT().
			IndexPublic(context.Background()).
			Once().
			Return(types.PublicClusterRes{}, errorx.ErrInternalServerError)
		
		tester.Execute()
		tester.ResponseEqSimple(t, http.StatusInternalServerError, httpbase.R{
			Msg:  errorx.ErrInternalServerError.Error(),
			Code: errorx.ErrInternalServerError.(errorx.CustomError).Code(),
		})
	})
}
