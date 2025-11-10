package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	mocktemporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/moderation/component"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/common"
)

type RepoTester struct {
	*testutil.GinTester
	handler *RepoHandler
	mocks   struct {
		rc  *mockcomp.MockRepoComponent
		cfg *config.Config
	}
}

func newRepoTester(t *testing.T) *RepoTester {
	tester := &RepoTester{GinTester: testutil.NewGinTester()}
	tester.mocks.rc = mockcomp.NewMockRepoComponent(t)
	tester.mocks.cfg = &config.Config{}
	tester.handler = &RepoHandler{
		rc:     tester.mocks.rc,
		config: &config.Config{},
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

		tester.mocks.rc.EXPECT().GetNamespaceWhiteList(mock.Anything).Return([]string{"admin", "test"}, nil).Once()
		tester.WithBody(t, req)
		tester.Execute()

		tester.ResponseEq(t, 200, "OK", nil)
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

		tester.mocks.rc.EXPECT().GetNamespaceWhiteList(mock.Anything).Return([]string{"admin", "test"}, nil).Once()

		mockWorkflowClient := mocktemporal.NewMockClient(t)
		temporal.Assign(mockWorkflowClient)
		workflowOptions := client.StartWorkflowOptions{
			TaskQueue: "moderation_repo_full_check_queue",
		}
		repo := common.Repo{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}
		we := mocks.NewWorkflowRun(t)
		we.On("GetID").Return("1").Once()
		mockWorkflowClient.EXPECT().ExecuteWorkflow(mock.Anything, workflowOptions, mock.Anything, repo, tester.mocks.cfg).Return(we, nil).Once()

		tester.WithBody(t, req)
		tester.Execute()

		tester.ResponseEq(t, 200, "OK", nil)
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
