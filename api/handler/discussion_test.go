package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalclient "go.temporal.io/sdk/client"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	csghubtemporal "opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
	workflowcommon "opencsg.com/csghub-server/moderation/workflow/common"
)

type discussionTemporalStub struct {
	csghubtemporal.Client
	executeWorkflow func(context.Context, temporalclient.StartWorkflowOptions, interface{}, ...interface{}) (temporalclient.WorkflowRun, error)
}

func (s *discussionTemporalStub) ExecuteWorkflow(
	ctx context.Context,
	options temporalclient.StartWorkflowOptions,
	workflow interface{},
	args ...interface{},
) (temporalclient.WorkflowRun, error) {
	return s.executeWorkflow(ctx, options, workflow, args...)
}

type DiscussionTester struct {
	*testutil.GinTester
	handler *DiscussionHandler
	mocks   struct {
		discussion  *mockcomponent.MockDiscussionComponent
		sensitive   *mockcomponent.MockSensitiveComponent
		mediaPolicy *mockcomponent.MockCommentMediaPolicy
	}
}

func NewDiscussionTester(t *testing.T) *DiscussionTester {
	tester := &DiscussionTester{GinTester: testutil.NewGinTester()}
	tester.mocks.discussion = mockcomponent.NewMockDiscussionComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)
	tester.mocks.mediaPolicy = mockcomponent.NewMockCommentMediaPolicy(t)

	tester.handler = &DiscussionHandler{
		discussion:         tester.mocks.discussion,
		sensitive:          tester.mocks.sensitive,
		commentMediaPolicy: tester.mocks.mediaPolicy,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *DiscussionTester) WithHandleFunc(fn func(h *DiscussionHandler) gin.HandlerFunc) *DiscussionTester {
	t.Handler(fn(t.handler))
	return t
}

func TestDiscussionHandler_CreateRepoDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateRepoDiscussion
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.CreateRepoDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CreateRepoDiscussion(
		tester.Ctx(), types.CreateRepoDiscussionRequest{
			CurrentUser: "u",
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			Title:       "foo",
		},
	).Return(&types.CreateDiscussionResponse{ID: 123}, nil)
	tester.WithParam("repo_type", "models").WithBody(t, &types.CreateRepoDiscussionRequest{
		Title: "foo",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.CreateDiscussionResponse{ID: 123})

}

func TestDiscussionHandler_UpdateDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateDiscussion
	})
	tester.WithUser()

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.UpdateDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().UpdateDiscussion(
		tester.Ctx(), types.UpdateDiscussionRequest{
			CurrentUser: "u",
			ID:          1,
			Title:       "foo",
		},
	).Return(nil)
	tester.WithParam("id", "1").WithBody(t, &types.UpdateDiscussionRequest{
		Title: "foo",
	}).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_DeleteDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.DeleteDiscussion
	})
	tester.WithUser()

	tester.mocks.discussion.EXPECT().DeleteDiscussion(
		tester.Ctx(), "u", int64(1),
	).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_ShowDiscussion(t *testing.T) {
	t.Run("ok case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ShowDiscussion
		})

		tester.mocks.discussion.EXPECT().GetDiscussion(
			tester.Ctx(), "u", int64(1),
			10, 1,
		).Return(&types.ShowDiscussionResponse{Title: "foo"}, nil)
		tester.WithUser().WithParam("id", "1").WithQuery("per", "10").WithQuery("page", "1").Execute()

		tester.ResponseEq(t, 200, tester.OKText, &types.ShowDiscussionResponse{Title: "foo"})
	})

	t.Run("404 case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ShowDiscussion
		})
		tester.mocks.discussion.EXPECT().GetDiscussion(
			tester.Ctx(), "u", int64(2),
			10, 1,
		).Once().Return(nil, errorx.ErrDatabaseNoRows)
		tester.WithUser().WithParam("id", "2").WithQuery("per", "10").WithQuery("page", "1").Execute()
		tester.ResponseEqSimple(t, 404, httpbase.R{
			Code: errorx.ErrDatabaseNoRows.Code(),
			Msg:  errorx.ErrDatabaseNoRows.Error(),
		})
	})
}

