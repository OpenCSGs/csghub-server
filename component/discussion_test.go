package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDiscussionComponent_CreateDisucssion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	repo := &database.Repository{
		ID: 1,
	}
	mockRepoStore.EXPECT().FindByPath(mock.Anything, types.ModelRepo, "namespace", "name").Return(repo, nil).Once()

	user := &database.User{
		ID:       1,
		Username: "user",
		Avatar:   "avatar",
	}
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

	req := CreateRepoDiscussionRequest{
		Title:       "test discussion",
		RepoType:    "model",
		Namespace:   "namespace",
		Name:        "name",
		CurrentUser: "user",
	}
	actualDisc, err := comp.CreateRepoDiscussion(context.TODO(), req)
	require.Nil(t, err)

	expectedDisc := &CreateDiscussionResponse{
		ID:           1,
		Title:        "test discussion",
		CommentCount: 0,
		CreatedAt:    dbdisc.CreatedAt,
		User: &DiscussionResponse_User{
			ID:       1,
			Username: "user",
			Avatar:   "avatar",
		},
	}
	require.Equal(t, expectedDisc, actualDisc)

}

func TestDiscussionComponent_GetDisussion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
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
	mockDiscussionStore.EXPECT().FindDiscussionComments(mock.Anything, int64(1)).Return(comments, nil).Once()

	resp, err := comp.GetDiscussion(context.TODO(), int64(1))
	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(1), resp.ID)
	require.Equal(t, "test discussion", resp.Title)
	// require.Equal(t, int64(1), resp.CommentCount)
	require.Equal(t, "test comment", resp.Comments[0].Content)
	require.Equal(t, "user", resp.Comments[0].User.Username)
	require.Equal(t, "avatar", resp.Comments[0].User.Avatar)

}

func TestDiscussionComponent_UpdateDisussion(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	req := UpdateDiscussionRequest{
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
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
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
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	repo := &database.Repository{
		ID: 1,
	}
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
	mockDiscussionStore.EXPECT().FindByDiscussionableID(mock.Anything, database.DiscussionableTypeRepo, repo.ID).Return(discussions, nil).Once()

	resp, err := comp.ListRepoDiscussions(context.TODO(), ListRepoDiscussionRequest{
		RepoType:    types.ModelRepo,
		Namespace:   "namespace",
		Name:        "name",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Len(t, resp.Discussions, 1)

}

func TestDiscussionComponent_CreateDisussionComment(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	req := CreateCommentRequest{
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
			Username: "user",
			Avatar:   "avatar",
		},
	}
	mockDiscussionStore.EXPECT().FindByID(mock.Anything, int64(1)).Return(&disc, nil).Once()

	user := &database.User{
		ID:       1,
		Username: "user",
		Avatar:   "avatar",
	}
	mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

	comment := database.Comment{
		// ID:              1,
		Content:         req.Content,
		UserID:          user.ID,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
	}
	mockDiscussionStore.EXPECT().CreateComment(mock.Anything, comment).Return(&comment, nil).Once()

	resp, err := comp.CreateDiscussionComment(context.TODO(), req)
	require.Nil(t, err)
	require.Equal(t, int64(1), resp.CommentableID)
	require.Equal(t, "user", resp.User.Username)

}

func TestDiscussionComponent_UpdateComment(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockUserStore := mockdb.NewMockUserStore(t)
	mockDiscussionStore := mockdb.NewMockDiscussionStore(t)
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
	}

	req := CreateCommentRequest{
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

	req := CreateCommentRequest{
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
	// new discussionComponentImpl from mock db store
	comp := &discussionComponentImpl{
		repoStore:       mockRepoStore,
		userStore:       mockUserStore,
		discussionStore: mockDiscussionStore,
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
	}
	mockDiscussionStore.EXPECT().FindDiscussionComments(mock.Anything, discussionID).Return(comments, nil).Once()

	resp, err := comp.ListDiscussionComments(context.TODO(), discussionID)
	require.Nil(t, err)
	require.Len(t, resp, 1)
	require.Equal(t, comments[0].Content, resp[0].Content)

}

func TestCreateRepoDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := CreateRepoDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}

func TestUpdateDiscussionRequest_GetSensitiveFields(t *testing.T) {

	req := UpdateDiscussionRequest{
		Title: "title",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "title", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}
func TestCreateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := CreateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}

func TestUpdateCommentRequest_GetSensitiveFields(t *testing.T) {

	req := UpdateCommentRequest{
		Content: "content",
	}
	fields := req.GetSensitiveFields()
	require.Len(t, fields, 1)
	require.Equal(t, "content", fields[0].Value())
	require.Equal(t, "comment_detection", fields[0].Scenario)
}
