package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

type DiscussionTester struct {
	*GinTester
	handler *DiscussionHandler
	mocks   struct {
		discussion *mockcomponent.MockDiscussionComponent
		sensitive  *mockcomponent.MockSensitiveComponent
	}
}

func NewDiscussionTester(t *testing.T) *DiscussionTester {
	tester := &DiscussionTester{GinTester: NewGinTester()}
	tester.mocks.discussion = mockcomponent.NewMockDiscussionComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)

	tester.handler = &DiscussionHandler{
		discussion: tester.mocks.discussion,
		sensitive:  tester.mocks.sensitive,
	}
	tester.WithParam("namespace", "u")
	tester.WithParam("name", "r")
	return tester
}

func (t *DiscussionTester) WithHandleFunc(fn func(h *DiscussionHandler) gin.HandlerFunc) *DiscussionTester {
	t.ginHandler = fn(t.handler)
	return t
}

func TestDiscussionHandler_CreateRepoDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateRepoDiscussion
	})
	tester.RequireUser(t)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.ctx, &types.CreateRepoDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CreateRepoDiscussion(
		tester.ctx, types.CreateRepoDiscussionRequest{
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
	tester.RequireUser(t)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.ctx, &types.UpdateDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().UpdateDiscussion(
		tester.ctx, types.UpdateDiscussionRequest{
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
	tester.RequireUser(t)

	tester.mocks.discussion.EXPECT().DeleteDiscussion(
		tester.ctx, "u", int64(1),
	).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_ShowDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.ShowDiscussion
	})

	tester.mocks.discussion.EXPECT().GetDiscussion(
		tester.ctx, int64(1),
	).Return(&types.ShowDiscussionResponse{Title: "foo"}, nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ShowDiscussionResponse{Title: "foo"})

}

func TestDiscussionHandler_ListRepoDiscussions(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.ListRepoDiscussions
	})

	tester.mocks.discussion.EXPECT().ListRepoDiscussions(
		tester.ctx, types.ListRepoDiscussionRequest{
			CurrentUser: "u",
			RepoType:    types.ModelRepo,
			Namespace:   "u",
			Name:        "r",
		},
	).Return(&types.ListRepoDiscussionResponse{Discussions: []*types.CreateDiscussionResponse{
		{ID: 1},
	}}, nil)
	tester.WithUser().WithParam("repo_type", "models").Execute()

	tester.ResponseEq(t, 200, tester.OKText, &types.ListRepoDiscussionResponse{
		Discussions: []*types.CreateDiscussionResponse{{ID: 1}},
	})

}

func TestDiscussionHandler_CreateDiscussionComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.CreateDiscussionComment
	})
	tester.RequireUser(t)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.ctx, &types.CreateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CreateDiscussionComment(
		tester.ctx, types.CreateCommentRequest{
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

func TestDiscussionHandler_UpdateComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.UpdateComment
	})
	tester.RequireUser(t)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		tester.ctx, &types.UpdateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().UpdateComment(
		tester.ctx, "u", int64(1), "foo",
	).Return(nil)
	tester.WithParam("id", "1").WithBody(
		t, &types.UpdateCommentRequest{Content: "foo"},
	).Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_DeleteComment(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.DeleteComment
	})
	tester.RequireUser(t)

	tester.mocks.discussion.EXPECT().DeleteComment(
		tester.ctx, "u", int64(1),
	).Return(nil)
	tester.WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_ListDiscussionComments(t *testing.T) {
	tester := NewDiscussionTester(t).WithHandleFunc(func(h *DiscussionHandler) gin.HandlerFunc {
		return h.ListDiscussionComments
	})

	tester.mocks.discussion.EXPECT().ListDiscussionComments(
		tester.ctx, int64(1),
	).Return([]*types.DiscussionResponse_Comment{{Content: "foo"}}, nil)
	tester.WithUser().WithParam("id", "1").Execute()

	tester.ResponseEq(t, 200, tester.OKText, []*types.DiscussionResponse_Comment{{Content: "foo"}})

}