func TestDiscussionHandler_ListRepoDiscussions(t *testing.T) {
	t.Run("ok case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ListRepoDiscussions
		})

		tester.mocks.discussion.EXPECT().ListRepoDiscussions(
			tester.Ctx(), types.ListRepoDiscussionRequest{
				CurrentUser: "u",
				RepoType:    types.ModelRepo,
				Namespace:   "u",
				Name:        "r",
			},
			10, 1,
		).Return(&types.ListRepoDiscussionResponse{Discussions: []*types.CreateDiscussionResponse{
			{ID: 1},
		}}, 1, nil)
		tester.WithUser().WithParam("repo_type", "models").WithQuery("per", "10").WithQuery("page", "1").Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"data": &types.ListRepoDiscussionResponse{Discussions: []*types.CreateDiscussionResponse{
				{ID: 1},
			}},
			"total": 1,
			"msg":   "OK",
		})
	})

	t.Run("404 case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ListRepoDiscussions
		})
		tester.mocks.discussion.EXPECT().ListRepoDiscussions(
			tester.Ctx(), types.ListRepoDiscussionRequest{
				CurrentUser: "u",
				RepoType:    types.ModelRepo,
				Namespace:   "u",
				Name:        "r",
			},
			10, 1,
		).Once().Return(nil, 0, errorx.ErrDatabaseNoRows)
		tester.WithUser().WithParam("repo_type", "models").WithQuery("per", "10").WithQuery("page", "1").Execute()
		tester.ResponseEqSimple(t, 404, httpbase.R{
			Code: errorx.ErrDatabaseNoRows.Code(),
			Msg:  errorx.ErrDatabaseNoRows.Error(),
		})
	})
}

func TestDiscussionHandler_CreateDiscussionComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateDiscussionComment
	})
	tester.WithUser()

	tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.CreateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "foo").
		Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil)
	tester.mocks.discussion.EXPECT().CreateDiscussionComment(
		tester.Ctx(), types.CreateCommentRequest{
			CurrentUser:   "u",
			Content:       "foo",
			CommentableID: 1,
		},
	).Return(&types.CreateCommentResponse{ID: 1}, nil)
	tester.WithParam("id", "1").WithParam("repo_type", "models").WithBody(
		t, &types.CreateCommentRequest{Content: "foo"},
	).Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.CreateCommentResponse{ID: 1})

}

func TestDiscussionHandler_CreateCommentChecksAccessBeforeModeration(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateDiscussionComment
	})
	tester.WithUser()
	tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).
		Return(errorx.ErrForbiddenMsg("no repo access"))

	tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()

	tester.ResponseEqCode(t, 403)
	tester.mocks.sensitive.AssertNotCalled(t, "CheckRequestV2")
	tester.mocks.mediaPolicy.AssertNotCalled(t, "Check")
	tester.mocks.discussion.AssertNotCalled(t, "CreateDiscussionComment")
}

func TestDiscussionHandler_UpdateComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateComment
	})
	tester.WithUser()

	tester.mocks.discussion.EXPECT().CheckCommentOwnership(tester.Ctx(), "u", int64(1)).Return(nil)
	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.UpdateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CommentUnderModeration(tester.Ctx(), int64(1)).Return(false, nil)
	tester.mocks.mediaPolicy.EXPECT().CheckUpdate(tester.Ctx(), "foo").
		Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil)
	tester.mocks.discussion.EXPECT().UpdateComment(
		tester.Ctx(), "u", int64(1), "foo",
	).Return(nil)
	tester.WithParam("comment_id", "1").WithBody(
		t, &types.UpdateCommentRequest{Content: "foo"},
	).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_UpdateCommentChecksOwnershipBeforeModeration(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateComment
	})
	tester.WithUser()
	tester.mocks.discussion.EXPECT().CheckCommentOwnership(tester.Ctx(), "u", int64(1)).
		Return(errorx.ErrForbiddenMsg("not owner"))

	tester.WithParam("comment_id", "1").WithBody(t, &types.UpdateCommentRequest{Content: "media"}).Execute()

	tester.ResponseEqCode(t, 403)
	tester.mocks.sensitive.AssertNotCalled(t, "CheckRequestV2")
	tester.mocks.mediaPolicy.AssertNotCalled(t, "CheckUpdate")
	tester.mocks.discussion.AssertNotCalled(t, "CommentUnderModeration")
}

