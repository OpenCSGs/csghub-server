package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type MemoryTester struct {
	*testutil.GinTester
	handler *MemoryHandler
	mocks   struct {
		memory *mockcomponent.MockMemoryComponent
	}
}

func NewMemoryTester(t *testing.T) *MemoryTester {
	tester := &MemoryTester{GinTester: testutil.NewGinTester()}
	tester.mocks.memory = mockcomponent.NewMockMemoryComponent(t)
	tester.handler = &MemoryHandler{memory: tester.mocks.memory}
	return tester
}

func (t *MemoryTester) WithHandleFunc(fn func(h *MemoryHandler) gin.HandlerFunc) *MemoryTester {
	t.Handler(fn(t.handler))
	return t
}

func TestMemoryHandler_CreateProject(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.CreateProject
	})

	req := types.CreateMemoryProjectRequest{OrgID: "org", ProjectID: "proj"}
	resp := &types.MemoryProjectResponse{OrgID: "org", ProjectID: "proj", Description: "desc"}

	tester.WithBody(t, req)
	tester.mocks.memory.On("CreateProject", mock.Anything, &req).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.MemoryProjectResponse{
		OrgID:       "org",
		ProjectID:   "proj",
		Description: "desc",
	})
}

func TestMemoryHandler_CreateProject_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.CreateProject
	})

	req := types.CreateMemoryProjectRequest{OrgID: "", ProjectID: "proj"}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_GetProject(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.GetProject
	})

	req := types.GetMemoryProjectRequest{OrgID: "org", ProjectID: "proj"}
	resp := &types.MemoryProjectResponse{OrgID: "org", ProjectID: "proj", Description: "desc"}

	tester.WithBody(t, req)
	tester.mocks.memory.On("GetProject", mock.Anything, &req).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, &types.MemoryProjectResponse{
		OrgID:       "org",
		ProjectID:   "proj",
		Description: "desc",
	})
}

func TestMemoryHandler_GetProject_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.GetProject
	})

	req := types.GetMemoryProjectRequest{OrgID: "org", ProjectID: ""}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_ListProjects(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.ListProjects
	})

	resp := []*types.MemoryProjectRef{{OrgID: "org", ProjectID: "proj"}}
	tester.mocks.memory.On("ListProjects", mock.Anything).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, resp)
}

func TestMemoryHandler_DeleteProject(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.DeleteProject
	})

	req := types.DeleteMemoryProjectRequest{OrgID: "org", ProjectID: "proj"}
	tester.WithBody(t, req)
	tester.mocks.memory.On("DeleteProject", mock.Anything, &req).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, gin.H{"deleted": true})
}

func TestMemoryHandler_AddMemories(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.AddMemories
	})

	req := types.AddMemoriesRequest{
		AgentID:   "agent",
		SessionID: "session",
		OrgID:     "org",
		ProjectID: "proj",
		Types:     []types.MemoryType{types.MemoryTypeEpisodic},
		Messages: []types.MemoryMessage{
			{Content: "hello", Role: "user", UserID: "u1"},
		},
	}
	resp := &types.AddMemoriesResponse{Created: []types.MemoryMessage{{UID: "e_1", Content: "hello"}}}

	tester.WithBody(t, req)
	tester.mocks.memory.On("AddMemories", mock.Anything, &req).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, resp)
}

func TestMemoryHandler_AddMemories_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.AddMemories
	})

	req := types.AddMemoriesRequest{
		Messages: []types.MemoryMessage{{Content: " "}},
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_SearchMemories(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.SearchMemories
	})

	req := types.SearchMemoriesRequest{
		OrgID:        "org",
		ProjectID:    "proj",
		ContentQuery: "query",
	}
	resp := &types.SearchMemoriesResponse{
		Status:  0,
		Content: []types.MemoryMessage{{UID: "e_1", Content: "hello"}},
	}

	tester.WithBody(t, req)
	tester.mocks.memory.On("SearchMemories", mock.Anything, &req).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, resp)
}

func TestMemoryHandler_SearchMemories_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.SearchMemories
	})

	req := types.SearchMemoriesRequest{
		ContentQuery:  "query",
		PageSize:      10,
		PageNum:       0,
		MinSimilarity: floatPtr(1.2),
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_ListMemories(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.ListMemories
	})

	req := types.ListMemoriesRequest{
		OrgID:     "org",
		ProjectID: "proj",
		PageSize:  10,
		PageNum:   1,
	}
	resp := &types.ListMemoriesResponse{
		Status:  0,
		Content: []types.MemoryMessage{{UID: "e_1", Content: "hello"}},
	}

	tester.WithBody(t, req)
	tester.mocks.memory.On("ListMemories", mock.Anything, &req).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, resp)
}

func TestMemoryHandler_ListMemories_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.ListMemories
	})

	req := types.ListMemoriesRequest{
		Types:    []types.MemoryType{"invalid"},
		PageSize: 10,
		PageNum:  1,
	}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_DeleteMemories(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.DeleteMemories
	})

	req := types.DeleteMemoriesRequest{OrgID: "org", ProjectID: "proj", UID: "e_1"}
	tester.WithBody(t, req)
	tester.mocks.memory.On("DeleteMemories", mock.Anything, &req).Return(nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, gin.H{"deleted": true})
}

func TestMemoryHandler_DeleteMemories_Validation(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.DeleteMemories
	})

	req := types.DeleteMemoriesRequest{}
	tester.WithBody(t, req)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusBadRequest)
}

func TestMemoryHandler_Health(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.Health
	})

	resp := &types.MemoryHealthResponse{Status: "healthy", Service: "memmachine"}
	tester.mocks.memory.On("Health", mock.Anything).Return(resp, nil)

	tester.Execute()
	tester.ResponseEq(t, http.StatusOK, tester.OKText, resp)
}

func TestMemoryHandler_ErrorMapping(t *testing.T) {
	tester := NewMemoryTester(t).WithHandleFunc(func(h *MemoryHandler) gin.HandlerFunc {
		return h.GetProject
	})

	req := types.GetMemoryProjectRequest{OrgID: "org", ProjectID: "proj"}
	tester.WithBody(t, req)
	tester.mocks.memory.On("GetProject", mock.Anything, &req).Return(nil, errorx.ErrRemoteServiceFail)

	tester.Execute()
	tester.ResponseEqCode(t, http.StatusServiceUnavailable)
}

func TestMemoryHandler_NoBidiControlCharacters(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	targetPath := filepath.Join(filepath.Dir(currentFile), "memory.go")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", targetPath, err)
	}
	for i, r := range string(data) {
		if unicode.Is(unicode.Bidi_Control, r) {
			t.Fatalf("bidi control character found at index %d in %s", i, targetPath)
		}
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
