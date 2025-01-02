package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/common/types"
)

type SpaceTester struct {
	*GinTester
	handler *SpaceHandler
	mocks   struct {
		space     *mockcomponent.MockSpaceComponent
		sensitive *mockcomponent.MockSensitiveComponent
		repo      *mockcomponent.MockRepoComponent
	}
}

func NewSpaceTester(t *testing.T) *SpaceTester {
	tester := &SpaceTester{GinTester: NewGinTester()}
	tester.mocks.space = mockcomponent.NewMockSpaceComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.repo = mockcomponent.NewMockRepoComponent(t)

	tester.handler = &SpaceHandler{
		space:     tester.mocks.space,
		sensitive: tester.mocks.sensitive,
		repo:      tester.mocks.repo,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *SpaceTester) WithHandleFunc(fn func(h *SpaceHandler) gin.HandlerFunc) *SpaceTester {
	t.ginHandler = fn(t.handler)
	return t
}

func TestSpaceHandler_Index(t *testing.T) {
	cases := []struct {
		sort   string
		source string
		error  bool
	}{
		{"most_download", "local", false},
		{"foo", "local", true},
		{"most_download", "bar", true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {

			tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
				return h.Index
			})

			if !c.error {
				tester.mocks.space.EXPECT().Index(tester.ctx, &types.RepoFilter{
					Search: "foo",
					Sort:   c.sort,
					Source: c.source,
				}, 10, 1).Return([]types.Space{
					{Name: "cc"},
				}, 100, nil)
			}

			tester.AddPagination(1, 10).WithQuery("search", "foo").
				WithQuery("sort", c.sort).
				WithQuery("source", c.source).Execute()

			if c.error {
				require.Equal(t, 400, tester.response.Code)
			} else {
				tester.ResponseEqSimple(t, 200, gin.H{
					"data":  []types.Space{{Name: "cc"}},
					"total": 100,
				})
			}
		})
	}

}

func TestSpaceHandler_Show(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Show
	})

	tester.WithUser()
	tester.mocks.space.EXPECT().Show(tester.ctx, "u", "r", "u").Return(&types.Space{
		Name: "m",
	}, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Space{Name: "m"})

}

func TestSpaceHandler_Create(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Create
	})
	tester.RequireUser(t)

	req := &types.CreateSpaceReq{CreateRepoReq: types.CreateRepoReq{}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.ctx, req).Return(true, nil)
	reqn := *req
	reqn.Username = "u"
	tester.mocks.space.EXPECT().Create(tester.ctx, reqn).Return(&types.Space{Name: "m"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Space{Name: "m"})
}

func TestSpaceHandler_Update(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Update
	})
	tester.RequireUser(t)

	req := &types.UpdateSpaceReq{UpdateRepoReq: types.UpdateRepoReq{}}
	tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.ctx, req).Return(true, nil)
	tester.mocks.space.EXPECT().Update(tester.ctx, &types.UpdateSpaceReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Namespace: "u",
			Name:      "r",
			Username:  "u",
		},
	}).Return(&types.Space{Name: "m"}, nil)
	tester.WithBody(t, req).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.Space{Name: "m"})
}

func TestSpaceHandler_Delete(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Delete
	})
	tester.RequireUser(t)

	tester.mocks.space.EXPECT().Delete(tester.ctx, "u", "r", "u").Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSpaceHandler_Run(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Run
	})
	tester.RequireUser(t)

	tester.mocks.repo.EXPECT().AllowAdminAccess(tester.ctx, types.SpaceRepo, "u", "r", "u").Return(true, nil)
	tester.mocks.space.EXPECT().Deploy(tester.ctx, "u", "r", "u").Return(123, nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSpaceHandler_Wakeup(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Wakeup
	})

	tester.mocks.space.EXPECT().Wakeup(tester.ctx, "u", "r").Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSpaceHandler_Stop(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Stop
	})
	tester.RequireUser(t)

	tester.mocks.repo.EXPECT().AllowAdminAccess(tester.ctx, types.SpaceRepo, "u", "r", "u").Return(true, nil)
	tester.mocks.space.EXPECT().Stop(tester.ctx, "u", "r", false).Return(nil)
	tester.Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)
}

func TestSpaceHandler_Status(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Status
	})
	tester.handler.spaceStatusCheckInterval = 0

	tester.mocks.repo.EXPECT().AllowReadAccess(
		tester.ctx, types.SpaceRepo, "u", "r", "u",
	).Return(true, nil)
	cc, cancel := context.WithCancel(tester.ctx.Request.Context())
	tester.ctx.Request = tester.ctx.Request.WithContext(cc)
	tester.mocks.space.EXPECT().Status(
		mock.Anything, "u", "r",
	).Return("", "s1", nil).Once()
	tester.mocks.space.EXPECT().Status(
		mock.Anything, "u", "r",
	).RunAndReturn(func(ctx context.Context, s1, s2 string) (string, string, error) {
		cancel()
		return "", "s3", nil
	}).Once()

	tester.WithUser().Execute()
	require.Equal(t, 200, tester.response.Code)
	headers := tester.response.Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:status\ndata:s1\n\nevent:status\ndata:s3\n\n",
		tester.response.Body.String(),
	)

}

func TestSpaceHandler_Logs(t *testing.T) {
	tester := NewSpaceTester(t).WithHandleFunc(func(h *SpaceHandler) gin.HandlerFunc {
		return h.Logs
	})

	tester.mocks.repo.EXPECT().AllowReadAccess(
		tester.ctx, types.SpaceRepo, "u", "r", "u",
	).Return(true, nil)
	runlogChan := make(chan string)
	tester.mocks.space.EXPECT().Logs(
		mock.Anything, "u", "r",
	).Return(deploy.NewMultiLogReader(nil, runlogChan), nil)
	cc, cancel := context.WithCancel(tester.ctx.Request.Context())
	tester.ctx.Request = tester.ctx.Request.WithContext(cc)
	go func() {
		runlogChan <- "foo"
		runlogChan <- "bar"
		close(runlogChan)
		cancel()
	}()

	tester.WithUser().Execute()
	require.Equal(t, 200, tester.response.Code)
	headers := tester.response.Header()
	require.Equal(t, "text/event-stream", headers.Get("Content-Type"))
	require.Equal(t, "no-cache", headers.Get("Cache-Control"))
	require.Equal(t, "keep-alive", headers.Get("Connection"))
	require.Equal(t, "chunked", headers.Get("Transfer-Encoding"))
	require.Equal(
		t, "event:Container\ndata:foo\n\nevent:Container\ndata:bar\n\n",
		tester.response.Body.String(),
	)
}
