package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type discussionComponentImpl struct {
	repoCompo             RepoComponent
	discussionStore       database.DiscussionStore
	repoStore             database.RepoStore
	userStore             database.UserStore
	notificationSvcClient rpc.NotificationSvcClient
	config                *config.Config
}

func (c *discussionComponentImpl) checkRepoReadAccess(ctx context.Context, repoID int64, currentUser string) (*database.Repository, error) {
	repo, err := c.repoStore.FindById(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to find repository by id '%d': %w", repoID, err)
	}

	// Check if the user has read access to the repository
	allow, err := c.repoCompo.AllowReadAccessRepo(ctx, repo, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user can access repo: %w", err)
	}
	if !allow {
		return nil, errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' does not have access to repository '%s'", currentUser, repo.Path))
	}

	return repo, nil
}

type DiscussionComponent interface {
	CreateRepoDiscussion(ctx context.Context, req types.CreateRepoDiscussionRequest) (*types.CreateDiscussionResponse, error)
	GetDiscussion(ctx context.Context, currentUser string, id int64) (*types.ShowDiscussionResponse, error)
	UpdateDiscussion(ctx context.Context, req types.UpdateDiscussionRequest) error
	DeleteDiscussion(ctx context.Context, currentUser string, id int64) error
	ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest) (*types.ListRepoDiscussionResponse, error)
	CreateDiscussionComment(ctx context.Context, req types.CreateCommentRequest) (*types.CreateCommentResponse, error)
	UpdateComment(ctx context.Context, currentUser string, id int64, content string) error
	DeleteComment(ctx context.Context, currentUser string, id int64) error
	ListDiscussionComments(ctx context.Context, currentUser string, discussionID int64) ([]*types.DiscussionResponse_Comment, error)
}

func NewDiscussionComponent(config *config.Config) (DiscussionComponent, error) {
	repoCompo, err := NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	ds := database.NewDiscussionStore()
	rs := database.NewRepoStore()
	us := database.NewUserStore()
	return &discussionComponentImpl{
		repoCompo:       repoCompo,
		discussionStore: ds, repoStore: rs, userStore: us,
		notificationSvcClient: rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
			rpc.AuthWithApiKey(config.APIToken)),
		config: config,
	}, nil
}

func (c *discussionComponentImpl) CreateRepoDiscussion(ctx context.Context, req types.CreateRepoDiscussionRequest) (*types.CreateDiscussionResponse, error) {
	//get repo by namespace and name
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	_, err = c.checkRepoReadAccess(ctx, repo.ID, req.CurrentUser)
	if err != nil {
		return nil, err
	}
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	discussion, err := c.discussionStore.Create(ctx, database.Discussion{
		Title:              req.Title,
		DiscussionableID:   repo.ID,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             user.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion: %w", err)
	}
	resp := &types.CreateDiscussionResponse{
		ID: discussion.ID,
		User: &types.DiscussionResponse_User{
			ID:       user.ID,
			Username: user.Username,
			Avatar:   user.Avatar,
		},
		Title:        discussion.Title,
		CommentCount: discussion.CommentCount,
		CreatedAt:    discussion.CreatedAt,
	}
	return resp, nil
}

func (c *discussionComponentImpl) GetDiscussion(ctx context.Context, currentUser string, id int64) (*types.ShowDiscussionResponse, error) {
	discussion, err := c.discussionStore.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", id, err)
	}

	//TOOD: support other discussionable type, like collection
	if discussion.DiscussionableType != database.DiscussionableTypeRepo {
		return nil, fmt.Errorf("discussion '%d' is not a repo discussion", id)
	}

	_, err = c.checkRepoReadAccess(ctx, discussion.DiscussionableID, currentUser)
	if err != nil {
		return nil, err
	}

	comments, err := c.discussionStore.FindDiscussionComments(ctx, discussion.ID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussion.ID, err)
	}
	resp := &types.ShowDiscussionResponse{
		ID:    discussion.ID,
		Title: discussion.Title,
		User: &types.DiscussionResponse_User{
			ID:       discussion.User.ID,
			Username: discussion.User.Username,
			Avatar:   discussion.User.Avatar,
		},
	}
	for _, comment := range comments {
		resp.Comments = append(resp.Comments, &types.DiscussionResponse_Comment{
			ID:      comment.ID,
			Content: comment.Content,
			User: &types.DiscussionResponse_User{
				ID:       comment.User.ID,
				Username: comment.User.Username,
				Avatar:   comment.User.Avatar,
			},
		})
	}
	return resp, nil
}

func (c *discussionComponentImpl) UpdateDiscussion(ctx context.Context, req types.UpdateDiscussionRequest) error {
	//check if the user is the owner of the discussion
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	discussion, err := c.discussionStore.FindByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", req.ID, err)
	}

	if discussion.UserID != user.ID {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' is not the owner of the discussion '%d'", req.CurrentUser, req.ID))
	}
	err = c.discussionStore.UpdateByID(ctx, req.ID, req.Title)
	if err != nil {
		return fmt.Errorf("failed to update discussion by id '%d': %w", req.ID, err)
	}
	return nil
}

