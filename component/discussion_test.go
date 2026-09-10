package component

import (
	"context"
	"sync"
	"testing"
	"time"

	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestDiscussionComponent_CreateDisucssion(t *testing.T) {
	repo := &database.Repository{
		ID: 1,
	}

	user := &database.User{
		ID: 1, Username: "user",
		Avatar: "avatar",
	}

	req := types.CreateRepoDiscussionRequest{
		Title:       "test discussion",
		RepoType:    "model",
		Namespace:   "namespace",
		Name:        "name",
		CurrentUser: "user",
	}

	t.Run("success", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockUserStore := mockdb.NewMockUserStore(t)
		mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
		mockRepoComponent := mockcomp.NewMockRepoComponent(t)
		// new discussionComponentImpl from mock db store
		comp := &discussionComponentImpl{
			repoStore:       mockRepoStore,
			userStore:       mockUserStore,
			discussionStore: mockDiscussionStore,
			repoCompo:       mockRepoComponent,
		}
		mockRepoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "namespace", "name").Return(repo, nil).Once()
		mockRepoStore.EXPECT().FindById(mock.Anything, repo.ID).Return(repo, nil).Once()
		mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, repo, "user").Return(true, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		disc := database.Discussion{
			Title:              "test discussion",
			DiscussionableID:   repo.ID,
			DiscussionableType: database.DiscussionableTypeRepo,
			UserID:             user.ID,
		}
		dbdisc := disc
		dbdisc.ID = 1
		dbdisc.CreatedAt = time.Now()
		mockDiscussionStore.EXPECT().Create(mock.Anything, disc).Return(&dbdisc, nil).Once()

		actualDisc, err := comp.CreateRepoDiscussion(context.TODO(), req)
		require.Nil(t, err)

		expectedDisc := &types.CreateDiscussionResponse{
			ID:           1,
			Title:        "test discussion",
			CommentCount: 0,
			CreatedAt:    dbdisc.CreatedAt,
			User: &types.DiscussionResponse_User{
				ID:       1,
				Username: "user",
				Avatar:   "avatar",
			},
		}
		require.Equal(t, expectedDisc, actualDisc)
	})

	t.Run("forbidden", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockUserStore := mockdb.NewMockUserStore(t)
		mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
		mockRepoComponent := mockcomp.NewMockRepoComponent(t)
		// new discussionComponentImpl from mock db store
		comp := &discussionComponentImpl{
			repoStore:       mockRepoStore,
			userStore:       mockUserStore,
			discussionStore: mockDiscussionStore,
			repoCompo:       mockRepoComponent,
		}
		mockRepoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "namespace", "name").Return(repo, nil).Once()
		mockRepoStore.EXPECT().FindById(mock.Anything, repo.ID).Return(repo, nil).Once()
		mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, repo, "user").Return(false, errorx.ErrForbidden).Once()

		actualDisc, err := comp.CreateRepoDiscussion(context.TODO(), req)
		require.ErrorIs(t, err, errorx.ErrForbidden)
		require.Nil(t, actualDisc)
	})
}

func TestDiscussionComponent_GetDiscussion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	disc := database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
		CommentCount: 1,
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, int64(1)).Return(&disc, nil).Once()
	mockRepoStore.EXPECT().FindById(mock.Anything, int64(1)).Return(&database.Repository{ID: 1}, nil).Once()
	mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, &database.Repository{ID: 1}, "user").Return(true, nil).Once()
	comments := []database.Comment{
		{
			ID:      1,
			Content: "test comment",
			User: &database.User{
				ID:       1,
				Username: "user",
				Avatar:   "avatar",
			},
		},
	}
	mockDiscussionStore.EXPECT().FindVisibleDiscussionComments(mock.Anything, database.FindVisibleCommentsReq{
		DiscussionID: 1, CurrentUser: "user", Per: 10, Page: 1,
	}).Return(comments, 1, nil).Once()

	// Updated to include pagination parameters
	resp, err := comp.GetDiscussion(context.TODO(), "user", int64(1), 10, 1)
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(1), resp.ID)
	require.Equal(t, "test discussion", resp.Title)
	require.NotNil(t, resp.Comments)
	require.Equal(t, 1, resp.Comments.Total)
	require.Equal(t, 10, resp.Comments.Per)
	require.Equal(t, 1, resp.Comments.Page)
	require.Len(t, resp.Comments.Data, 1)
	require.Equal(t, "test comment", resp.Comments.Data[0].Content)
	require.Equal(t, "user", resp.Comments.Data[0].User.Username)
	require.Equal(t, "avatar", resp.Comments.Data[0].User.Avatar)
}