func TestDiscussionHandler_DeleteComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.DeleteComment
	})
	tester.WithUser()

	tester.mocks.discussion.EXPECT().DeleteComment(
		tester.Ctx(), "u", int64(1),
	).Return(nil)
	tester.WithParam("comment_id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_CreateDiscussionCommentMediaGate(t *testing.T) {
	t.Run("pending forwards deferred notification to workflow", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.CreateDiscussionComment
		})
		tester.WithUser()
		tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
		items := []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}}
		notification := &types.CommentNotification{CommentID: 7, MsgUUID: "msg-1"}
		tester.handler.cfg = &config.Config{APIToken: "must-not-enter-temporal-history"}
		tester.handler.cfg.SensitiveCheck.MediaModerationPollInterval = 5 * time.Second
		tester.handler.cfg.SensitiveCheck.MediaModerationWorkflowTimeout = 10 * time.Minute
		tester.handler.temporal = &discussionTemporalStub{executeWorkflow: func(
			_ context.Context, _ temporalclient.StartWorkflowOptions, _ interface{}, args ...interface{},
		) (temporalclient.WorkflowRun, error) {
			require.Len(t, args, 2)
			req, ok := args[0].(workflowcommon.PollCommentMediaReq)
			require.True(t, ok)
			require.Equal(t, notification, req.Notification)
			opts, ok := args[1].(workflowcommon.PollCommentMediaOptions)
			require.True(t, ok, "workflow payload must use the non-sensitive options DTO")
			require.Equal(t, 5*time.Second, opts.PollInterval)
			require.Equal(t, 10*time.Minute, opts.WorkflowTimeout)
			for _, arg := range args {
				_, containsFullConfig := arg.(*config.Config)
				require.False(t, containsFullConfig, "full runtime config must not enter Temporal history")
			}
			return nil, nil
		}}

		tester.mocks.sensitive.EXPECT().CheckRequestV2(tester.Ctx(), &types.CreateCommentRequest{Content: "media"}).Return(true, nil)
		tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "media").Return(types.CommentMediaDecision{
			Decision: types.MediaModerationDecisionPending, Items: items,
		}, nil)
		tester.mocks.discussion.EXPECT().CreateDiscussionComment(tester.Ctx(), types.CreateCommentRequest{
			CurrentUser: "u", Content: "media", CommentableID: 1, MediaItems: items,
		}).Return(&types.CreateCommentResponse{ID: 7, Notification: notification}, nil)

		tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()
		tester.ResponseEqCode(t, 200)
	})

	t.Run("pending creates comment and links media", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.CreateDiscussionComment
		})
		tester.WithUser()
		tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
		items := []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}}

		tester.mocks.sensitive.EXPECT().CheckRequestV2(
			tester.Ctx(), &types.CreateCommentRequest{Content: "media"},
		).Return(true, nil)
		tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "media").
			Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPending, Items: items}, nil)
		tester.mocks.discussion.EXPECT().CreateDiscussionComment(
			tester.Ctx(), types.CreateCommentRequest{
				CurrentUser: "u", Content: "media", CommentableID: 1, MediaItems: items,
			},
		).Return(&types.CreateCommentResponse{ID: 7}, nil)
		// Comment creation and media linking are atomic in the component. The
		// temporal client is nil in the tester, so workflow start fails and the
		// committed pending comment must be removed with a detached context.
		tester.mocks.discussion.EXPECT().DeleteComment(mock.MatchedBy(func(ctx context.Context) bool {
			return ctx.Err() == nil
		}), "u", int64(7)).Return(nil).Once()
		tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()
		tester.ResponseEqCode(t, 503)
	})

	t.Run("workflow failure cleanup survives request cancellation", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.CreateDiscussionComment
		})
		tester.WithUser()
		items := []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}}
		requestCtx, cancelRequest := context.WithCancel(context.Background())
		cancelRequest()
		tester.Gctx().Request = tester.Gctx().Request.WithContext(requestCtx)
		tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(requestCtx, "u", int64(1)).Return(nil)

		tester.mocks.sensitive.EXPECT().CheckRequestV2(
			requestCtx, &types.CreateCommentRequest{Content: "media"},
		).Return(true, nil)
		tester.mocks.mediaPolicy.EXPECT().Check(requestCtx, "media").
			Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPending, Items: items}, nil)
		tester.mocks.discussion.EXPECT().CreateDiscussionComment(
			requestCtx, types.CreateCommentRequest{
				CurrentUser: "u", Content: "media", CommentableID: 1, MediaItems: items,
			},
		).Return(&types.CreateCommentResponse{ID: 8}, nil)
		tester.mocks.discussion.EXPECT().DeleteComment(mock.MatchedBy(func(ctx context.Context) bool {
			return ctx.Err() == nil
		}), "u", int64(8)).Return(nil).Once()

		tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()
		tester.ResponseEqCode(t, 503)
	})

	t.Run("rejected blocks write", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.CreateDiscussionComment
		})
		tester.WithUser()
		tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
		tester.mocks.sensitive.EXPECT().CheckRequestV2(
			tester.Ctx(), &types.CreateCommentRequest{Content: "media"},
		).Return(true, nil)
		tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "media").
			Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil)
		tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()
		tester.ResponseEqCode(t, 400)
		tester.mocks.discussion.AssertNotCalled(t, "CreateDiscussionComment")
	})

	t.Run("provider unavailable blocks write", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.CreateDiscussionComment
		})
		tester.WithUser()
		tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
		tester.mocks.sensitive.EXPECT().CheckRequestV2(
			tester.Ctx(), &types.CreateCommentRequest{Content: "media"},
		).Return(true, nil)
		tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "media").
			Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("offline"))
		tester.WithParam("id", "1").WithBody(t, &types.CreateCommentRequest{Content: "media"}).Execute()
		tester.ResponseEqCode(t, 503)
		tester.mocks.discussion.AssertNotCalled(t, "CreateDiscussionComment")
	})
}

