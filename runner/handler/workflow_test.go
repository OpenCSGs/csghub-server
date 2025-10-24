package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/runner/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type WorkFlowTester struct {
	*testutil.GinTester
	handler *ArgoHandler
	mocks   struct {
		wfComp *mockcom.MockWorkFlowComponent
	}
}

func (t *WorkFlowTester) WithHandleFunc(fn func(h *ArgoHandler) gin.HandlerFunc) *WorkFlowTester {
	t.Handler(fn(t.handler))
	return t
}

func NewWorkflowTester(t *testing.T) *WorkFlowTester {
	tester := &WorkFlowTester{GinTester: testutil.NewGinTester()}
	tester.mocks.wfComp = mockcom.NewMockWorkFlowComponent(t)

	tester.handler = &ArgoHandler{
		wfc: tester.mocks.wfComp,
	}
	tester.WithParam("namespace", "testInternalId")
	tester.WithParam("name", "testUserId")
	return tester
}

func TestWorkflowHandler_DeleteWorkflow(t *testing.T) {
	tester := NewWorkflowTester(t).WithHandleFunc(func(h *ArgoHandler) gin.HandlerFunc {
		return h.DeleteWorkflow
	})

	tester.mocks.wfComp.EXPECT().DeleteWorkflow(mock.Anything, int64(1), "test").Return(nil)

	tester.WithParam("id", "1")
	tester.WithBody(t, types.ArgoWorkFlowDeleteReq{
		ID:       1,
		Username: "test",
	})

	tester.Execute()

	assert.Equal(t, http.StatusOK, tester.Response().Code)
}

func TestWorkflowHandler_GetWorkflow(t *testing.T) {

	tester := NewWorkflowTester(t).WithHandleFunc(func(h *ArgoHandler) gin.HandlerFunc {
		return h.GetWorkflow
	})
	tester.WithParam("id", "1")
	tester.WithBody(t, types.ArgoWorkFlowGetReq{
		Username: "test",
	})
	tester.mocks.wfComp.EXPECT().GetWorkflow(mock.Anything, int64(1), "test").Return(&database.ArgoWorkflow{}, nil)

	tester.Execute()

	assert.Equal(t, http.StatusOK, tester.Response().Code)
}
