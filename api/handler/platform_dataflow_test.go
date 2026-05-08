package handler

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type PlatformDataflowTester struct {
	*testutil.GinTester
	handler  *PlatformDataflowHandler
	mockComp *mockcomponent.MockPlatformDataflowComponent
}

func NewPlatformDataflowTester(t *testing.T) *PlatformDataflowTester {
	tester := &PlatformDataflowTester{GinTester: testutil.NewGinTester()}
	tester.mockComp = mockcomponent.NewMockPlatformDataflowComponent(t)

	tester.handler = &PlatformDataflowHandler{
		component: tester.mockComp,
	}
	tester.WithParam("uuid", "test-ns-uuid")
	return tester
}

func (t *PlatformDataflowTester) WithHandleFunc(fn func(h *PlatformDataflowHandler) gin.HandlerFunc) *PlatformDataflowTester {
	t.Handler(fn(t.handler))
	return t
}

func (t *PlatformDataflowTester) WithUserAndUUID() *PlatformDataflowTester {
	t.Gctx().Set("currentUser", "testuser")
	t.Gctx().Set("currentUserUUID", "test-user-uuid")
	return t
}

func TestPlatformDataflowHandler_CreateJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		req := &types.DataflowArgoJobReq{
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}
		resp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "argo-task-1",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Pending",
		}

		tester.mockComp.EXPECT().CreateJob(tester.Ctx(), &types.DataflowArgoJobReq{
			OpUserUUID:  "test-user-uuid",
			Username:    "testuser",
			NSUUID:      "test-ns-uuid",
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}).Return(resp, nil)

		tester.WithBody(t, req).Execute()
		tester.ResponseEq(t, 201, "Created", resp)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("uuid", "")

		req := &types.DataflowArgoJobReq{}
		tester.WithBody(t, req).Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("invalid_request_body", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		tester.Gctx().Request.Body = io.NopCloser(bytes.NewBuffer([]byte("{invalid json")))
		tester.Gctx().Request.Header = map[string][]string{"Content-Type": {"application/json"}}

		tester.Execute()

		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.CreateJob
		})
		tester.WithUserAndUUID()

		req := &types.DataflowArgoJobReq{
			ResourceId:  1,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "python script.py",
			DagTasks:    []types.ArgoDagTask{{ID: "task1", Name: "task1", Template: "main"}},
		}

		tester.mockComp.EXPECT().CreateJob(tester.Ctx(), mock.Anything).Return(nil, errors.New("some error"))

		tester.WithBody(t, req).Execute()
		tester.ResponseEqCode(t, 500)
	})
}

func TestPlatformDataflowHandler_DeleteJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().DeleteJob(tester.Ctx(), &types.DataflowDeleteReq{
			OpUserUUID: "test-user-uuid",
			Username:   "testuser",
			NSUUID:     "test-ns-uuid",
			ArgoTaskID: "argo-task-1",
		}).Return(nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, nil)
	})

	t.Run("missing_task_id", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")
		tester.WithParam("uuid", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.DeleteJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().DeleteJob(tester.Ctx(), mock.Anything).Return(errors.New("some error"))

		tester.Execute()
		tester.ResponseEqCode(t, 500)
	})
}

func TestPlatformDataflowHandler_GetJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		resp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "argo-task-1",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Running",
		}

		tester.mockComp.EXPECT().GetJob(tester.Ctx(), &types.DataflowArgoJobReq{
			OpUserUUID: "test-user-uuid",
			Username:   "testuser",
			NSUUID:     "test-ns-uuid",
			ArgoTaskID: "argo-task-1",
		}).Return(resp, nil)

		tester.Execute()
		tester.ResponseEq(t, 200, tester.OKText, resp)
	})

	t.Run("missing_task_id", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("missing_ns_uuid", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")
		tester.WithParam("uuid", "")

		tester.Execute()
		tester.ResponseEqCode(t, 400)
	})

	t.Run("component_error", func(t *testing.T) {
		tester := NewPlatformDataflowTester(t).WithHandleFunc(func(h *PlatformDataflowHandler) gin.HandlerFunc {
			return h.GetJob
		})
		tester.WithUserAndUUID()
		tester.WithParam("task_id", "argo-task-1")

		tester.mockComp.EXPECT().GetJob(tester.Ctx(), mock.Anything).Return(nil, errors.New("some error"))

		tester.Execute()
		tester.ResponseEqCode(t, 500)
	})
}
