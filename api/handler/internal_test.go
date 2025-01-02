package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	temporal_mock "go.temporal.io/sdk/mocks"
	workflow_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type InternalTester struct {
	*GinTester
	handler *InternalHandler
	mocks   struct {
		internal *mockcomponent.MockInternalComponent
		workflow *workflow_mock.MockClient
	}
}

func NewInternalTester(t *testing.T) *InternalTester {
	tester := &InternalTester{GinTester: NewGinTester()}
	tester.mocks.internal = mockcomponent.NewMockInternalComponent(t)
	tester.mocks.workflow = workflow_mock.NewMockClient(t)

	tester.handler = &InternalHandler{
		internal:       tester.mocks.internal,
		temporalClient: tester.mocks.workflow,
		config:         &config.Config{},
	}
	tester.WithParam("internalId", "testInternalId")
	tester.WithParam("userId", "testUserId")
	return tester
}

func (t *InternalTester) WithHandleFunc(fn func(h *InternalHandler) gin.HandlerFunc) *InternalTester {
	t.ginHandler = fn(t.handler)
	return t
}

func TestInternalHandler_Allowed(t *testing.T) {
	tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
		return h.Allowed
	})

	tester.mocks.internal.EXPECT().Allowed(tester.ctx).Return(true, nil)
	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"status":  true,
		"message": "allowed",
	})
}

func TestInternalHandler_SSHAllowed(t *testing.T) {
	t.Run("https", func(t *testing.T) {
		tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
			return h.SSHAllowed
		})

		tester.WithBody(t, &types.GitalyAllowedReq{
			Protocol: "https",
		}).Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"status":  true,
			"message": "allowed",
		})
	})

	t.Run("ssh", func(t *testing.T) {
		tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
			return h.SSHAllowed
		})

		tester.mocks.internal.EXPECT().SSHAllowed(tester.ctx, types.SSHAllowedReq{
			RepoType:  types.ModelRepo,
			Namespace: "u",
			Name:      "r",
			Action:    "act",
			Changes:   "c",
			KeyID:     "k",
			Protocol:  "ssh",
			CheckIP:   "ci",
		}).Return(&types.SSHAllowedResp{Message: "msg"}, nil)
		tester.WithHeader("Content-Type", "application/json").WithBody(t, &types.GitalyAllowedReq{
			Protocol:     "ssh",
			GlRepository: "models/u/r",
			Action:       "act",
			KeyID:        "k",
			Changes:      "c",
			CheckIP:      "ci",
		}).Execute()

		tester.ResponseEqSimple(t, 200, &types.SSHAllowedResp{Message: "msg"})
	})

}

func TestInternalHandler_LfsAuthenticate(t *testing.T) {
	tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
		return h.LfsAuthenticate
	})

	tester.mocks.internal.EXPECT().LfsAuthenticate(tester.ctx, types.LfsAuthenticateReq{
		RepoType:  types.ModelRepo,
		Namespace: "u",
		Name:      "r",
		Repo:      "models/u/r",
	}).Return(&types.LfsAuthenticateResp{LfsToken: "t"}, nil)
	tester.WithHeader("Content-Type", "application/json").WithBody(t, &types.LfsAuthenticateReq{
		Repo: "models/u/r",
	}).Execute()

	tester.ResponseEqSimple(t, 200, &types.LfsAuthenticateResp{LfsToken: "t"})
}

func TestInternalHandler_PreReceive(t *testing.T) {
	tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
		return h.PreReceive
	})

	tester.Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"reference_counter_increased": true,
	})
}

func TestInternalHandler_PostReceive(t *testing.T) {
	tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
		return h.PostReceive
	})

	tester.mocks.internal.EXPECT().GetCommitDiff(tester.ctx, types.GetDiffBetweenTwoCommitsReq{
		LeftCommitId:  "foo",
		RightCommitId: "bar",
		Namespace:     "u",
		Name:          "r",
		Ref:           "main",
		RepoType:      types.ModelRepo,
	}).Return(&types.GiteaCallbackPushReq{Ref: "foo"}, nil)

	runMock := &temporal_mock.WorkflowRun{}
	runMock.On("GetID").Return("id")
	tester.mocks.workflow.EXPECT().ExecuteWorkflow(
		tester.ctx, client.StartWorkflowOptions{
			TaskQueue: workflow.HandlePushQueueName,
		}, mock.Anything,
		&types.GiteaCallbackPushReq{Ref: "ref/heads/main"}, &config.Config{},
	).Return(
		runMock, nil,
	)
	tester.WithHeader("Content-Type", "application/json").WithBody(t, &types.PostReceiveReq{
		Changes:      "foo bar ref/heads/main\n",
		GlRepository: "models/u/r",
	}).Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"reference_counter_decreased": true,
		"messages": []Messages{
			{
				Message: "Welcome to OpenCSG!",
				Type:    "alert",
			},
		},
	})
}

func TestInternalHandler_GetAuthorizedKeys(t *testing.T) {
	tester := NewInternalTester(t).WithHandleFunc(func(h *InternalHandler) gin.HandlerFunc {
		return h.GetAuthorizedKeys
	})

	tester.mocks.internal.EXPECT().GetAuthorizedKeys(tester.ctx, "k").Return(&database.SSHKey{
		ID:      1,
		Content: "kk",
	}, nil)
	tester.WithQuery("key", "k").Execute()

	tester.ResponseEqSimple(t, 200, gin.H{
		"id":  int64(1),
		"key": "kk",
	})
}
