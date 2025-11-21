package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type FinetuneTester struct {
	*testutil.GinTester
	handler *FinetuneHandler
	mocks   struct {
		finetune  *mockcomponent.MockFinetuneComponent
		sensitive *mockcomponent.MockSensitiveComponent
		agent     *mockcomponent.MockAgentComponent
	}
}

func NewFinetuneTester(t *testing.T) *FinetuneTester {
	tester := &FinetuneTester{GinTester: testutil.NewGinTester()}
	tester.mocks.finetune = mockcomponent.NewMockFinetuneComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.agent = mockcomponent.NewMockAgentComponent(t)
	tester.handler = &FinetuneHandler{
		ftComp:         tester.mocks.finetune,
		sensitive:      tester.mocks.sensitive,
		agentComponent: tester.mocks.agent,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *FinetuneTester) WithHandleFunc(fn func(h *FinetuneHandler) gin.HandlerFunc) *FinetuneTester {
	t.Handler(fn(t.handler))
	return t
}

func TestFinetuneHandler_Run(t *testing.T) {
	tester := NewFinetuneTester(t).WithHandleFunc(func(h *FinetuneHandler) gin.HandlerFunc {
		return h.RunFinetuneJob
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.FinetuneReq{
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
		Agent:              "{\"type\":\"code\",\"id\":\"123\",\"request_id\":\"123\"}",
	}).Return(true, nil)
	tester.mocks.finetune.EXPECT().CreateFinetuneJob(tester.Ctx(), types.FinetuneReq{
		Username:           "u",
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
		LearningRate:       0.0001,
		Agent:              "{\"type\":\"code\",\"id\":\"123\",\"request_id\":\"123\"}",
	}).Return(&types.ArgoWorkFlowRes{ID: 1, TaskId: "task-123"}, nil)

	tester.mocks.agent.EXPECT().CreateTaskIfInstanceExists(tester.Ctx(), mock.MatchedBy(func(req *types.AgentInstanceTaskReq) bool {
		return req.TaskID == "task-123" &&
			req.Agent == "{\"type\":\"code\",\"id\":\"123\",\"request_id\":\"123\"}" &&
			req.Type == types.AgentTaskTypeFinetuneJob &&
			req.Username == "u"
	})).Return(nil)

	tester.WithBody(t, &types.FinetuneReq{
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
		Agent:              "{\"type\":\"code\",\"id\":\"123\",\"request_id\":\"123\"}",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ArgoWorkFlowRes{ID: 1, TaskId: "task-123"})
}

func TestFinetuneHandler_Run_NoAgent(t *testing.T) {
	tester := NewFinetuneTester(t).WithHandleFunc(func(h *FinetuneHandler) gin.HandlerFunc {
		return h.RunFinetuneJob
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.FinetuneReq{
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
	}).Return(true, nil)
	tester.mocks.finetune.EXPECT().CreateFinetuneJob(tester.Ctx(), types.FinetuneReq{
		Username:           "u",
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
		LearningRate:       0.0001,
	}).Return(&types.ArgoWorkFlowRes{ID: 1, TaskId: "task-123"}, nil)

	tester.WithBody(t, &types.FinetuneReq{
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "u/m",
		DatasetId:          "u/d",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ArgoWorkFlowRes{ID: 1, TaskId: "task-123"})
}

func TestFinetuneHandler_Get(t *testing.T) {
	tester := NewFinetuneTester(t).WithHandleFunc(func(h *FinetuneHandler) gin.HandlerFunc {
		return h.GetFinetuneJob
	})
	tester.WithUser()

	tester.mocks.finetune.EXPECT().GetFinetuneJob(tester.Ctx(), types.FinetineGetReq{
		Username: "u",
		ID:       1,
	}).Return(&types.FinetuneRes{ID: 1}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.FinetuneRes{ID: 1})

}

func TestFinetuneHandler_Delete(t *testing.T) {
	tester := NewFinetuneTester(t).WithHandleFunc(func(h *FinetuneHandler) gin.HandlerFunc {
		return h.DeleteFinetuneJob
	})
	tester.WithUser()

	tester.mocks.finetune.EXPECT().DeleteFinetuneJob(tester.Ctx(), types.ArgoWorkFlowDeleteReq{
		Username: "u",
		ID:       1,
	}).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestFinetuneHandler_ReadLogNonStream_Success(t *testing.T) {
	tester := NewFinetuneTester(t).WithHandleFunc(func(h *FinetuneHandler) gin.HandlerFunc {
		return func(c *gin.Context) {
			req := types.FinetuneLogReq{
				CurrentUser: "u",
				ID:          1,
				Since:       "1hour",
			}
			h.readLogNonStream(c, req)
		}
	})
	tester.WithUser()

	logs := "mock logs content"
	tester.mocks.finetune.EXPECT().ReadJobLogsNonStream(tester.Ctx(), types.FinetuneLogReq{
		CurrentUser: "u",
		ID:          1,
		Since:       "1hour",
	}).Return(logs, nil)

	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, logs)
}
