package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
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

//type fakeRespWrapper struct {
//	reader io.ReadCloser
//}
//
//func (f *fakeRespWrapper) Stream(ctx context.Context) (io.ReadCloser, error) {
//	return f.reader, nil
//}
//
//func (f *fakeRespWrapper) DoRaw(ctx context.Context) ([]byte, error) {
//	return io.ReadAll(f.reader)
//}
//
//type streamReadCloser struct {
//	buffs [][]byte // test, miss lock
//	err   error    // set error
//}
//
//func (s *streamReadCloser) Read(p []byte) (n int, err error) {
//	if s.err != nil {
//		return 0, s.err
//	}
//
//	if len(s.buffs) == 0 {
//		return 0, io.EOF
//	}
//
//	buff := s.buffs[0]
//	if len(p) >= len(buff) {
//		n = copy(p, buff)
//		copy(s.buffs, s.buffs[1:])
//		s.buffs = s.buffs[:len(s.buffs)-1]
//	} else {
//		n = copy(p, buff)
//		copy(buff, buff[n:])
//		buff = buff[:len(buff)-n]
//		s.buffs[0] = buff
//	}
//
//	return n, nil
//}
//func (s *streamReadCloser) Close() error {
//	s.err = io.EOF
//	return nil
//}

func Test_readPodLogsFromCluster(t *testing.T) {
	client := fake.NewClientset()

	status := corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	var setErr error = nil
	client.PrependReactor("get", "pods", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "log" {
			// TODO Based on the code submitted in https://github.com/kubernetes/kubernetes/pull/91485,
			// the GetLogs() method does not support custom objects; instead, it creates an internal fakerest.RESTClient.
			// Tests for this functionality will be added once Kubernetes provides native support.
			return false, nil, nil
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-pod",
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "app",
						Image: "image-test",
					},
				},
			},
			Status: status,
		}
		return true, pod, setErr
	})

	gin.SetMode(gin.TestMode)

	hfn := func() (*httptest.ResponseRecorder, *gin.Context) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = &http.Request{}
		return w, c
	}

	w, c := hfn()
	h := &K8sHandler{
		k8sNameSpace: "test-ns",
	}
	testCluster := &cluster.Cluster{
		CID:           "stream-log",
		ID:            "test",
		Client:        client,
		KnativeClient: knativefake.NewSimpleClientset(),
	}

	h.readPodLogsFromCluster(c, testCluster, "pod-test", "svc-test")
	require.Equal(t, http.StatusOK, w.Code)

	// test pod PodPending
	status = corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionFalse,
			},
		},
	}
	w, c = hfn()
	h.readPodLogsFromCluster(c, testCluster, "pod-test", "svc-test")
	require.Equal(t, http.StatusBadRequest, w.Code)

	// set error
	setErr = errors.New("test error")
	w, c = hfn()
	h.readPodLogsFromCluster(c, testCluster, "pod-test", "svc-test")
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
