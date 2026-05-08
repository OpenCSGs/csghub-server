package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type DataflowTester struct {
	*testutil.GinTester
	handler *DataflowHandler
	mocks   struct {
		dfComp *mockcom.MockDataflowComponent
	}
}

func (t *DataflowTester) WithHandleFunc(fn func(h *DataflowHandler) gin.HandlerFunc) *DataflowTester {
	t.Handler(fn(t.handler))
	return t
}

func NewDataflowTester(t *testing.T) *DataflowTester {
	tester := &DataflowTester{GinTester: testutil.NewGinTester()}
	tester.mocks.dfComp = mockcom.NewMockDataflowComponent(t)
	tester.handler = &DataflowHandler{
		dfc: tester.mocks.dfComp,
	}
	return tester
}

func TestDataflowHandler_CreateDataflowWorkflow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.CreateDataflowWorkflow
		})

		req := types.DataflowArgoJobReq{
			ClusterID:   "test-cluster",
			ArgoTaskID:  "df-task-1",
			JobID:       "df-job-1",
			JobName:     "test-job",
			ResourceId:  100,
			StorageSize: "10Gi",
			Entrypoint:  "main",
			Template: types.ArgoFlowTemplate{
				Name:  "echo",
				Image: "alpine:latest",
			},
			DagTasks: []types.ArgoDagTask{
				{ID: "task-1", Name: "task1", Template: "echo"},
			},
		}
		resp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "df-task-1",
			JobID:      "df-job-1",
			JobName:    "test-job",
			Status:     "Pending",
		}

		tester.mocks.dfComp.EXPECT().CreateWorkflow(mock.Anything, &req).Return(resp, nil)
		tester.WithBody(t, req)
		tester.Execute()

		assert.Equal(t, http.StatusOK, tester.Response().Code)

		var actual types.DataflowArgoJobResp
		err := json.Unmarshal(tester.Response().Body.Bytes(), &actual)
		assert.NoError(t, err)
		assert.Equal(t, resp.ID, actual.ID)
		assert.Equal(t, resp.ArgoTaskID, actual.ArgoTaskID)
	})

	t.Run("bad request", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.CreateDataflowWorkflow
		})

		tester.WithBody(t, "invalid json will be parsed differently")

		tester.Execute()

		assert.Equal(t, http.StatusBadRequest, tester.Response().Code)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.CreateDataflowWorkflow
		})

		req := types.DataflowArgoJobReq{
			ClusterID:   "test-cluster",
			ResourceId:  100,
			JobID:       "df-job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "main",
			Template: types.ArgoFlowTemplate{
				Name:  "echo",
				Image: "alpine:latest",
			},
			DagTasks: []types.ArgoDagTask{
				{ID: "task-1", Name: "task1", Template: "echo"},
			},
		}

		tester.mocks.dfComp.EXPECT().CreateWorkflow(mock.Anything, &req).Return(nil, errors.New("creation failed"))
		tester.WithBody(t, req)
		tester.Execute()

		assert.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})
}

func TestDataflowHandler_GetDataflowStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.GetDataflowStatus
		})

		resp := &types.DataflowArgoJobResp{
			ArgoTaskID: "df-task-1",
			JobID:      "df-job-1",
			JobName:    "test-job",
			Status:     "Running",
		}

		tester.mocks.dfComp.EXPECT().GetStatus(mock.Anything, &types.DataflowArgoReq{
			ArgoTaskID: "df-task-1",
			ClusterID:  "cluster-1",
		}).Return(resp, nil)

		tester.WithParam("task_id", "df-task-1")
		tester.WithQuery("cluster_id", "cluster-1")
		tester.Execute()

		assert.Equal(t, http.StatusOK, tester.Response().Code)

		var actual types.DataflowArgoJobResp
		err := json.Unmarshal(tester.Response().Body.Bytes(), &actual)
		assert.NoError(t, err)
		assert.Equal(t, resp.Status, actual.Status)
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.GetDataflowStatus
		})

		tester.mocks.dfComp.EXPECT().GetStatus(mock.Anything, &types.DataflowArgoReq{
			ArgoTaskID: "unknown-task",
			ClusterID:  "cluster-1",
		}).Return(nil, errors.New("workflow not found"))

		tester.WithParam("task_id", "unknown-task")
		tester.WithQuery("cluster_id", "cluster-1")
		tester.Execute()

		assert.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})
}

func TestDataflowHandler_DeleteDataflowWorkflow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.DeleteDataflowWorkflow
		})

		tester.mocks.dfComp.EXPECT().DeleteWorkflow(mock.Anything, &types.DataflowArgoReq{
			ArgoTaskID: "df-task-1",
			ClusterID:  "cluster-1",
		}).Return(nil)

		tester.WithParam("task_id", "df-task-1")
		tester.WithQuery("cluster_id", "cluster-1")
		tester.Execute()

		assert.Equal(t, http.StatusOK, tester.Response().Code)

		var body map[string]string
		err := json.Unmarshal(tester.Response().Body.Bytes(), &body)
		assert.NoError(t, err)
		assert.Equal(t, "dataflow workflow deleted successfully", body["message"])
	})

	t.Run("component error", func(t *testing.T) {
		tester := NewDataflowTester(t).WithHandleFunc(func(h *DataflowHandler) gin.HandlerFunc {
			return h.DeleteDataflowWorkflow
		})

		tester.mocks.dfComp.EXPECT().DeleteWorkflow(mock.Anything, &types.DataflowArgoReq{
			ArgoTaskID: "unknown-task",
			ClusterID:  "cluster-1",
		}).Return(errors.New("delete failed"))

		tester.WithParam("task_id", "unknown-task")
		tester.WithQuery("cluster_id", "cluster-1")
		tester.Execute()

		assert.Equal(t, http.StatusInternalServerError, tester.Response().Code)
	})
}
