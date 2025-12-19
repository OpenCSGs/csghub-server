package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type MCPServerTester struct {
	*testutil.GinTester
	handler *MCPServerHandler
	mocks   struct {
		mcpServerComp *mockcomponent.MockMCPServerComponent
		sensitive     *mockcomponent.MockSensitiveComponent
	}
}

func NewMCPServerTester(t *testing.T) *MCPServerTester {
	tester := &MCPServerTester{GinTester: testutil.NewGinTester()}
	tester.mocks.mcpServerComp = mockcomponent.NewMockMCPServerComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)

	tester.handler = &MCPServerHandler{
		mcpComp:   tester.mocks.mcpServerComp,
		sensitive: tester.mocks.sensitive,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *MCPServerTester) WithHandleFunc(fn func(h *MCPServerHandler) gin.HandlerFunc) *MCPServerTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMCPServerHandler_Create(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Create
	})

	req := &types.CreateMCPServerReq{
		CreateRepoReq: types.CreateRepoReq{
			Namespace: "u",
			Name:      "r",
		},
	}

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.mcpServerComp.EXPECT().Create(tester.Ctx(), req).Return(&types.MCPServer{ID: 1}, nil)

	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.MCPServer{ID: 1})
}

func TestMCPServerHandler_Delete(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Delete
	})

	req := &types.UpdateMCPServerReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Namespace: "u",
			Name:      "r",
		},
	}

	tester.mocks.mcpServerComp.EXPECT().Delete(tester.Ctx(), req).Return(nil)

	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestMCPServerHandler_Update(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Update
	})

	req := &types.UpdateMCPServerReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Namespace: "u",
			Name:      "r",
		},
	}

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.mcpServerComp.EXPECT().Update(tester.Ctx(), req).Return(&types.MCPServer{ID: 1}, nil)

	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.MCPServer{ID: 1})
}

func TestMCPServerHandler_Show(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Show
	})

	tester.mocks.mcpServerComp.EXPECT().Show(tester.Ctx(), "u", "r", "", false, false).Return(&types.MCPServer{ID: 1}, nil)

	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.MCPServer{ID: 1})
}

func TestMCPServerHandler_Index(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Index
	})
	filter := new(types.RepoFilter)
	filter.Sort = "recently_update"

	tester.mocks.mcpServerComp.EXPECT().Index(tester.Ctx(), filter, 50, 1, false).Return([]*types.MCPServer{{ID: 1}}, 1, nil)

	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{"data": []*types.MCPServer{{ID: 1}}, "total": 1})
}

func TestMCPServerHandler_Properties(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Properties
	})

	req := &types.MCPPropertyFilter{
		Kind: types.MCPPropTool,
		Per:  50,
		Page: 1,
	}

	tester.mocks.mcpServerComp.EXPECT().Properties(tester.Ctx(), req).Return([]types.MCPServerProperties{{ID: 1}}, 1, nil)

	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, gin.H{"data": []types.MCPServerProperties{{ID: 1}}, "total": 1})
}

func TestMCPServerHandler_Deploy(t *testing.T) {
	tester := NewMCPServerTester(t)
	tester.WithHandleFunc(func(h *MCPServerHandler) gin.HandlerFunc {
		return h.Deploy
	})

	req := &types.DeployMCPServerReq{
		CreateRepoReq: types.CreateRepoReq{
			Namespace: "u1",
			Name:      "r1",
		},
		ResourceID: 11,
		ClusterID:  "ab45d3ba-a2ff-466e-887a-b2e5c0c070c5",
	}

	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), req).Return(true, nil)
	tester.mocks.mcpServerComp.EXPECT().Deploy(tester.Ctx(), &types.DeployMCPServerReq{
		MCPRepo: types.RepoRequest{
			Namespace: "u",
			Name:      "r",
		},
		CreateRepoReq: types.CreateRepoReq{
			Namespace:     "u1",
			Name:          "r1",
			Nickname:      "r1",
			DefaultBranch: "",
		},
		ResourceID: 11,
		ClusterID:  "ab45d3ba-a2ff-466e-887a-b2e5c0c070c5",
	}).Return(&types.Space{
		Name: "r",
	}, nil)

	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Space{
		Name: "r",
	})
}
