package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/moderation/component"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/component"
)

type RepoTester struct {
	*testutil.GinTester
	handler *RepoHandler
	mocks   struct {
		rc *mockcomp.MockRepoComponent
	}
}

func newRepoTester(t *testing.T) *RepoTester {
	tester := &RepoTester{GinTester: testutil.NewGinTester()}
	tester.mocks.rc = mockcomp.NewMockRepoComponent(t)
	tester.handler = &RepoHandler{
		rc: tester.mocks.rc,
	}
	tester.WithParam("id", "1")
	return tester
}

func (t *RepoTester) WithHandleFunc(fn func(h *RepoHandler) gin.HandlerFunc) *RepoTester {
	t.Handler(fn(t.handler))
	return t
}

func TestRepoHandler_FullCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("request with namespace in whitelist", func(t *testing.T) {
		tester := newRepoTester(t).WithHandleFunc(func(h *RepoHandler) gin.HandlerFunc {
			return h.FullCheck
		})
		req := request{
			Namespace: "admin",
			Name:      "test_repo",
			RepoType:  types.ModelRepo,
		}

		tester.mocks.rc.EXPECT().RepoFullCheck(mock.Anything, component.RepoFullCheckRequest{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}).Return(&types.RepoFullCheckResult{Skipped: true}, nil).Once()
		tester.WithBody(t, req)
		tester.Execute()

		tester.ResponseEq(t, 200, "OK", &types.RepoFullCheckResult{Skipped: true})
	})

	t.Run("request with namespace not in whitelist, trigger workflow", func(t *testing.T) {
		tester := newRepoTester(t).WithHandleFunc(func(h *RepoHandler) gin.HandlerFunc {
			return h.FullCheck
		})
		req := request{
			Namespace: "user1",
			Name:      "test_repo",
			RepoType:  types.ModelRepo,
		}

		tester.mocks.rc.EXPECT().RepoFullCheck(mock.Anything, component.RepoFullCheckRequest{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}).Return(&types.RepoFullCheckResult{
			Skipped:    false,
			WorkflowID: "1",
		}, nil).Once()

		tester.WithBody(t, req)
		tester.Execute()

		tester.ResponseEq(t, 200, "OK", &types.RepoFullCheckResult{Skipped: false, WorkflowID: "1"})
	})

	t.Run("bad request", func(t *testing.T) {
		tester := newRepoTester(t).WithHandleFunc(func(h *RepoHandler) gin.HandlerFunc {
			return h.FullCheck
		})

		tester.WithBody(t, `{"bad": "json"}`)
		tester.Execute()

		tester.ResponseEq(t, 400, "json: cannot unmarshal string into Go value of type handler.request", nil)
	})
}

type request struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
}
