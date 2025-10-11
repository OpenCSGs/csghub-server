package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type NotebookHandlerTester struct {
	*testutil.GinTester
	handler *NotebookHandler
	mocks   struct {
		component *mockcomponent.MockNotebookComponent
	}
}

func NewNotebookHandlerTester(t *testing.T) *NotebookHandlerTester {
	tester := &NotebookHandlerTester{
		GinTester: testutil.NewGinTester(),
	}
	tester.mocks.component = mockcomponent.NewMockNotebookComponent(t)
	tester.handler = &NotebookHandler{
		nc: tester.mocks.component,
	}

	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	tester.WithParam("instance", "test-instance")
	tester.WithParam("id", "1")
	tester.WithParam("type", "inference")
	return tester
}

func (t *NotebookHandlerTester) WithHandleFunc(fn func(h *NotebookHandler) gin.HandlerFunc) *NotebookHandlerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestNotebookHandler_Create(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.WithUser()

	req := &types.CreateNotebookReq{
		CurrentUser:        "u",
		ResourceID:         1,
		RuntimeFrameworkID: 1,
		DeployName:         "test-instance",
	}

	tester.mocks.component.EXPECT().CreateNotebook(tester.Ctx(), req).Return(&types.NotebookRes{ID: 123}, nil)
	tester.WithBody(t, req).Execute()
	tester.ResponseEq(t, 200, tester.OKText, types.NotebookRes{ID: 123})
}
func TestNotebookHandler_Get_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Get
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.GetNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	expectedRes := &types.NotebookRes{ID: 123, DeployName: "test-notebook"}
	tester.mocks.component.EXPECT().GetNotebook(tester.Ctx(), req).Return(expectedRes, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, expectedRes)
}

func TestNotebookHandler_Get_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Get
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Get_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Get
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.GetNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().GetNotebook(tester.Ctx(), req).Return(nil, assert.AnError)

	tester.Execute()
	tester.ResponseEqCode(t, 500)
}
func TestNotebookHandler_Start_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Start
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.StartNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().StartNotebook(tester.Ctx(), req).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestNotebookHandler_Start_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Start
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Start_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Start
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.StartNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().StartNotebook(tester.Ctx(), req).Return(assert.AnError)

	tester.Execute()
	tester.ResponseEqCode(t, 500)
}
func TestNotebookHandler_Stop_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Stop
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.StopNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().StopNotebook(tester.Ctx(), req).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestNotebookHandler_Stop_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Stop
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Stop_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Stop
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.StopNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().StopNotebook(tester.Ctx(), req).Return(assert.AnError)

	tester.Execute()
	tester.ResponseEqCode(t, 500)
}
func TestNotebookHandler_Delete_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.DeleteNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().DeleteNotebook(tester.Ctx(), req).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestNotebookHandler_Delete_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Delete_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.DeleteNotebookReq{
		CurrentUser: "u",
		ID:          123,
	}
	tester.mocks.component.EXPECT().DeleteNotebook(tester.Ctx(), req).Return(assert.AnError)

	tester.Execute()
	tester.ResponseEqCode(t, 500)
}
func TestNotebookHandler_Update_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	tester.WithParam("id", "456")

	req := &types.UpdateNotebookReq{
		ResourceID: 1,
	}
	expectedReq := &types.UpdateNotebookReq{
		ID:          456,
		ResourceID:  1,
		CurrentUser: "u",
	}
	tester.mocks.component.EXPECT().UpdateNotebook(tester.Ctx(), expectedReq).Return(nil)

	tester.WithBody(t, req).Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestNotebookHandler_Update_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	req := &types.UpdateNotebookReq{
		ResourceID: 1,
	}
	tester.WithBody(t, req).Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Update_BadJSON(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	// Send invalid JSON
	tester.WithBody(t, "").Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Update_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	req := &types.UpdateNotebookReq{
		ResourceID: 1,
	}
	expectedReq := &types.UpdateNotebookReq{
		CurrentUser: "u",
		ID:          123,
		ResourceID:  1,
	}
	tester.mocks.component.EXPECT().UpdateNotebook(tester.Ctx(), expectedReq).Return(assert.AnError)

	tester.WithBody(t, req).Execute()
	tester.ResponseEqCode(t, 500)
}

// Wakeup tests

func TestNotebookHandler_Wakeup_Success(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Wakeup
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	tester.mocks.component.EXPECT().Wakeup(tester.Ctx(), int64(123)).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestNotebookHandler_Wakeup_InvalidID(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Wakeup
	})
	tester.WithUser()
	tester.WithParam("id", "invalid")

	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestNotebookHandler_Wakeup_ErrorFromComponent(t *testing.T) {
	tester := NewNotebookHandlerTester(t)
	tester.WithHandleFunc(func(h *NotebookHandler) gin.HandlerFunc {
		return h.Wakeup
	})
	tester.WithUser()
	tester.WithParam("id", "123")

	tester.mocks.component.EXPECT().Wakeup(tester.Ctx(), int64(123)).Return(assert.AnError)

	tester.Execute()
	tester.ResponseEqCode(t, 500)
}