func TestDiscussionComponent_UpdateDisussion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	req := types.UpdateDiscussionRequest{
		ID:          1,
		Title:       "test discussion",
		CurrentUser: "user",
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.CurrentUser).Return(database.User{ID: 1}, nil).Once()

	disc := database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, req.ID).Return(&disc, nil).Once()
	mockDiscussionStore.EXPECT().UpdateByID(mock.Anything, req.ID, req.Title).Return(nil).Once()

	err := comp.UpdateDiscussion(context.TODO(), req)
	require.Nil(t, err)
}

func TestDiscussionComponent_DeleteDisussion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	currentUser := "user"
	id := int64(1)

	disc := database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, id).Return(&disc, nil).Once()
	mockDiscussionStore.EXPECT().DeleteByID(mock.Anything, id).Return(nil).Once()

	err := comp.DeleteDiscussion(context.TODO(), currentUser, id)
	require.Nil(t, err)
}

func TestDiscussionComponent_ListRepoDiscussions(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	repo := &database.Repository{
		ID: 1,
	}
	mockRepoStore.EXPECT().FindById(mock.Anything, repo.ID).Return(repo, nil).Once()
	mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, repo, "user").Return(true, nil).Once()
	mockRepoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "namespace", "name").Return(repo, nil).Once()

	var discussions []database.Discussion
	discussions = append(discussions, database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
	})
	discussions = append(discussions, database.Discussion{
		ID:                 2,
		Title:              "test discussion 2",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             2,
		User:               nil,
	})
	mockDiscussionStore.EXPECT().FindByDiscussionableID(mock.Anything, database.DiscussionableTypeRepo, repo.ID, 10, 1).Return(discussions, 1, nil).Once()

	resp, _, err := comp.ListRepoDiscussions(context.TODO(), types.ListRepoDiscussionRequest{
		RepoType:    types.ModelRepo,
		Namespace:   "namespace",
		Name:        "name",
		CurrentUser: "user",
	}, 10, 1)
	require.Nil(t, err)
	require.Len(t, resp.Discussions, 1)
	require.Equal(t, "test discussion", resp.Discussions[0].Title)

}

func TestDiscussionComponent_CreateDisussionComment(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
	config := &config.Config{}
	config.Notification.NotificationRetryCount = 1
	config.APIToken = "test-api-token"
	config.Notification.Host = "localhost"
	config.Notification.Port = 8095

	var wg sync.WaitGroup
	wg.Add(1)

	mockNotificationRpc.EXPECT().
		Send(mock.Anything, mock.MatchedBy(func(msg *types.MessageRequest) bool {
			defer wg.Done()
			return msg.Scenario == types.MessageScenarioDiscussion
		})).
		Return(nil).
		Once()

	comp := &discussionComponentImpl{
		repoStore:             mockRepoStore,
		userStore:             mockUserStore,
		discussionStore:       mockDiscussionStore,
		repoCompo:             mockRepoComponent,
		notificationSvcClient: mockNotificationRpc,
		config:                config,
	}

	req := types.CreateCommentRequest{
		Content:         "test comment",
		CommentableID:   1,
		CommentableType: database.CommentableTypeDiscussion,
		CurrentUser:     "user",
	}

	disc := database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user2",
			UUID:     "other-uuid",
		},
	}

	mockDiscussionStore.EXPECT().FindByID(mock.Anything, int64(1)).Return(&disc, nil).Once()
	mockRepoStore.EXPECT().FindById(mock.Anything, int64(1)).Return(&database.Repository{
		ID:   1,
		Path: "repo/path",
	}, nil).Once()
	mockRepoComponent.EXPECT().
		AllowReadAccessRepo(
			mock.Anything,
			mock.MatchedBy(func(repo *database.Repository) bool {
				return repo != nil && repo.ID == 1
			}),
			"user",
		).
		Return(true, nil).
		Once()

	user := &database.User{
		ID:       1,
		Username: "user",
		Avatar:   "avatar",
		UUID:     "test-uuid",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

	comment := database.Comment{
		Content:         req.Content,
		UserID:          user.ID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
	}
	mockDiscussionStore.EXPECT().CreateCommentWithMedia(mock.Anything, comment, req.MediaItems).Return(&comment, nil).Once()

	resp, err := comp.CreateDiscussionComment(context.TODO(), req)

	wg.Wait()

	require.Nil(t, err)
	require.Equal(t, int64(1), resp.CommentableID)
	require.Equal(t, "user", resp.User.Username)
}

