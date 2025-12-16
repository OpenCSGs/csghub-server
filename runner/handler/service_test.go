package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestK8sHandler_CreateRevisions_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().CreateRevisions(mock.Anything, mock.Anything).Return(nil)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.POST("/api/v1/:service/revision", handler.CreateRevisions)

	request := &types.CreateRevisionReq{
		ClusterID:      "test-cluster",
		SvcName:        "test-service",
		Commit:         "abc123",
		InitialTraffic: 50,
	}

	body, err := json.Marshal(request)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/v1/test-service/revision", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestK8sHandler_CreateRevisions_InvalidParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	// No mock setup expected for invalid request

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.POST("/api/v1/:service/revision", handler.CreateRevisions)

	request := &[]types.CreateRevisionReq{}

	body, err := json.Marshal(request)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/v1/test-service/revision", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestK8sHandler_CreateRevisions_ServiceComponentError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().CreateRevisions(mock.Anything, mock.Anything).Return(assert.AnError)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.POST("/api/v1/:service/revision", handler.CreateRevisions)

	request := &types.CreateRevisionReq{
		ClusterID:      "test-cluster",
		SvcName:        "test-service",
		Commit:         "abc123",
		InitialTraffic: 50,
	}

	body, err := json.Marshal(request)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/api/v1/test-service/revision", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestK8sHandler_SetVersionsTraffic_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().SetVersionsTraffic(mock.Anything, "test-cluster", "test-service", mock.Anything).Return(nil)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.PUT("/api/v1/:service/traffic", handler.SetVersionsTraffic)

	trafficReqs := []types.TrafficReq{
		{Commit: "commit1", TrafficPercent: 50},
		{Commit: "commit2", TrafficPercent: 50},
	}

	body, err := json.Marshal(trafficReqs)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("PUT", "/api/v1/test-service/traffic?cluster_id=test-cluster", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestK8sHandler_SetVersionsTraffic_InvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	// No mock setup expected for invalid request

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.PUT("/api/v1/:service/traffic", handler.SetVersionsTraffic)
	sc.EXPECT().SetVersionsTraffic(mock.Anything, "test-cluster", "test-service", mock.Anything).Return(errorx.ErrRunnerMaxRevision)

	trafficReqs := []types.TrafficReq{
		{Commit: "", TrafficPercent: 100},
	}

	body, err := json.Marshal(trafficReqs)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("PUT", "/api/v1/test-service/traffic?cluster_id=test-cluster", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestK8sHandler_SetVersionsTraffic_ServiceComponentError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().SetVersionsTraffic(mock.Anything, "test-cluster", "test-service", mock.Anything).Return(assert.AnError)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.PUT("/api/v1/:service/traffic", handler.SetVersionsTraffic)

	trafficReqs := []types.TrafficReq{
		{Commit: "commit1", TrafficPercent: 100},
	}

	body, err := json.Marshal(trafficReqs)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("PUT", "/api/v1/test-service/traffic?cluster_id=test-cluster", bytes.NewBuffer(body))
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestK8sHandler_ListKsvcVersions_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().ListVersions(mock.Anything, "test-cluster", "test-service").Return([]types.KsvcRevisionInfo{}, nil)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.GET("/api/v1/:service/versions", handler.ListKsvcVersions)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/v1/test-service/versions?cluster_id=test-cluster", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestK8sHandler_ListKsvcVersions_ServiceComponentError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().ListVersions(mock.Anything, "test-cluster", "test-service").Return(nil, assert.AnError)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.GET("/api/v1/:service/versions", handler.ListKsvcVersions)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/v1/test-service/versions?cluster_id=test-cluster", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestK8sHandler_DeleteKsvcVersion_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().DeleteKsvcVersion(mock.Anything, "test-cluster", "test-service", "commit123").Return(nil)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.DELETE("/api/v1/:service/version/:commit_id", handler.DeleteKsvcVersion)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", "/api/v1/test-service/version/commit123?cluster_id=test-cluster", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestK8sHandler_DeleteKsvcVersion_ServiceComponentError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sc := mockcomponent.NewMockServiceComponent(t)
	sc.EXPECT().DeleteKsvcVersion(mock.Anything, "test-cluster", "test-service", "commit123").Return(assert.AnError)

	handler := &K8sHandler{
		serviceComponent: sc,
	}

	router := gin.Default()
	router.DELETE("/api/v1/:service/version/:commit_id", handler.DeleteKsvcVersion)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", "/api/v1/test-service/version/commit123?cluster_id=test-cluster", nil)
	require.NoError(t, err)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
}