func (c *discussionComponentImpl) DeleteDiscussion(ctx context.Context, currentUser string, id int64) error {
	discussion, err := c.discussionStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", id, err)
	}
	if discussion.User.Username != currentUser {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' is not the owner of the discussion '%d'", currentUser, id))
	}
	err = c.discussionStore.DeleteByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete discussion by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest) (*types.ListRepoDiscussionResponse, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}

	_, err = c.checkRepoReadAccess(ctx, repo.ID, req.CurrentUser)
	if err != nil {
		return nil, err
	}

	discussions, err := c.discussionStore.FindByDiscussionableID(ctx, database.DiscussionableTypeRepo, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo discussions by repo type '%s', namespace '%s', name '%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	resp := &types.ListRepoDiscussionResponse{}
	for _, discussion := range discussions {
		if discussion.User == nil {
			continue
		}
		resp.Discussions = append(resp.Discussions, &types.CreateDiscussionResponse{
			ID:           discussion.ID,
			Title:        discussion.Title,
			CommentCount: discussion.CommentCount,
			CreatedAt:    discussion.CreatedAt,
			User: &types.DiscussionResponse_User{
				ID:       discussion.User.ID,
				Username: discussion.User.Username,
				Avatar:   discussion.User.Avatar,
			},
		})
	}
	return resp, nil
}

func (c *discussionComponentImpl) CreateDiscussionComment(ctx context.Context, req types.CreateCommentRequest) (*types.CreateCommentResponse, error) {
	req.CommentableType = database.CommentableTypeDiscussion
	// get discussion by id
	discussion, err := c.discussionStore.FindByID(ctx, req.CommentableID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", req.CommentableID, err)
	}

	//TOOD: support other discussionable type, like collection
	if discussion.DiscussionableType != database.DiscussionableTypeRepo {
		return nil, fmt.Errorf("discussion '%d' is not a repo discussion", discussion.ID)
	}

	repo, err := c.checkRepoReadAccess(ctx, discussion.DiscussionableID, req.CurrentUser)
	if err != nil {
		return nil, err
	}

	//get user by username
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	// create comment
	comment, err := c.discussionStore.CreateComment(ctx, database.Comment{
		Content:         req.Content,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
		UserID:          user.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion comment: %w", err)
	}
	if user.UUID != discussion.User.UUID {
		go func() {
			notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err = c.sendCommentMessage(notificationCtx, repo.RepositoryType, repo.Path, user.UUID, []string{discussion.User.UUID})
			if err != nil {
				slog.Error("failed to send comment message", slog.String("repoPath", repo.Path), slog.String("repoType", string(repo.RepositoryType)),
					slog.String("senderUUID", user.UUID), slog.Any("userUUIDs", []string{discussion.User.UUID}), slog.Any("err", err))
			}
		}()
	}
	return &types.CreateCommentResponse{
		ID:              comment.ID,
		CommentableID:   comment.CommentableID,
		CommentableType: comment.CommentableType,
		CreatedAt:       comment.CreatedAt,
		User: &types.DiscussionResponse_User{
			ID:       user.ID,
			Username: user.Username,
			Avatar:   user.Avatar,
		},
	}, nil
}

func (c *discussionComponentImpl) UpdateComment(ctx context.Context, currentUser string, id int64, content string) error {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	//get comment by id
	comment, err := c.discussionStore.FindCommentByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find comment by id '%d': %w", id, err)
	}

	// Get the discussion associated with the comment
	_, err = c.discussionStore.FindByID(ctx, comment.CommentableID)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", comment.CommentableID, err)
	}

	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' is not the owner of the comment '%d'", currentUser, id))
	}
	err = c.discussionStore.UpdateComment(ctx, id, content)
	if err != nil {
		return fmt.Errorf("failed to update comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) DeleteComment(ctx context.Context, currentUser string, id int64) error {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	//get comment by id
	comment, err := c.discussionStore.FindCommentByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find comment by id '%d': %w", id, err)
	}
	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' is not the owner of the comment '%d'", currentUser, id))
	}
	err = c.discussionStore.DeleteComment(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) ListDiscussionComments(ctx context.Context, currentUser string, discussionID int64) ([]*types.DiscussionResponse_Comment, error) {
	// Get discussion by id
	discussion, err := c.discussionStore.FindByID(ctx, discussionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", discussionID, err)
	}
	//TOOD: support other discussionable type, like collection
	if discussion.DiscussionableType != database.DiscussionableTypeRepo {
		return nil, fmt.Errorf("discussion '%d' is not a repo discussion", discussion.ID)
	}
	// Get the repository associated with the discussion
	_, err = c.checkRepoReadAccess(ctx, discussion.DiscussionableID, currentUser)
	if err != nil {
		return nil, err
	}

	comments, err := c.discussionStore.FindDiscussionComments(ctx, discussionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussionID, err)
	}
	resp := make([]*types.DiscussionResponse_Comment, 0, len(comments))
	for _, comment := range comments {
		if comment.User == nil {
			continue
		}
		resp = append(resp, &types.DiscussionResponse_Comment{
			ID:      comment.ID,
			Content: comment.Content,
			User: &types.DiscussionResponse_User{
				ID:       comment.User.ID,
				Username: comment.User.Username,
				Avatar:   comment.User.Avatar,
			},
			CreatedAt: comment.CreatedAt,
		})
	}
	return resp, nil
}

func (c *discussionComponentImpl) sendCommentMessage(ctx context.Context, repoType types.RepositoryType, repoPath string, senderUUID string, userUUIDs []string) error {
	repoUrl := GetRepoUrl(repoType, repoPath)
	url := fmt.Sprintf("%s/community", repoUrl)

	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        userUUIDs,
		SenderUUID:       senderUUID,
		NotificationType: types.NotificationComment,
		CreateAt:         time.Now(),
		ClickActionURL:   url,
		Template:         string(types.MessageScenarioDiscussion),
		Payload: map[string]any{
			"repo_type": repoType,
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}
	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioDiscussion,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := c.config.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = c.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}
	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}
	return nil
}
