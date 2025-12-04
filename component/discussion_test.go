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
	// Updated to include pagination parameters: per=10, page=1
	mockDiscussionStore.EXPECT().FindDiscussionComments(mock.Anything, int64(1), 10, 1).Return(comments, 1, nil).Once()

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
	mockDiscussionStore.EXPECT().CreateComment(mock.Anything, comment).Return(&comment, nil).Once()

	resp, err := comp.CreateDiscussionComment(context.TODO(), req)

	wg.Wait()

	require.Nil(t, err)
	require.Equal(t, int64(1), resp.CommentableID)
	require.Equal(t, "user", resp.User.Username)
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
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, discussionID).Return(disc, nil).Once()
	mockRepoStore.EXPECT().FindById(mock.Anything, int64(1)).Return(&database.Repository{ID: 1}, nil).Once()
	mockRepoComponent.EXPECT().AllowReadAccessRepo(mock.Anything, &database.Repository{ID: 1}, "user").Return(true, nil).Once()
	mockDiscussionStore.EXPECT().FindDiscussionComments(mock.Anything, discussionID, 10, 1).Return(comments, 1, nil).Once()

	resp, total, err := comp.ListDiscussionComments(context.TODO(), "user", discussionID, 10, 1)
	require.Nil(t, err)
	require.Len(t, resp, 1)
	require.Equal(t, comments[0].Content, resp[0].Content)
	require.Equal(t, 1, total)
}

func TestCreateRepoDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := types.CreateRepoDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}

func TestUpdateDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := types.UpdateDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}
func TestCreateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := types.CreateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}

func TestUpdateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := types.UpdateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}
