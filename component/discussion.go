package component

import (
	"context"
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
	mediaStore            database.MediaModerationStore
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
	GetDiscussion(ctx context.Context, currentUser string, id int64, cPer int, cPage int) (*types.ShowDiscussionResponse, error)
	UpdateDiscussion(ctx context.Context, req types.UpdateDiscussionRequest) error
	DeleteDiscussion(ctx context.Context, currentUser string, id int64) error
	ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest, per int, page int) (*types.ListRepoDiscussionResponse, int, error)
	CheckDiscussionCommentAccess(ctx context.Context, currentUser string, discussionID int64) error
	CreateDiscussionComment(ctx context.Context, req types.CreateCommentRequest) (*types.CreateCommentResponse, error)
	CheckCommentOwnership(ctx context.Context, currentUser string, id int64) error
	UpdateComment(ctx context.Context, currentUser string, id int64, content string) error
	DeleteComment(ctx context.Context, currentUser string, id int64) error
	ListDiscussionComments(ctx context.Context, currentUser string, discussionID int64, per int, page int) ([]*types.DiscussionResponse_Comment, int, error)
	// LinkCommentMedia associates a comment with its pending media items so the
	// visibility logic and the poll workflow can find them by comment.
	LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error
	// CommentUnderModeration reports whether a comment still has non-terminal
	// media moderation rows linked to it. Such comments cannot be edited.
	CommentUnderModeration(ctx context.Context, commentID int64) (bool, error)
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
		discussionStore: ds, mediaStore: database.NewMediaModerationStore(), repoStore: rs, userStore: us,
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