func TestDiscussionHandler_UpdateCommentUnderModeration(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateComment
	})
	tester.WithUser()
	tester.mocks.discussion.EXPECT().CheckCommentOwnership(tester.Ctx(), "u", int64(1)).Return(nil)
	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.UpdateCommentRequest{Content: "media"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CommentUnderModeration(tester.Ctx(), int64(1)).Return(true, nil)
	tester.WithParam("comment_id", "1").WithBody(t, &types.UpdateCommentRequest{Content: "media"}).Execute()
	tester.ResponseEqCode(t, 409)
	tester.mocks.discussion.AssertNotCalled(t, "UpdateComment")
}

func TestDiscussionHandler_UpdateCommentWithNewMediaDoesNotSubmit(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateComment
	})
	tester.WithUser()
	tester.mocks.discussion.EXPECT().CheckCommentOwnership(tester.Ctx(), "u", int64(1)).Return(nil)
	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.UpdateCommentRequest{Content: "media"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CommentUnderModeration(tester.Ctx(), int64(1)).Return(false, nil)
	tester.mocks.mediaPolicy.EXPECT().CheckUpdate(tester.Ctx(), "media").
		Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPending}, nil)

	tester.WithParam("comment_id", "1").WithBody(t, &types.UpdateCommentRequest{Content: "media"}).Execute()
	tester.ResponseEqCode(t, 409)
	tester.mocks.mediaPolicy.AssertNotCalled(t, "Check")
	tester.mocks.discussion.AssertNotCalled(t, "UpdateComment")
}

func TestDiscussionHandler_CreateCommentMediaLimitReturnsBadRequest(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateDiscussionComment
	})
	tester.WithUser()
	tester.mocks.discussion.EXPECT().CheckDiscussionCommentAccess(tester.Ctx(), "u", int64(1)).Return(nil)
	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.Ctx(), &types.CreateCommentRequest{Content: "too-many-media"},
	).Return(true, nil)
	tester.mocks.mediaPolicy.EXPECT().Check(tester.Ctx(), "too-many-media").
		Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, component.ErrInvalidCommentMedia)

	tester.WithParam("id", "1").WithParam("repo_type", "models").WithBody(
		t, &types.CreateCommentRequest{Content: "too-many-media"},
	).Execute()
	tester.ResponseEqCode(t, 400)
	tester.mocks.discussion.AssertNotCalled(t, "CreateDiscussionComment")
}

func TestDiscussionHandler_ListDiscussionComments(t *testing.T) {
	t.Run("ok case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ListDiscussionComments
		})

		tester.mocks.discussion.EXPECT().ListDiscussionComments(
			tester.Ctx(), "u", int64(1), 10, 1,
		).Return([]*types.DiscussionResponse_Comment{{Content: "foo"}}, 1, nil)
		tester.WithUser().WithParam("id", "1").WithQuery("per", "10").WithQuery("page", "1").Execute()

		tester.ResponseEqSimple(t, 200, gin.H{
			"data":  []*types.DiscussionResponse_Comment{{Content: "foo"}},
			"total": 1,
			"msg":   "OK",
		})
	})

	t.Run("404 case", func(t *testing.T) {
		tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
			return h.ListDiscussionComments
		})
		tester.mocks.discussion.EXPECT().ListDiscussionComments(
			tester.Ctx(), "u", int64(1), 10, 1,
		).Once().Return(nil, 0, errorx.ErrDatabaseNoRows)
		tester.WithUser().WithParam("id", "1").WithQuery("per", "10").WithQuery("page", "1").Execute()
		tester.ResponseEqSimple(t, 404, httpbase.R{
			Code: errorx.ErrDatabaseNoRows.Code(),
			Msg:  errorx.ErrDatabaseNoRows.Error(),
		})
	})
}
