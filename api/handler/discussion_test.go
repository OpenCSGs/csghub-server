package handler_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/apitest"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/builder/testutil"
	"opencsg.com/csghub-server/common/types"
)

type DiscussionTester struct {
	*testutil.GinTester
	handler *handler.DiscussionHandler
	mocks   struct {
		discussion *mockcomponent.MockDiscussionComponent
		sensitive  *mockcomponent.MockSensitiveComponent
	}
}

func NewDiscussionTester(t *testing.T) *DiscussionTester {
	tester := &DiscussionTester{GinTester: testutil.NewGinTester()}
	tester.mocks.discussion = mockcomponent.NewMockDiscussionComponent(t)
	tester.mocks.sensitive = mockcomponent.NewMockSensitiveComponent(t)

	tester.handler = handler.NewDiscussionHandlerDI(
		tester.mocks.discussion,
		tester.mocks.sensitive,
	)
	return tester
}

func TestDiscussionHandler_CreateRepoDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewPostRequest(
		"/api/v1/models/u/r/discussions", &types.CreateRepoDiscussionRequest{
			Title: "foo",
		},
	)
	require.NoError(t, err)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		mock.Anything, &types.CreateRepoDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CreateRepoDiscussion(
		mock.Anything, types.CreateRepoDiscussionRequest{
			CurrentUser: "u",
			Namespace:   "u",
			Name:        "r",
			RepoType:    types.ModelRepo,
			Title:       "foo",
		},
	).Return(&types.CreateDiscussionResponse{ID: 123}, nil)

	require.Nil(t, err)
	resp := server.AuthAndSend(t, req)
	resp.ResponseEq(t, 200, tester.OKText, &types.CreateDiscussionResponse{ID: 123})
}

func TestDiscussionHandler_UpdateDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewPutRequest(
		"/api/v1/discussions/1", types.UpdateDiscussionRequest{
			CurrentUser: "u",
			ID:          1,
			Title:       "foo",
		},
	)
	require.NoError(t, err)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		mock.Anything, &types.UpdateDiscussionRequest{Title: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().UpdateDiscussion(
		mock.Anything, types.UpdateDiscussionRequest{
			CurrentUser: "u",
			ID:          1,
			Title:       "foo",
		},
	).Return(nil)
	resp := server.AuthAndSend(t, req)
	resp.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_DeleteDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewDeleteRequest(
		"/api/v1/discussions/1",
	)
	require.NoError(t, err)

	tester.mocks.discussion.EXPECT().DeleteDiscussion(
		mock.Anything, "u", int64(1),
	).Return(nil)
	resp := server.AuthAndSend(t, req)

	resp.ResponseEq(t, 200, tester.OKText, nil)
}

func TestDiscussionHandler_ShowDiscussion(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewGetRequest(
		"/api/v1/discussions/1",
	)
	require.NoError(t, err)

	tester.mocks.discussion.EXPECT().GetDiscussion(
		mock.Anything, int64(1),
	).Return(&types.ShowDiscussionResponse{Title: "foo"}, nil)
	resp := server.Send(req)

	resp.ResponseEq(t, 200, tester.OKText, &types.ShowDiscussionResponse{Title: "foo"})
}

func TestDiscussionHandler_ListRepoDiscussions(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewGetRequest(
		"/api/v1/models/u/r/discussions",
	)
	require.NoError(t, err)

	tester.mocks.discussion.EXPECT().ListRepoDiscussions(
		mock.Anything, types.ListRepoDiscussionRequest{
			RepoType:  types.ModelRepo,
			Namespace: "u",
			Name:      "r",
		},
	).Return(&types.ListRepoDiscussionResponse{Discussions: []*types.CreateDiscussionResponse{
		{ID: 1},
	}}, nil)
	resp := server.Send(req)
	resp.ResponseEq(t, 200, tester.OKText, &types.ListRepoDiscussionResponse{
		Discussions: []*types.CreateDiscussionResponse{{ID: 1}},
	})
}

func TestDiscussionHandler_CreateDiscussionComment(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewPostRequest(
		"/api/v1/discussions/1/comments",
		&types.CreateCommentRequest{Content: "foo"},
	)
	require.NoError(t, err)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		mock.Anything, &types.CreateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().CreateDiscussionComment(
		mock.Anything, types.CreateCommentRequest{
			CurrentUser:   "u",
			Content:       "foo",
			CommentableID: 1,
		},
	).Return(&types.CreateCommentResponse{ID: 1}, nil)

	resp := server.AuthAndSend(t, req)
	resp.ResponseEq(t, 200, tester.OKText, &types.CreateCommentResponse{ID: 1})
}

func TestDiscussionHandler_UpdateComment(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewPutRequest(
		"/api/v1/discussions/1/comments/2",
		&types.UpdateCommentRequest{Content: "foo"},
	)
	require.NoError(t, err)

	tester.mocks.sensitive.EXPECT().CheckRequestV2(
		mock.Anything, &types.UpdateCommentRequest{Content: "foo"},
	).Return(true, nil)
	tester.mocks.discussion.EXPECT().UpdateComment(
		mock.Anything, "u", int64(1), "foo",
	).Return(nil)

	resp := server.AuthAndSend(t, req)
	resp.ResponseEq(t, 200, tester.OKText, nil)
}

func TestDiscussionHandler_DeleteComment(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewDeleteRequest(
		"/api/v1/discussions/1/comments/2",
	)
	require.NoError(t, err)

	tester.mocks.discussion.EXPECT().DeleteComment(
		mock.Anything, "u", int64(1),
	).Return(nil)

	resp := server.AuthAndSend(t, req)
	resp.ResponseEq(t, 200, tester.OKText, nil)

}

func TestDiscussionHandler_ListDiscussionComments(t *testing.T) {
	tester := NewDiscussionTester(t)
	server := apitest.NewTestServer(t, func(s *router.ServerImpl) {
		s.DiscussionHandler = tester.handler
	})
	req, err := server.NewGetRequest(
		"/api/v1/discussions/1/comments",
	)
	require.NoError(t, err)

	tester.mocks.discussion.EXPECT().ListDiscussionComments(
		mock.Anything, int64(1),
	).Return([]*types.DiscussionResponse_Comment{{Content: "foo"}}, nil)

	resp := server.Send(req)
	resp.ResponseEq(t, 200, tester.OKText, []*types.DiscussionResponse_Comment{{Content: "foo"}})

}
