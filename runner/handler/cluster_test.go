package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGetClusterInfoByID_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/clusters/c1", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "c1"}}

	cluster := mockcomponent.NewMockClusterComponent(t)

	h := &ClusterHandler{clusterComponent: cluster}
	cluster.EXPECT().ByClusterID(mock.Anything, "c1").Return(database.ClusterInfo{
		ClusterID:    "c1",
		Region:       "region1",
		Zone:         "zone1",
		Provider:     "provider1",
		StorageClass: "standard",
	}, nil)
	cluster.EXPECT().GetResourceByID(mock.Anything, "c1").Return(types.StatusClusterWide, map[string]types.NodeResourceInfo{
		"node1": {
			NodeHardware: types.NodeHardware{
				TotalCPU:     4,
				AvailableCPU: 2,
				TotalMem:     8192,
				AvailableMem: 4096,
				TotalXPU:     1,
				AvailableXPU: 0,
			},
		},
	}, nil)
	h.GetClusterInfoByID(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp types.ClusterRes
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.ClusterID != "c1" {
		t.Fatalf("expected cluster id c1, got %s", resp.ClusterID)
	}
	if len(resp.Resources) != 1 {
		t.Fatalf("expected 1 resource entry, got %d", len(resp.Resources))
	}
	if resp.ResourceStatus != types.StatusClusterWide {
		t.Fatalf("unexpected resource status: %v", resp.ResourceStatus)
	}
}

func TestGetClusterInfoByID_GetResourceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/clusters/c2", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "c2"}}

	cluster := mockcomponent.NewMockClusterComponent(t)
	cluster.EXPECT().ByClusterID(mock.Anything, "c2").Return(database.ClusterInfo{
		ClusterID:    "c2",
		Region:       "region2",
		Zone:         "zone2",
		Provider:     "provider2",
		StorageClass: "standard",
	}, nil)
	cluster.EXPECT().GetResourceByID(mock.Anything, "c2").Return(types.StatusUncertain, nil, errors.New("cluster not found"))
	h := &ClusterHandler{clusterComponent: cluster}
	h.GetClusterInfoByID(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body: %s", w.Code, w.Body.String())
	}
}
