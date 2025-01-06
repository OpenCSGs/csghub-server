package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

type EvaluationTester struct {
	*GinTester
	handler *EvaluationHandler
	mocks   struct {
		evaluation *mockcomponent.MockEvaluationComponent
		sensitive  *mockcomponent.MockSensitiveComponent
	}
}

func NewEvaluationTester(t *testing.T) *EvaluationTester {
	tester := &EvaluationTester{GinTester: NewGinTester()}
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
	t.ginHandler = fn(t.handler)
	return t
}

func TestEvaluationHandler_Run(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.RunEvaluation
	})
	tester.RequireUser(t)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.ctx, &types.EvaluationReq{}).Return(true, nil)
	tester.mocks.evaluation.EXPECT().CreateEvaluation(tester.ctx, types.EvaluationReq{
		Username: "u",
	}).Return(&types.ArgoWorkFlowRes{ID: 1}, nil)
	tester.WithBody(t, &types.EvaluationReq{}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ArgoWorkFlowRes{ID: 1})

}

func TestEvaluationHandler_Get(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.GetEvaluation
	})
	tester.RequireUser(t)

	tester.mocks.evaluation.EXPECT().GetEvaluation(tester.ctx, types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(&types.EvaluationRes{ID: 1}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.EvaluationRes{ID: 1})

}

func TestEvaluationHandler_Delete(t *testing.T) {
	tester := NewEvaluationTester(t).WithHandleFunc(func(h *EvaluationHandler) gin.HandlerFunc {
		return h.DeleteEvaluation
	})
	tester.RequireUser(t)

	tester.mocks.evaluation.EXPECT().DeleteEvaluation(tester.ctx, types.EvaluationGetReq{
		Username: "u",
		ID:       1,
	}).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}