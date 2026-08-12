package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

func TestModelHandler_CreateInferenceVersion_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().CreateInferenceVersion(mock.Anything, mock.Anything).Return(nil)

	handler := &ModelHandler{
		model: mc,
	}
	router.POST("/api/v1/models/:namespace/:name/run/versions/:id", handler.CreateInferenceVersion)

	req := &types.CreateInferenceVersionReq{
		CommitID:       "test-commit",
		TrafficPercent: 50,
	}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/api/v1/models/test-namespace/test-model/run/versions/123", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelHandler_CreateInferenceVersion_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.POST("/api/v1/models/:namespace/:name/run/versions/:id", handler.CreateInferenceVersion)

	req := &types.CreateInferenceVersionReq{
		CommitID:       "test-commit",
		TrafficPercent: 50,
	}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/api/v1/models/test-namespace/test-model/run/versions/invalid", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_CreateInferenceVersion_InvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.POST("/api/v1/models/:namespace/:name/run/versions/:id", handler.CreateInferenceVersion)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/api/v1/models/test-namespace/test-model/run/versions/123", bytes.NewBuffer([]byte("invalid json")))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_CreateInferenceVersion_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().CreateInferenceVersion(mock.Anything, mock.Anything).Return(errors.New("service error"))

	handler := &ModelHandler{
		model: mc,
	}
	router.POST("/api/v1/models/:namespace/:name/run/versions/:id", handler.CreateInferenceVersion)

	req := &types.CreateInferenceVersionReq{
		CommitID:       "test-commit",
		TrafficPercent: 50,
	}
	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("POST", "/api/v1/models/test-namespace/test-model/run/versions/123", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelHandler_ListInferenceVersions_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	expectedVersions := []types.ListInferenceVersionsResp{
		{
			Commit:         "commit1",
			TrafficPercent: 50,
			IsReady:        true,
		},
	}
	mc.EXPECT().ListInferenceVersions(mock.Anything, int64(123)).Return(expectedVersions, nil)

	handler := &ModelHandler{
		model: mc,
	}
	router.GET("/api/v1/models/:namespace/:name/run/versions/:id", handler.ListInferenceVersions)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/test-namespace/test-model/run/versions/123", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelHandler_ListInferenceVersions_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.GET("/api/v1/models/:namespace/:name/run/versions/:id", handler.ListInferenceVersions)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/test-namespace/test-model/run/versions/invalid", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_ListInferenceVersions_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().ListInferenceVersions(mock.Anything, int64(123)).Return(nil, errors.New("service error"))

	handler := &ModelHandler{
		model: mc,
	}
	router.GET("/api/v1/models/:namespace/:name/run/versions/:id", handler.ListInferenceVersions)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/test-namespace/test-model/run/versions/123", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelHandler_UpdateInferenceTraffic_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().UpdateInferenceVersionTraffic(mock.Anything, int64(123), mock.Anything).Return(nil)

	handler := &ModelHandler{
		model: mc,
	}
	router.PUT("/api/v1/models/:namespace/:name/run/versions/:id/traffic", handler.UpdateInferenceTraffic)

	trafficReq := []types.UpdateInferenceVersionTrafficReq{
		{
			CommitID:       "commit1",
			TrafficPercent: 60,
		},
		{
			CommitID:       "commit2",
			TrafficPercent: 40,
		},
	}
	body, _ := json.Marshal(trafficReq)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("PUT", "/api/v1/models/test-namespace/test-model/run/versions/123/traffic", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelHandler_UpdateInferenceTraffic_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.PUT("/api/v1/models/:namespace/:name/run/versions/:id/traffic", handler.UpdateInferenceTraffic)

	trafficReq := []types.UpdateInferenceVersionTrafficReq{
		{
			CommitID:       "commit1",
			TrafficPercent: 60,
		},
	}
	body, _ := json.Marshal(trafficReq)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("PUT", "/api/v1/models/test-namespace/test-model/run/versions/invalid/traffic", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_UpdateInferenceTraffic_InvalidRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.PUT("/api/v1/models/:namespace/:name/run/versions/:id/traffic", handler.UpdateInferenceTraffic)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("PUT", "/api/v1/models/test-namespace/test-model/run/versions/123/traffic", bytes.NewBuffer([]byte("invalid json")))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_UpdateInferenceTraffic_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().UpdateInferenceVersionTraffic(mock.Anything, int64(123), mock.Anything).Return(errors.New("service error"))

	handler := &ModelHandler{
		model: mc,
	}
	router.PUT("/api/v1/models/:namespace/:name/run/versions/:id/traffic", handler.UpdateInferenceTraffic)

	trafficReq := []types.UpdateInferenceVersionTrafficReq{
		{
			CommitID:       "commit1",
			TrafficPercent: 100,
		},
	}
	body, _ := json.Marshal(trafficReq)
	w := httptest.NewRecorder()
	request, _ := http.NewRequest("PUT", "/api/v1/models/test-namespace/test-model/run/versions/123/traffic", bytes.NewBuffer(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelHandler_DeleteInferenceVersion_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().DeleteInferenceVersion(mock.Anything, int64(123), "commit123").Return(nil)

	handler := &ModelHandler{
		model: mc,
	}
	router.DELETE("/api/v1/models/:namespace/:name/run/versions/:id/:commit_id", handler.DeleteInferenceVersion)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("DELETE", "/api/v1/models/test-namespace/test-model/run/versions/123/commit123", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelHandler_DeleteInferenceVersion_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{
		model: mc,
	}
	router.DELETE("/api/v1/models/:namespace/:name/run/versions/:id/:commit_id", handler.DeleteInferenceVersion)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("DELETE", "/api/v1/models/test-namespace/test-model/run/versions/invalid/commit123", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_DeleteInferenceVersion_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().DeleteInferenceVersion(mock.Anything, int64(123), "commit123").Return(errors.New("service error"))

	handler := &ModelHandler{
		model: mc,
	}
	router.DELETE("/api/v1/models/:namespace/:name/run/versions/:id/:commit_id", handler.DeleteInferenceVersion)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("DELETE", "/api/v1/models/test-namespace/test-model/run/versions/123/commit123", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelHandler_IndexV2_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().IndexV2(mock.Anything, mock.Anything, 10, 2, false).Return([]*types.Model{
		{ID: 1, Name: "model1", Path: "u/model1"},
		{ID: 2, Name: "model2", Path: "u/model2"},
	}, 100, nil)

	handler := &ModelHandler{model: mc}
	router.GET("/api/v1/models/v2", handler.IndexV2)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/v2?per=10&page=2&sort=trending", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelHandler_IndexV2_InvalidSort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{model: mc}
	router.GET("/api/v1/models/v2", handler.IndexV2)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/v2?sort=invalid_sort", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_IndexV2_InvalidSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)

	handler := &ModelHandler{model: mc}
	router.GET("/api/v1/models/v2", handler.IndexV2)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/v2?source=invalid", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestModelHandler_IndexV2_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	mc := mockcomponent.NewMockModelComponent(t)
	mc.EXPECT().IndexV2(mock.Anything, mock.Anything, 10, 1, false).Return(nil, 0, errors.New("db error"))

	handler := &ModelHandler{model: mc}
	router.GET("/api/v1/models/v2", handler.IndexV2)

	w := httptest.NewRecorder()
	request, _ := http.NewRequest("GET", "/api/v1/models/v2?per=10&page=1", nil)

	router.ServeHTTP(w, request)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}