func TestDiscussionComponent_CreatePendingCommentDoesNotNotify(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	mockNotificationRPC := mockrpc.NewMockNotificationSvcClient(t)
	notified := make(chan struct{}, 1)
	mockNotificationRPC.EXPECT().Send(mock.Anything, mock.Anything).
		Run(func(context.Context, *types.MessageRequest) { notified <- struct{}{} }).Return(nil).Maybe()

	cfg := &config.Config{}
	cfg.Notification.NotificationRetryCount = 1
	comp := &discussionComponentImpl{
		repoStore: mockRepoStore, userStore: mockUserStore, discussionStore: mockDiscussionStore,
		repoCompo: mockRepoComponent, notificationSvcClient: mockNotificationRPC, config: cfg,
	}
	req := types.CreateCommentRequest{
		Content: "pending", CommentableID: 1, CommentableType: database.CommentableTypeDiscussion,
		CurrentUser: "author", MediaItems: []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeVideo}},
	}
	discussion := database.Discussion{
		ID: 1, DiscussionableID: 1, DiscussionableType: database.DiscussionableTypeRepo,
		User: &database.User{Username: "owner", UUID: "owner-uuid"},
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, int64(1)).Return(&discussion, nil)
	repo := &database.Repository{ID: 1, Path: "repo/path"}
	mockRepoStore.EXPECT().FindById(mock.Anything, int64(1)).Return(repo, nil)
	mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, repo, "author").Return(true, nil)
	user := database.User{ID: 2, Username: "author", UUID: "author-uuid"}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "author").Return(user, nil)
	mockDiscussionStore.EXPECT().CreateCommentWithMedia(mock.Anything, mock.Anything, req.MediaItems).
		Return(&database.Comment{ID: 9, CommentableID: 1}, nil)

	resp, err := comp.CreateDiscussionComment(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Notification)
	require.Equal(t, int64(9), resp.Notification.CommentID)
	require.Equal(t, "author-uuid", resp.Notification.SenderUUID)
	require.Equal(t, "owner-uuid", resp.Notification.RecipientUUID)
	require.NotEmpty(t, resp.Notification.MsgUUID)
	select {
	case <-notified:
		t.Fatal("pending comment must not notify before moderation passes")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDiscussionComponent_UpdateComment(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	req := types.CreateCommentRequest{
		Content:         "test comment",
		CommentableID:   1,
		CommentableType: database.CommentableTypeDiscussion,
		CurrentUser:     "user",
	}
	user := &database.User{
		ID:       1,
		Username: "user",
		Avatar:   "avatar",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()
	disc := database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		User: &database.User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, int64(1)).Return(&disc, nil).Once()
	comment := database.Comment{
		ID:              1,
		Content:         req.Content,
		UserID:          user.ID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
	}
	mockDiscussionStore.EXPECT().FindCommentByID(mock.Anything, comment.ID).Return(&comment, nil).Once()
	mockDiscussionStore.EXPECT().UpdateComment(mock.Anything, comment.ID, "new comment").Return(nil).Once()

	err := comp.UpdateComment(context.TODO(), req.CurrentUser, comment.ID, "new comment")
	require.Nil(t, err)

}

func TestDiscussionComponent_DeleteComment(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	req := types.CreateCommentRequest{
		Content:         "test comment",
		CommentableID:   1,
		CommentableType: database.CommentableTypeDiscussion,
		CurrentUser:     "user",
	}
	user := &database.User{
		ID:       1,
		Username: "user",
		Avatar:   "avatar",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

	comment := database.Comment{
		ID:              1,
		Content:         req.Content,
		UserID:          user.ID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
	}
	mockDiscussionStore.EXPECT().FindCommentByID(mock.Anything, comment.ID).Return(&comment, nil).Once()
	mockDiscussionStore.EXPECT().DeleteComment(mock.Anything, comment.ID).Return(nil).Once()

	err := comp.DeleteComment(context.TODO(), req.CurrentUser, comment.ID)
	require.Nil(t, err)

}

func TestDiscussionComponent_ListDiscussionComments(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	mockRepoComponent := mockcomp.NewMockRepoComponent(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
		repoCompo:       mockRepoComponent,
	}

	discussionID := int64(1)
	comments := []database.Comment{
		{
			ID:      1,
			Content: "test comment",
			User: &database.User{
				ID:       1,
				Username: "user",
				Avatar:   "avatar",
			},
		},
		{
			ID:      2,
			Content: "test comment",
			User:    nil,
		},
	}
	disc := &database.Discussion{
		ID:                 1,
		Title:              "test discussion",
		DiscussionableID:   1,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             1,
		CommentCount:       2,
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, discussionID).Return(disc, nil).Once()
	mockRepoStore.EXPECT().FindById(mock.Anything, int64(1)).Return(&database.Repository{ID: 1}, nil).Once()
	mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, &database.Repository{ID: 1}, "user").Return(true, nil).Once()
	mockDiscussionStore.EXPECT().FindVisibleDiscussionComments(mock.Anything, database.FindVisibleCommentsReq{
		DiscussionID: discussionID, CurrentUser: "user", Per: 10, Page: 1,
	}).Return(comments, 2, nil).Once()

	resp, total, err := comp.ListDiscussionComments(context.TODO(), "user", discussionID, 10, 1)
	require.Nil(t, err)
	require.Len(t, resp, 2)
	require.Equal(t, comments[0].Content, resp[0].Content)
	require.NotNil(t, resp[0].User)
	require.Equal(t, "user", resp[0].User.Username)
	require.Equal(t, comments[1].Content, resp[1].Content)
	require.NotNil(t, resp[1].User)
	require.Equal(t, "deleted user", resp[1].User.Username)
	require.Equal(t, int64(0), resp[1].User.ID)
	require.Equal(t, 2, total)
}

func TestCreateRepoDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := types.CreateRepoDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, types.ScenarioCommentDetection, fields[0].Scenario)
}

func TestUpdateDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := types.UpdateDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, types.ScenarioCommentDetection, fields[0].Scenario)
}
func TestCreateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := types.CreateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, types.ScenarioCommentDetection, fields[0].Scenario)
}

func TestUpdateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := types.UpdateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, types.ScenarioCommentDetection, fields[0].Scenario)
}

// TestDiscussionComponent_MediaModerationVisibility covers the bug where a
// pending media comment was visible to everyone. A comment whose linked media
// is still under moderation must be hidden from non-authors and visible only
// to its author with PendingModeration=true. Two comments sharing one pending
// resource must both be hidden independently.
func TestDiscussionComponent_MediaModerationVisibility(t *testing.T) {
	author := &database.User{ID: 1, Username: "author", Avatar: "a"}
	other := &database.User{ID: 2, Username: "other", Avatar: "b"}

	t.Run("pending hidden from non-author, visible to author", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{
			{ID: 10, Content: "pending", User: author},
			{ID: 11, Content: "passed", User: other},
		}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{10, 11}).Return([]database.CommentMediaView{
			{CommentID: 10, DataID: "d", Status: database.MediaModerationStatusPending, MediaType: types.MediaTypeVideo},
			{CommentID: 11, DataID: "d2", Status: database.MediaModerationStatusPass, MediaType: types.MediaTypeAudio},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "author", comments)
		require.NoError(t, err)
		require.Len(t, resp, 2)
		var pending, passed *types.DiscussionResponse_Comment
		for _, c := range resp {
			if c.ID == 10 {
				pending = c
			}
			if c.ID == 11 {
				passed = c
			}
		}
		require.NotNil(t, pending)
		require.True(t, pending.PendingModeration)
		require.NotNil(t, passed)
		require.False(t, passed.PendingModeration)
	})

	t.Run("pending hidden from a different user", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{
			{ID: 10, Content: "pending", User: author},
			{ID: 11, Content: "passed", User: other},
		}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{10, 11}).Return([]database.CommentMediaView{
			{CommentID: 10, DataID: "d", Status: database.MediaModerationStatusPending, MediaType: types.MediaTypeVideo},
			{CommentID: 11, DataID: "d2", Status: database.MediaModerationStatusPass, MediaType: types.MediaTypeAudio},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "other", comments)
		require.NoError(t, err)
		require.Len(t, resp, 1)
		require.Equal(t, int64(11), resp[0].ID)
		require.False(t, resp[0].PendingModeration)
	})

	t.Run("two comments sharing one pending resource both hidden from non-authors", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{
			{ID: 20, Content: "first", User: author},
			{ID: 21, Content: "second", User: other},
		}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{20, 21}).Return([]database.CommentMediaView{
			{CommentID: 20, DataID: "shared", Status: database.MediaModerationStatusPending, MediaType: types.MediaTypeVideo},
			{CommentID: 21, DataID: "shared", Status: database.MediaModerationStatusPending, MediaType: types.MediaTypeVideo},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "", comments)
		require.NoError(t, err)
		require.Empty(t, resp)
	})

	t.Run("passed media visible to everyone", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{{ID: 30, Content: "passed", User: author}}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{30}).Return([]database.CommentMediaView{
			{CommentID: 30, DataID: "d", Status: database.MediaModerationStatusPass, MediaType: types.MediaTypeVideo},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "other", comments)
		require.NoError(t, err)
		require.Len(t, resp, 1)
		require.False(t, resp[0].PendingModeration)
	})

	t.Run("errored media still hidden (under moderation)", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{{ID: 40, Content: "errored", User: author}}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{40}).Return([]database.CommentMediaView{
			{CommentID: 40, DataID: "d", Status: database.MediaModerationStatusError, MediaType: types.MediaTypeVideo},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "other", comments)
		require.NoError(t, err)
		require.Empty(t, resp)
	})

	t.Run("rejected media stays hidden until comment is deleted", func(t *testing.T) {
		mockMediaStore := mockdb.NewMockMediaModerationStore(t)
		comp := &discussionComponentImpl{mediaStore: mockMediaStore}
		comments := []database.Comment{{ID: 41, Content: "rejected", User: author}}
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{41}).Return([]database.CommentMediaView{
			{CommentID: 41, DataID: "d", Status: database.MediaModerationStatusReject, MediaType: types.MediaTypeVideo},
		}, nil)

		resp, err := comp.buildCommentResponses(context.TODO(), "other", comments)
		require.NoError(t, err)
		require.Empty(t, resp)
	})
}

