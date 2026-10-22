package handler

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type fakePromptVersionComponent struct {
	create func(context.Context, types.PromptVersionReq, *types.CreatePromptVersionReq) (*types.PromptVersion, error)
	list   func(context.Context, types.PromptVersionReq) ([]types.PromptVersion, error)
	get    func(context.Context, types.PromptVersionReq) (*types.PromptVersionDetail, error)
	update func(context.Context, types.PromptVersionReq, *types.UpdatePromptReq) (*types.PromptVersionDetail, error)
}

func (f *fakePromptVersionComponent) CreatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.CreatePromptVersionReq) (*types.PromptVersion, error) {
	return f.create(ctx, req, body)
}

func (f *fakePromptVersionComponent) ListPromptVersions(ctx context.Context, req types.PromptVersionReq) ([]types.PromptVersion, error) {
	return f.list(ctx, req)
}

func (f *fakePromptVersionComponent) GetPromptVersion(ctx context.Context, req types.PromptVersionReq) (*types.PromptVersionDetail, error) {
	return f.get(ctx, req)
}

func (f *fakePromptVersionComponent) UpdatePromptVersion(ctx context.Context, req types.PromptVersionReq, body *types.UpdatePromptReq) (*types.PromptVersionDetail, error) {
	return f.update(ctx, req, body)
}

func TestPromptHandler_CreatePromptVersion(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.CreatePromptVersion
	})
	tester.WithUser().WithParam("file_path", "/folder/prompt.jsonl")
	tester.handler.promptVersion = &fakePromptVersionComponent{create: func(ctx context.Context, req types.PromptVersionReq, body *types.CreatePromptVersionReq) (*types.PromptVersion, error) {
		if req.Namespace != "u" || req.Name != "r" || req.CurrentUser != "u" || req.FilePath != "folder/prompt.jsonl" {
			t.Fatalf("unexpected request: %+v", req)
		}
		if body.Version != "v1" {
			t.Fatalf("unexpected body: %+v", body)
		}
		return &types.PromptVersion{ID: 1, Version: "v1", FilePath: "folder/prompt.jsonl", Commit: "c1"}, nil
	}}
	tester.WithBody(t, &types.CreatePromptVersionReq{Version: "v1"}).Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.PromptVersion{ID: 1, Version: "v1", FilePath: "folder/prompt.jsonl", Commit: "c1"})
}

func TestPromptHandler_CreatePromptVersionConflict(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.CreatePromptVersion
	})
	tester.WithUser().WithParam("file_path", "/prompt.jsonl")
	tester.handler.promptVersion = &fakePromptVersionComponent{create: func(context.Context, types.PromptVersionReq, *types.CreatePromptVersionReq) (*types.PromptVersion, error) {
		return nil, errorx.ErrDatabaseDuplicateKey
	}}
	tester.WithBody(t, &types.CreatePromptVersionReq{Version: "v1"}).Execute()
	tester.ResponseEqCode(t, 409)
}

func TestPromptHandler_CreatePromptVersionBadRequest(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.CreatePromptVersion
	})
	tester.WithUser().WithParam("file_path", "/prompt.jsonl")
	tester.handler.promptVersion = &fakePromptVersionComponent{}
	tester.WithBody(t, &types.CreatePromptVersionReq{}).Execute()
	tester.ResponseEqCode(t, 400)
}

func TestPromptHandler_ListPromptVersions(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.ListPromptVersions
	})
	tester.WithUser().WithParam("file_path", "/folder/prompt.jsonl")
	expected := []types.PromptVersion{{ID: 1, Version: "v1", FilePath: "folder/prompt.jsonl", Commit: "c1"}}
	tester.handler.promptVersion = &fakePromptVersionComponent{list: func(ctx context.Context, req types.PromptVersionReq) ([]types.PromptVersion, error) {
		if req.Namespace != "u" || req.Name != "r" || req.CurrentUser != "u" || req.FilePath != "folder/prompt.jsonl" {
			t.Fatalf("unexpected request: %+v", req)
		}
		return expected, nil
	}}
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, expected)
}

func TestPromptHandler_GetPromptVersion(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.GetPromptVersion
	})
	tester.WithUser().WithParam("version", "v1").WithParam("file_path", "/prompt.jsonl")
	tester.handler.promptVersion = &fakePromptVersionComponent{get: func(ctx context.Context, req types.PromptVersionReq) (*types.PromptVersionDetail, error) {
		if req.Version != "v1" || req.FilePath != "prompt.jsonl" {
			t.Fatalf("unexpected request: %+v", req)
		}
		return &types.PromptVersionDetail{PromptVersion: types.PromptVersion{Version: "v1", Commit: "c1"}}, nil
	}}
	tester.Execute()
	tester.ResponseEq(t, 200, tester.OKText, &types.PromptVersionDetail{PromptVersion: types.PromptVersion{Version: "v1", Commit: "c1"}})
}

func TestPromptHandler_GetPromptVersionErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "bad request", err: errorx.ErrReqParamInvalid, code: 400},
		{name: "forbidden", err: errorx.ErrForbidden, code: 403},
		{name: "not found", err: errorx.ErrDatabaseNoRows, code: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
				return h.GetPromptVersion
			})
			tester.WithUser().WithParam("version", "v1").WithParam("file_path", "/prompt.jsonl")
			tester.handler.promptVersion = &fakePromptVersionComponent{get: func(context.Context, types.PromptVersionReq) (*types.PromptVersionDetail, error) {
				return nil, tt.err
			}}
			tester.Execute()
			tester.ResponseEqCode(t, tt.code)
		})
	}
}

func TestPromptHandler_GetPromptVersionBadParams(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.GetPromptVersion
	})
	tester.WithUser().WithParam("file_path", "/prompt.jsonl")
	tester.handler.promptVersion = &fakePromptVersionComponent{}
	tester.Execute()
	tester.ResponseEqCode(t, 400)
}

func TestPromptHandler_UpdatePromptVersion(t *testing.T) {
	tester := NewPromptTester(t).WithHandleFunc(func(h *PromptHandler) gin.HandlerFunc {
		return h.UpdatePromptVersion
	})
	tester.WithUser().WithParam("version", "v1").WithParam("file_path", "/folder/prompt.jsonl")
	body := &types.UpdatePromptReq{Prompt: types.Prompt{Title: "updated", Content: "content", Language: "zh"}}
	expected := &types.PromptVersionDetail{
		PromptVersion: types.PromptVersion{ID: 1, Version: "v1", FilePath: "folder/prompt.jsonl", Commit: "c2"},
		Prompt:        types.PromptOutput{Prompt: body.Prompt, FilePath: "folder/prompt.jsonl", CanWrite: true},
	}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), body).Return(true, nil)
	tester.handler.promptVersion = &fakePromptVersionComponent{update: func(ctx context.Context, req types.PromptVersionReq, actual *types.UpdatePromptReq) (*types.PromptVersionDetail, error) {
		if req.Namespace != "u" || req.Name != "r" || req.CurrentUser != "u" || req.Version != "v1" || req.FilePath != "folder/prompt.jsonl" {
			t.Fatalf("unexpected request: %+v", req)
		}
		if actual.Prompt.Title != "updated" {
			t.Fatalf("unexpected body: %+v", actual)
		}
		return expected, nil
	}}
	tester.WithBody(t, body).Execute()
	tester.ResponseEq(t, 200, tester.OKText, expected)
}
