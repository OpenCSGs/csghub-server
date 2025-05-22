package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type MonitorHandlerTester struct {
	*testutil.GinTester
	handler *MonitorHandler
	mocks   struct {
		component *mockcomponent.MockMonitorComponent
	}
}

func NewMonitorHandlerTester(t *testing.T) *MonitorHandlerTester {
	tester := &MonitorHandlerTester{
		GinTester: testutil.NewGinTester(),
	}
	tester.mocks.component = mockcomponent.NewMockMonitorComponent(t)
	tester.handler = &MonitorHandler{
		monitor: tester.mocks.component,
	}

	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	tester.WithParam("instance", "test-instance")
	tester.WithParam("id", "1")
	return tester
}

func (t *MonitorHandlerTester) WithHandleFunc(fn func(h *MonitorHandler) gin.HandlerFunc) *MonitorHandlerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMonitorHandler_CPUUsage(t *testing.T) {
	tester := NewMonitorHandlerTester(t)
	tester.WithHandleFunc(func(h *MonitorHandler) gin.HandlerFunc {
		return h.CPUUsage
	})

	req := &types.MonitorReq{
		CurrentUser:  "",
		Namespace:    "u",
		Name:         "r",
		RepoType:     "",
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
		TimeRange:    "1m",
	}

	tester.mocks.component.EXPECT().CPUUsage(tester.Ctx(), req).Return(&types.MonitorCPUResp{}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.MonitorCPUResp{})
}

func TestMonitorHandler_MemoryUsage(t *testing.T) {
	tester := NewMonitorHandlerTester(t)
	tester.WithHandleFunc(func(h *MonitorHandler) gin.HandlerFunc {
		return h.MemoryUsage
	})

	req := &types.MonitorReq{
		CurrentUser:  "",
		Namespace:    "u",
		Name:         "r",
		RepoType:     "",
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
		TimeRange:    "1m",
	}

	tester.mocks.component.EXPECT().MemoryUsage(tester.Ctx(), req).Return(&types.MonitorMemoryResp{}, nil)

	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.MonitorMemoryResp{})
}

func TestMonitorHandler_RequestCount(t *testing.T) {
	tester := NewMonitorHandlerTester(t)
	tester.WithHandleFunc(func(h *MonitorHandler) gin.HandlerFunc {
		return h.RequestCount
	})

	req := &types.MonitorReq{
		CurrentUser:  "",
		Namespace:    "u",
		Name:         "r",
		RepoType:     "",
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
		TimeRange:    "1m",
	}

	tester.mocks.component.EXPECT().RequestCount(tester.Ctx(), req).Return(&types.MonitorRequestCountResp{}, nil)
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.MonitorRequestCountResp{})
}

func TestMonitorHandler_RequestLatency(t *testing.T) {
	tester := NewMonitorHandlerTester(t)
	tester.WithHandleFunc(func(h *MonitorHandler) gin.HandlerFunc {
		return h.RequestLatency
	})

	req := &types.MonitorReq{
		CurrentUser:  "",
		Namespace:    "u",
		Name:         "r",
		RepoType:     "",
		DeployID:     1,
		Instance:     "test-instance",
		LastDuration: "30m",
		TimeRange:    "1m",
	}

	tester.mocks.component.EXPECT().RequestLatency(tester.Ctx(), req).Return(&types.MonitorRequestLatencyResp{}, nil)
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.MonitorRequestLatencyResp{})
}