func TestDiscussionComponent_CommentUnderModeration(t *testing.T) {
	mockMediaStore := mockdb.NewMockMediaModerationStore(t)
	comp := &discussionComponentImpl{mediaStore: mockMediaStore}

	t.Run("pending returns true", func(t *testing.T) {
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{1}).Return([]database.CommentMediaView{
			{CommentID: 1, Status: database.MediaModerationStatusPending},
		}, nil)
		ok, err := comp.CommentUnderModeration(context.TODO(), 1)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("reject remains under moderation until deletion", func(t *testing.T) {
		mockMediaStore.EXPECT().FindMediaByCommentIDs(mock.Anything, []int64{2}).Return([]database.CommentMediaView{
			{CommentID: 2, Status: database.MediaModerationStatusPass},
			{CommentID: 2, Status: database.MediaModerationStatusReject},
		}, nil)
		ok, err := comp.CommentUnderModeration(context.TODO(), 2)
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestDiscussionComponent_LinkCommentMedia(t *testing.T) {
	mockMediaStore := mockdb.NewMockMediaModerationStore(t)
	comp := &discussionComponentImpl{mediaStore: mockMediaStore}
	items := []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}}
	mockMediaStore.EXPECT().LinkCommentMedia(mock.Anything, int64(77), items).Return(nil)
	require.NoError(t, comp.LinkCommentMedia(context.TODO(), 77, items))
}
