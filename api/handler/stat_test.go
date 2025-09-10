package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

func TestStatHandler_GetStatSnap(t *testing.T) {
	t.Run("invalid target_type", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/statSnap?target_type=invalid&date_type=year", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		statHandler := &StatHandler{}
		statHandler.GetStatSnap(ginContext)

		require.Equal(t, http.StatusBadRequest, hr.Code, hr.Body.String())
	})

	t.Run("invalid date_type", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/statSnap?target_type=users&date_type=invalid", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		statHandler := &StatHandler{}
		statHandler.GetStatSnap(ginContext)

		require.Equal(t, http.StatusBadRequest, hr.Code, hr.Body.String())
	})

	t.Run("valid request", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/statSnap?target_type=users&date_type=year", nil)

		hr := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(hr)
		ginContext.Request = req

		statComp := mockcom.NewMockStatComponent(t)

		expectedResp := &types.StatSnapshotResp{}
		expectedReq := types.StatSnapshotReq{
			TargetType: types.StatTargetType("users"),
			DateType:   types.StatDateType("year"),
		}

		statComp.EXPECT().
			GetStatSnap(mock.Anything, expectedReq).
			Return(expectedResp, nil)

		statHandler := &StatHandler{sc: statComp}
		statHandler.GetStatSnap(ginContext)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err := json.Unmarshal(hr.Body.Bytes(), &resp)
		require.Nil(t, err)
		require.Equal(t, "", resp.Code)
		require.Equal(t, "", resp.Msg)
		require.NotNil(t, resp.Data)
	})
}

func TestStatHandler_StatRunningDeploys(t *testing.T) {
	t.Run("internal error", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/stat/running-deploys", nil)
		hr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(hr)
		ctx.Request = req

		// Set current user
		httpbase.SetCurrentUser(ctx, "test-user")

		mockComp := mockcom.NewMockStatComponent(t)
		mockComp.EXPECT().
			StatRunningDeploys(mock.Anything).
			Return(nil, errors.New("internal error"))

		handler := &StatHandler{sc: mockComp}
		handler.StatRunningDeploys(ctx)

		require.Equal(t, http.StatusInternalServerError, hr.Code, hr.Body.String())
	})

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest("get", "/api/v1/stat/running-deploys", nil)
		hr := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(hr)
		ctx.Request = req

		// Set current user
		httpbase.SetCurrentUser(ctx, "test-user")

		mockComp := mockcom.NewMockStatComponent(t)
		expected := map[int]*types.StatRunningDeploy{
			1: {DeployNum: 3, CPUNum: 4, GPUNum: 1},
			2: {DeployNum: 2, CPUNum: 2, GPUNum: 0},
		}

		mockComp.EXPECT().
			StatRunningDeploys(mock.Anything).
			Return(expected, nil)

		handler := &StatHandler{sc: mockComp}
		handler.StatRunningDeploys(ctx)

		require.Equal(t, http.StatusOK, hr.Code, hr.Body.String())

		var resp httpbase.R
		err := json.Unmarshal(hr.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.Equal(t, "", resp.Code)
		require.NotNil(t, resp.Data)

		// Convert the response to expected type and verify
		dataBytes, _ := json.Marshal(resp.Data)
		var actual map[string]types.StatRunningDeploy
		err = json.Unmarshal(dataBytes, &actual)
		require.NoError(t, err)

		// Check values are consistent
		require.Equal(t, expected[1].DeployNum, actual["1"].DeployNum)
		require.Equal(t, expected[1].CPUNum, actual["1"].CPUNum)
		require.Equal(t, expected[1].GPUNum, actual["1"].GPUNum)
		require.Equal(t, expected[2].DeployNum, actual["2"].DeployNum)
		require.Equal(t, expected[2].CPUNum, actual["2"].CPUNum)
		require.Equal(t, expected[2].GPUNum, actual["2"].GPUNum)
	})
}
