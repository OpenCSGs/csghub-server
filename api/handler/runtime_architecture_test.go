package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type RuntimeArchitectureTester struct {
	*GinTester
	handler *RuntimeArchitectureHandler
	mocks   struct {
		repo        *mockcomponent.MockRepoComponent
		runtimeArch *mockcomponent.MockRuntimeArchitectureComponent
	}
}

func NewRuntimeArchitectureTester(t *testing.T) *RuntimeArchitectureTester {
	tester := &RuntimeArchitectureTester{GinTester: NewGinTester()}
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)
	tester.mocks.runtimeArch = mockcomponent.NewMockRuntimeArchitectureComponent(t)

	tester.handler = &RuntimeArchitectureHandler{
		repo:        tester.mocks.repo,
		runtimeArch: tester.mocks.runtimeArch,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *RuntimeArchitectureTester) WithHandleFunc(fn func(h *RuntimeArchitectureHandler) gin.HandlerFunc) *RuntimeArchitectureTester {
	t.ginHandler = fn(t.handler)
	return t
}

func TestRuntimeArchHandler_ListByRuntimeFrameworkID(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.ListByRuntimeFrameworkID
	})

	tester.mocks.runtimeArch.EXPECT().ListByRuntimeFrameworkID(tester.ctx, int64(1)).Return([]database.RuntimeArchitecture{{ID: 1}}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, []database.RuntimeArchitecture{{ID: 1}})
}

func TestRuntimeArchHandler_UpdateArchitecture(t *testing.T) {
	tester := NewRuntimeArchitectureTester(t).WithHandleFunc(func(h *RuntimeArchitectureHandler) gin.HandlerFunc {
		return h.UpdateArchitecture
	})

	tester.mocks.runtimeArch.EXPECT().SetArchitectures(
		tester.ctx, int64(1), []string{"foo"},
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
		tester.ctx, int64(1), []string{"foo"},
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

	tester.mocks.runtimeArch.EXPECT().ScanArchitecture(
		tester.ctx, int64(1), 2, []string{"foo"},
	).Return(nil)
	tester.WithParam("id", "1").WithQuery("scan_type", "2").WithBody(t, &types.RuntimeFrameworkModels{
		Models: []string{"foo"},
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}
