package component

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mq"
)

type discussionComponentImpl struct {
	repoCompo       RepoComponent
	discussionStore database.DiscussionStore
	repoStore       database.RepoStore
	userStore       database.UserStore
	sysMQ           mq.MessageQueue
}

type DiscussionComponent interface {
	CreateRepoDiscussion(ctx context.Context, req types.CreateRepoDiscussionRequest) (*types.CreateDiscussionResponse, error)
	GetDiscussion(ctx context.Context, id int64) (*types.ShowDiscussionResponse, error)
	UpdateDiscussion(ctx context.Context, req types.UpdateDiscussionRequest) error
	DeleteDiscussion(ctx context.Context, currentUser string, id int64) error
	ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest) (*types.ListRepoDiscussionResponse, error)
	CreateDiscussionComment(ctx context.Context, req types.CreateCommentRequest) (*types.CreateCommentResponse, error)
	UpdateComment(ctx context.Context, currentUser string, id int64, content string) error
	DeleteComment(ctx context.Context, currentUser string, id int64) error
	ListDiscussionComments(ctx context.Context, discussionID int64) ([]*types.DiscussionResponse_Comment, error)
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
		discussionStore: ds,
		repoStore:       rs,
		userStore:       us,
		sysMQ:           mq.SystemMQ,
		repoCompo:       repoCompo,
	}, nil
}

func (c *discussionComponentImpl) CreateRepoDiscussion(ctx context.Context, req types.CreateRepoDiscussionRequest) (*types.CreateDiscussionResponse, error) {
	//TODO:check if the user can access the repo

	//get repo by namespace and name
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
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

func (c *discussionComponentImpl) GetDiscussion(ctx context.Context, id int64) (*types.ShowDiscussionResponse, error) {
	discussion, err := c.discussionStore.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", id, err)
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
		return fmt.Errorf("user '%s' is not the owner of the discussion '%d'", req.CurrentUser, req.ID)
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
		return fmt.Errorf("user '%s' is not the owner of the discussion '%d'", currentUser, id)
	}
	err = c.discussionStore.DeleteByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete discussion by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest) (*types.ListRepoDiscussionResponse, error) {
	//TODO:check if the user can access the repo
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	discussions, err := c.discussionStore.FindByDiscussionableID(ctx, database.DiscussionableTypeRepo, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo discussions by repo type '%s', namespace '%s', name '%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	resp := &types.ListRepoDiscussionResponse{}
	for _, discussion := range discussions {
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
	// get discussion by  id
	discussion, err := c.discussionStore.FindByID(ctx, req.CommentableID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", req.CommentableID, err)
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

	if user.UUID != discussion.User.UUID && c.sysMQ != nil {
		repo, err := c.checkRepoReadAccess(ctx, discussion.DiscussionableID, req.CurrentUser)
		if err != nil {
			return nil, err
		}
		go func(repoType types.RepositoryType, repoPath string, senderUUID string, userUUIDs []string) {
			err = c.sendCommentMessage(ctx, repoType, repoPath, senderUUID, userUUIDs)
			if err != nil {
				slog.Error("failed to send comment message", slog.String("repoPath", repoPath), slog.String("repoType", repoPath),
					slog.String("senderUUID", senderUUID), slog.Any("userUUIDs", userUUIDs), slog.Any("err", err))
			}
		}(repo.RepositoryType, repo.Path, user.UUID, []string{discussion.User.UUID})
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
	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return fmt.Errorf("user '%s' is not the owner of the comment '%d'", currentUser, id)
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
		return fmt.Errorf("user '%s' is not the owner of the comment '%d'", currentUser, id)
	}
	err = c.discussionStore.DeleteComment(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) ListDiscussionComments(ctx context.Context, discussionID int64) ([]*types.DiscussionResponse_Comment, error) {
	comments, err := c.discussionStore.FindDiscussionComments(ctx, discussionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussionID, err)
	}
	resp := make([]*types.DiscussionResponse_Comment, 0, len(comments))
	for _, comment := range comments {
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

func (c *discussionComponentImpl) sendCommentMessage(ctx context.Context, repoType types.RepositoryType, repoPath string, senderUUID string, userUUIDs []string) error {
	repoTypeStr := string(repoType)
	title := fmt.Sprintf("New %s Comment", repoTypeStr)
	content := fmt.Sprintf("Your %s has a new comment. Join the conversation!", repoTypeStr)
	url := fmt.Sprintf("/%ss/%s/community", repoTypeStr, repoPath)
	if repoType == types.MCPServerRepo {
		url = fmt.Sprintf("/mcp/servers/%s/community", repoPath)
	}
	msg := types.NotificationMessage{
		MsgUUID:          uuid.New().String(),
		UserUUIDs:        userUUIDs,
		SenderUUID:       senderUUID,
		NotificationType: types.NotificationComment,
		Title:            title,
		Content:          content,
		CreateAt:         time.Now(),
		ClickActionURL:   url,
	}
	err := c.sysMQ.PublishSiteInternalMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to publish site msg, msg: %+v, err: %w", msg, err)
	}
	return nil
}
