package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	temporal_mock "go.temporal.io/sdk/mocks"
	workflow_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type RuntimeArchitectureTester struct {
	*testutil.GinTester
	handler *RuntimeArchitectureHandler
	mocks   struct {
		repo        *mockcomponent.MockRepoComponent
		runtimeArch *mockcomponent.MockRuntimeArchitectureComponent
		workflow    *workflow_mock.MockClient
	}
}

func NewRuntimeArchitectureTester(t *testing.T) *RuntimeArchitectureTester {
	tester := &RuntimeArchitectureTester{GinTester: testutil.NewGinTester()}
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)
	tester.mocks.runtimeArch = mockcomponent.NewMockRuntimeArchitectureComponent(t)
	tester.mocks.workflow = workflow_mock.NewMockClient(t)

	tester.handler = &RuntimeArchitectureHandler{
		repo:           tester.mocks.repo,
		runtimeArch:    tester.mocks.runtimeArch,
		temporalClient: tester.mocks.workflow,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *RuntimeArchitectureTester) WithHandleFunc(fn func(h *RuntimeArchitectureHandler) gin.HandlerFunc) *RuntimeArchitectureTester {
	t.Handler(fn(t.handler))
	return t
}

func TestRuntimeArchHandler_ListByRuntimeFrameworkID(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.ListByRuntimeFrameworkID
	})

	tester.mocks.runtimeArch.EXPECT().ListByRuntimeFrameworkID(tester.Ctx(), int64(1)).Return([]database.RuntimeArchitecture{{ID: 1}}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.RuntimeArchitecture{{ID: 1}})
}

func TestRuntimeArchHandler_UpdateArchitecture(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.UpdateArchitecture
	})

	tester.mocks.runtimeArch.EXPECT().SetArchitectures(
		tester.Ctx(), int64(1), []string{"foo"},
	).Return([]string{"bar"}, nil)
	tester.WithParam("id", "1").WithBody(t, &types.RuntimeArchitecture{
		Architectures: []string{"foo"},
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, []string{"bar"})
}

func TestRuntimeArchHandler_DeleteArchitecture(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.DeleteArchitecture
	})

	tester.mocks.runtimeArch.EXPECT().DeleteArchitectures(
		tester.Ctx(), int64(1), []string{"foo"},
	).Return([]string{"bar"}, nil)
	tester.WithParam("id", "1").WithBody(t, &types.RuntimeArchitecture{
		Architectures: []string{"foo"},
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, []string{"bar"})
}

func TestRuntimeArchHandler_ScanArchitecture(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.ScanArchitecture
	})

	runMock := &temporal_mock.WorkflowRun{}
	runMock.On("GetID").Return("id")
	tester.mocks.workflow.EXPECT().ExecuteWorkflow(
		tester.Ctx(), client.StartWorkflowOptions{
			TaskQueue: workflow.HandlePushQueueName,
		}, mock.Anything,
		mock.Anything,
	).Return(
		runMock, nil,
	)
	tester.WithParam("id", "1").WithQuery("scan_type", "2").WithQuery("task", string(types.TextGeneration)).WithBody(t, &types.RuntimeFrameworkModels{
		Models: []string{"foo"},
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
