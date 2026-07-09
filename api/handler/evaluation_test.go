package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type EvaluationTester struct {
	*testutil.GinTester
	handler *EvaluationHandler
	mocks   struct {
		evaluation *mockcomponent.MockEvaluationComponent
		sensitive  *mockcomponent.MockSensitiveComponent
	}
}

func NewEvaluationTester(t *testing.T) *EvaluationTester {
	tester := &EvaluationTester{GinTester: testutil.NewGinTester()}
	tester.mocks.evaluation = mockcomponent.NewMockEvaluationComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)

	tester.handler = &EvaluationHandler{
		evaluation: tester.mocks.evaluation,
		sensitive:  tester.mocks.sensitive,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *EvaluationTester) WithHandleFunc(fn func(h *EvaluationHandler) gin.HandlerFunc) *EvaluationTester {
	t.Handler(fn(t.handler))
	return t
}

func TestEvaluationHandler_Run(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.RunEvaluation
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.EvaluationReq{}).Return(true, nil)
	tester.mocks.evaluation.EXPECT().CreateEvaluation(tester.Ctx(), types.EvaluationReq{
		Username:       "u",
		OwnerNamespace: "u",
	}).Return(&types.ArgoWorkFlowRes{ID: 1}, nil)
	tester.WithBody(t, &types.EvaluationReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ArgoWorkFlowRes{ID: 1})

}

func TestEvaluationHandler_RunClawEvaluation(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.RunEvaluation
	})
	tester.WithUser()

	body := &types.EvaluationReq{
		TaskName:           "claw-test",
		RuntimeFrameworkId: 1,
		ResourceId:         1,
		Model:              "gpt-4",
		BaseURL:            "https://api.example.com/v1",
		ApiKey:             "sk-test",
	}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), body).Return(true, nil)
	tester.mocks.evaluation.EXPECT().CreateEvaluation(tester.Ctx(), types.EvaluationReq{
		Username:           "u",
		OwnerNamespace:     "u",
		TaskName:           "claw-test",
		RuntimeFrameworkId: 1,
		ResourceId:         1,
		Model:              "gpt-4",
		BaseURL:            "https://api.example.com/v1",
		ApiKey:             "sk-test",
	}).Return(&types.ArgoWorkFlowRes{ID: 1}, nil)
	tester.WithBody(t, body).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ArgoWorkFlowRes{ID: 1})
}

func TestEvaluationHandler_Get(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetEvaluation
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().GetEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(&types.EvaluationRes{ID: 1}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.EvaluationRes{ID: 1})

}

func TestEvaluationHandler_GetClawEvaluation(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetEvaluation
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().GetEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(&types.EvaluationRes{ID: 1, TaskType: types.TaskTypeClawEval}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.EvaluationRes{ID: 1, TaskType: types.TaskTypeClawEval})
}

func TestEvaluationHandler_GetClawEvaluationByTaskID(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetEvaluationByTaskID
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().GetEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		TaskID:   "task-123",
	}).Return(&types.EvaluationRes{ID: 123, TaskId: "task-123", TaskType: types.TaskTypeClawEval}, nil)
	tester.WithParam("task_id", "task-123").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.EvaluationRes{ID: 123, TaskId: "task-123", TaskType: types.TaskTypeClawEval})
}

func TestEvaluationHandler_GetForbidden(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetEvaluation
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().GetEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(nil, errorx.ErrForbidden)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEqCode(t, 403)
}

func TestEvaluationHandler_Delete(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.DeleteEvaluation
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().DeleteEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestEvaluationHandler_DeleteForbidden(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.DeleteEvaluation
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().DeleteEvaluation(tester.Ctx(), types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(errorx.ErrForbidden)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEqCode(t, 403)
}

func TestEvaluationHandler_GetLogs(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetLogs
	})
	tester.WithUser()

	tester.mocks.evaluation.EXPECT().ReadJobLogsNonStream(tester.Ctx(), types.EvaluationLogReq{
		CurrentUser: "u",
		ID:          1,
	}).Return("log line", nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, "log line")
}

func TestEvaluationHandler_GetLogs_TaskID(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetLogs
	})
	tester.WithUser()
	tester.WithParam("id", "task-123")

	tester.mocks.evaluation.EXPECT().ReadJobLogsNonStream(tester.Ctx(), types.EvaluationLogReq{
		CurrentUser: "u",
		TaskID:      "task-123",
	}).Return("log line", nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, "log line")
}

func TestEvaluationHandler_GetLogs_PermissionDenied(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetLogs
	})
	tester.WithUser()
	tester.WithParam("id", "1")

	tester.mocks.evaluation.EXPECT().ReadJobLogsNonStream(tester.Ctx(), types.EvaluationLogReq{
		CurrentUser: "u",
		ID:          1,
	}).Return("", errorx.ErrForbidden)
	tester.Execute()

	tester.ResponseEqCode(t, 403)
}

func TestEvaluationHandler_GetLogs_StreamClosesOnEOF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runCh := make(chan string)
	close(runCh)
	buildCh := make(chan string)
	close(buildCh)

	mockEval := mockcomponent.NewMockEvaluationComponent(t)
	mockEval.EXPECT().ReadJobLogsInStream(mock.Anything, types.EvaluationLogReq{
		CurrentUser: "u",
		ID:          1,
	}).Return(deploy.NewMultiLogReader(buildCh, runCh), nil)

	h := &EvaluationHandler{evaluation: mockEval}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/evaluations/1/logs?stream=true", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("currentUser", "u")

	h.readLogInStream(c, types.EvaluationLogReq{CurrentUser: "u", ID: 1})

	require.Equal(t, http.StatusOK, w.Code)
}

func TestEvaluationHandler_GetLogs_InvalidID(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetLogs
	})
	tester.WithUser()
	tester.WithParam("id", "")

	tester.Execute()

	tester.ResponseEqCode(t, 400)
}