func (c *discussionComponentImpl) GetDiscussion(ctx context.Context, currentUser string, id int64, cPer int, cPage int) (*types.ShowDiscussionResponse, error) {
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

	comments, visibleTotal, err := c.discussionStore.FindVisibleDiscussionComments(ctx, database.FindVisibleCommentsReq{
		DiscussionID: discussion.ID, CurrentUser: currentUser, Per: cPer, Page: cPage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussion.ID, err)
	}
	commentsData, err := c.buildCommentResponses(ctx, currentUser, comments)
	if err != nil {
		return nil, err
	}

	resp := &types.ShowDiscussionResponse{
		ID:    discussion.ID,
		Title: discussion.Title,
		User: &types.DiscussionResponse_User{
			ID:       discussion.User.ID,
			Username: discussion.User.Username,
			Avatar:   discussion.User.Avatar,
		},
		Comments: &types.CommentsWithPagination{
			Data:  commentsData,
			Total: visibleTotal,
			Page:  cPage,
			Per:   cPer,
		},
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

func (c *discussionComponentImpl) ListRepoDiscussions(ctx context.Context, req types.ListRepoDiscussionRequest, per int, page int) (*types.ListRepoDiscussionResponse, int, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}

	_, err = c.checkRepoReadAccess(ctx, repo.ID, req.CurrentUser)
	if err != nil {
		return nil, 0, err
	}

	discussions, total, err := c.discussionStore.FindByDiscussionableID(ctx, database.DiscussionableTypeRepo, repo.ID, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list repo discussions by repo type '%s', namespace '%s', name '%s': %w", req.RepoType, req.Namespace, req.Name, err)
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
	return resp, total, nil
}

func (c *discussionComponentImpl) CreateDiscussionComment(ctx context.Context, req types.CreateCommentRequest) (*types.CreateCommentResponse, error) {
	req.CommentableType = database.CommentableTypeDiscussion
	discussion, repo, user, err := c.resolveDiscussionCommentAccess(ctx, req.CurrentUser, req.CommentableID)
	if err != nil {
		return nil, err
	}
	// create comment
	comment, err := c.discussionStore.CreateCommentWithMedia(ctx, database.Comment{
		Content:         req.Content,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
		UserID:          user.ID,
	}, req.MediaItems)
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion comment: %w", err)
	}
	var notification *types.CommentNotification
	if user.UUID != discussion.User.UUID {
		notification = &types.CommentNotification{
			CommentID: comment.ID, MsgUUID: uuid.NewString(), RepoType: repo.RepositoryType, RepoPath: repo.Path,
			SenderUUID: user.UUID, RecipientUUID: discussion.User.UUID, CreatedAt: comment.CreatedAt,
		}
	}
	// Pending comments defer this stable notification payload to the moderation
	// workflow; ordinary comments preserve the existing immediate behavior.
	if len(req.MediaItems) == 0 && notification != nil {
		go func() {
			notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := c.sendCommentMessage(notificationCtx, *notification); err != nil {
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
		Notification: notification,
	}, nil
}

func (c *discussionComponentImpl) CheckDiscussionCommentAccess(ctx context.Context, currentUser string, discussionID int64) error {
	_, _, _, err := c.resolveDiscussionCommentAccess(ctx, currentUser, discussionID)
	return err
}

func (c *discussionComponentImpl) resolveDiscussionCommentAccess(
	ctx context.Context,
	currentUser string,
	discussionID int64,
) (*database.Discussion, *database.Repository, database.User, error) {
	discussion, err := c.discussionStore.FindByID(ctx, discussionID)
	if err != nil {
		return nil, nil, database.User{}, fmt.Errorf("failed to find discussion by id '%d': %w", discussionID, err)
	}
	if discussion.DiscussionableType != database.DiscussionableTypeRepo {
		return nil, nil, database.User{}, fmt.Errorf("discussion '%d' is not a repo discussion", discussion.ID)
	}
	repo, err := c.checkRepoReadAccess(ctx, discussion.DiscussionableID, currentUser)
	if err != nil {
		return nil, nil, database.User{}, err
	}
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, nil, database.User{}, fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	return discussion, repo, user, nil
}

func (c *discussionComponentImpl) UpdateComment(ctx context.Context, currentUser string, id int64, content string) error {
	comment, err := c.findOwnedComment(ctx, currentUser, id)
	if err != nil {
		return err
	}
	_, err = c.discussionStore.FindByID(ctx, comment.CommentableID)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", comment.CommentableID, err)
	}
	if err := c.discussionStore.UpdateComment(ctx, id, content); err != nil {
		return fmt.Errorf("failed to update comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *discussionComponentImpl) CheckCommentOwnership(ctx context.Context, currentUser string, id int64) error {
	_, err := c.findOwnedComment(ctx, currentUser, id)
	return err
}

func (c *discussionComponentImpl) findOwnedComment(ctx context.Context, currentUser string, id int64) (*database.Comment, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	//get comment by id
	comment, err := c.discussionStore.FindCommentByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find comment by id '%d': %w", id, err)
	}

	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return nil, errorx.ErrForbiddenMsg(fmt.Sprintf("user '%s' is not the owner of the comment '%d'", currentUser, id))
	}
	return comment, nil
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

func (c *discussionComponentImpl) ListDiscussionComments(ctx context.Context, currentUser string, discussionID int64, per int, page int) ([]*types.DiscussionResponse_Comment, int, error) {
	// Get discussion by id
	discussion, err := c.discussionStore.FindByID(ctx, discussionID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find discussion by id '%d': %w", discussionID, err)
	}
	//TOOD: support other discussionable type, like collection
	if discussion.DiscussionableType != database.DiscussionableTypeRepo {
		return nil, 0, fmt.Errorf("discussion '%d' is not a repo discussion", discussion.ID)
	}
	// Get the repository associated with the discussion
	_, err = c.checkRepoReadAccess(ctx, discussion.DiscussionableID, currentUser)
	if err != nil {
		return nil, 0, err
	}

	comments, visibleTotal, err := c.discussionStore.FindVisibleDiscussionComments(ctx, database.FindVisibleCommentsReq{
		DiscussionID: discussionID, CurrentUser: currentUser, Per: per, Page: page,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussionID, err)
	}
	resp, err := c.buildCommentResponses(ctx, currentUser, comments)
	if err != nil {
		return nil, 0, err
	}
	return resp, visibleTotal, nil
}

func (c *discussionComponentImpl) sendCommentMessage(ctx context.Context, notification types.CommentNotification) error {
	repoUrl := GetRepoUrl(notification.RepoType, notification.RepoPath)
	url := fmt.Sprintf("%s/community", repoUrl)

	msg := types.NotificationMessage{
		MsgUUID:          notification.MsgUUID,
		UserUUIDs:        []string{notification.RecipientUUID},
		SenderUUID:       notification.SenderUUID,
		NotificationType: types.NotificationComment,
		CreateAt:         notification.CreatedAt,
		ClickActionURL:   url,
		Template:         string(types.MessageScenarioDiscussion),
		Payload: map[string]any{
			"repo_type": notification.RepoType,
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

// LinkCommentMedia links a comment to the media moderation tasks it depends on
// by inserting idempotent comment_media rows so the visibility logic and the
// poll workflow can find them by comment.
func (c *discussionComponentImpl) LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error {
	if c.mediaStore == nil {
		return fmt.Errorf("media moderation store is not configured")
	}
	if len(items) == 0 {
		return nil
	}
	return c.mediaStore.LinkCommentMedia(ctx, commentID, items)
}

// CommentUnderModeration reports whether the comment still has at least one
// non-terminal media_moderations row linked through comment_media. Such
// comments are visible only to their author and cannot be edited.
func (c *discussionComponentImpl) CommentUnderModeration(ctx context.Context, commentID int64) (bool, error) {
	if c.mediaStore == nil {
		return false, nil
	}
	views, err := c.mediaStore.FindMediaByCommentIDs(ctx, []int64{commentID})
	if err != nil {
		return false, fmt.Errorf("find media moderation rows for comment %d: %w", commentID, err)
	}
	for _, view := range views {
		if view.Status != database.MediaModerationStatusPass {
			// Only pass is publishable. Reject remains hidden until finalize
			// deletes the comment; pending/error/unknown are also fail-closed.
			return true, nil
		}
	}
	return false, nil
}

// buildCommentResponses maps stored comments to API responses, applying the
// media-moderation visibility rule: a comment that is still under moderation
// (any linked media is non-terminal) is hidden from everyone except its
// author. The author additionally sees a pending_moderation flag so the
// client can explain the state.
func (c *discussionComponentImpl) buildCommentResponses(ctx context.Context, currentUser string, comments []database.Comment) ([]*types.DiscussionResponse_Comment, error) {
	if len(comments) == 0 {
		return []*types.DiscussionResponse_Comment{}, nil
	}
	commentIDs := make([]int64, 0, len(comments))
	for _, comment := range comments {
		commentIDs = append(commentIDs, comment.ID)
	}
	underModeration := make(map[int64]bool)
	if c.mediaStore != nil {
		views, err := c.mediaStore.FindMediaByCommentIDs(ctx, commentIDs)
		if err != nil {
			return nil, fmt.Errorf("find media moderation rows for comments: %w", err)
		}
		for _, view := range views {
			if view.Status != database.MediaModerationStatusPass {
				underModeration[view.CommentID] = true
			}
		}
	}

	resp := make([]*types.DiscussionResponse_Comment, 0, len(comments))
	for _, comment := range comments {
		pending := underModeration[comment.ID]
		// A pending comment is visible only to its author.
		if pending {
			if comment.User == nil || comment.User.Username != currentUser {
				continue
			}
		}
		var user *types.DiscussionResponse_User
		if comment.User == nil {
			user = &types.DiscussionResponse_User{
				ID:       0,
				Username: "deleted user",
				Avatar:   "",
			}
		} else {
			user = &types.DiscussionResponse_User{
				ID:       comment.User.ID,
				Username: comment.User.Username,
				Avatar:   comment.User.Avatar,
			}
		}
		resp = append(resp, &types.DiscussionResponse_Comment{
			ID:                comment.ID,
			Content:           comment.Content,
			User:              user,
			CreatedAt:         comment.CreatedAt,
			PendingModeration: pending,
		})
	}
	return resp, nil
}
