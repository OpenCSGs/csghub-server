package component

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type DiscussionComponent struct {
	ds *database.DiscussionStore
	rs *database.RepoStore
	us *database.UserStore
}

func NewDiscussionComponent() *DiscussionComponent {
	ds := database.NewDiscussionStore()
	rs := database.NewRepoStore()
	us := database.NewUserStore()
	return &DiscussionComponent{ds: ds, rs: rs, us: us}
}

func (c *DiscussionComponent) CreateRepoDiscussion(ctx context.Context, req CreateRepoDiscussionRequest) (*CreateDiscussionResponse, error) {
	//TODO:check if the user can access the repo

	//get repo by namespace and name
	repo, err := c.rs.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	discussion, err := c.ds.Create(ctx, database.Discussion{
		Title:              req.Title,
		DiscussionableID:   repo.ID,
		DiscussionableType: database.DiscussionableTypeRepo,
		UserID:             user.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion: %w", err)
	}
	resp := &CreateDiscussionResponse{
		ID: discussion.ID,
		User: &DiscussionResponse_User{
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

func (c *DiscussionComponent) GetDiscussion(ctx context.Context, id int64) (*ShowDiscussionResponse, error) {
	discussion, err := c.ds.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", id, err)
	}
	comments, err := c.ds.FindDiscussionComments(ctx, discussion.ID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussion.ID, err)
	}
	resp := &ShowDiscussionResponse{
		ID:    discussion.ID,
		Title: discussion.Title,
		User: &DiscussionResponse_User{
			ID:       discussion.User.ID,
			Username: discussion.User.Username,
			Avatar:   discussion.User.Avatar,
		},
	}
	for _, comment := range comments {
		resp.Comments = append(resp.Comments, &DiscussionResponse_Comment{
			ID:      comment.ID,
			Content: comment.Content,
			User: &DiscussionResponse_User{
				ID:       comment.User.ID,
				Username: comment.User.Username,
				Avatar:   comment.User.Avatar,
			},
		})
	}
	return resp, nil
}

func (c *DiscussionComponent) UpdateDiscussion(ctx context.Context, req UpdateDiscussionRequest) error {
	//check if the user is the owner of the discussion
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	discussion, err := c.ds.FindByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", req.ID, err)
	}
	if discussion.UserID != user.ID {
		return fmt.Errorf("user '%s' is not the owner of the discussion '%d'", req.CurrentUser, req.ID)
	}
	err = c.ds.UpdateByID(ctx, req.ID, req.Title)
	if err != nil {
		return fmt.Errorf("failed to update discussion by id '%d': %w", req.ID, err)
	}
	return nil
}

func (c *DiscussionComponent) DeleteDiscussion(ctx context.Context, currentUser string, id int64) error {
	discussion, err := c.ds.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find discussion by id '%d': %w", id, err)
	}
	if discussion.User.Username != currentUser {
		return fmt.Errorf("user '%s' is not the owner of the discussion '%d'", currentUser, id)
	}
	err = c.ds.DeleteByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete discussion by id '%d': %w", id, err)
	}
	return nil
}

func (c *DiscussionComponent) ListRepoDiscussions(ctx context.Context, req ListRepoDiscussionRequest) (*ListRepoDiscussionResponse, error) {
	//TODO:check if the user can access the repo
	repo, err := c.rs.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo by path '%s/%s/%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	discussions, err := c.ds.FindByDiscussionableID(ctx, database.DiscussionableTypeRepo, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo discussions by repo type '%s', namespace '%s', name '%s': %w", req.RepoType, req.Namespace, req.Name, err)
	}
	resp := &ListRepoDiscussionResponse{}
	for _, discussion := range discussions {
		resp.Discussions = append(resp.Discussions, &CreateDiscussionResponse{
			ID:           discussion.ID,
			Title:        discussion.Title,
			CommentCount: discussion.CommentCount,
			CreatedAt:    discussion.CreatedAt,
			User: &DiscussionResponse_User{
				ID:       discussion.User.ID,
				Username: discussion.User.Username,
				Avatar:   discussion.User.Avatar,
			},
		})
	}
	return resp, nil
}

func (c *DiscussionComponent) CreateDiscussionComment(ctx context.Context, req CreateCommentRequest) (*CreateCommentResponse, error) {
	req.CommentableType = database.CommentableTypeDiscussion
	// get discussion by  id
	_, err := c.ds.FindByID(ctx, req.CommentableID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion by id '%d': %w", req.CommentableID, err)
	}

	//get user by username
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by username '%s': %w", req.CurrentUser, err)
	}
	// create comment
	comment, err := c.ds.CreateComment(ctx, database.Comment{
		Content:         req.Content,
		CommentableID:   req.CommentableID,
		CommentableType: req.CommentableType,
		UserID:          user.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion comment: %w", err)
	}
	return &CreateCommentResponse{
		ID:              comment.ID,
		CommentableID:   comment.CommentableID,
		CommentableType: comment.CommentableType,
		CreatedAt:       comment.CreatedAt,
		User: &DiscussionResponse_User{
			ID:       user.ID,
			Username: user.Username,
			Avatar:   user.Avatar,
		},
	}, nil
}

func (c *DiscussionComponent) UpdateComment(ctx context.Context, currentUser string, id int64, content string) error {
	user, err := c.us.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	//get comment by id
	comment, err := c.ds.FindCommentByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find comment by id '%d': %w", id, err)
	}
	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return fmt.Errorf("user '%s' is not the owner of the comment '%d'", currentUser, id)
	}
	err = c.ds.UpdateComment(ctx, id, content)
	if err != nil {
		return fmt.Errorf("failed to update comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *DiscussionComponent) DeleteComment(ctx context.Context, currentUser string, id int64) error {
	user, err := c.us.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("failed to find user by username '%s': %w", currentUser, err)
	}
	//get comment by id
	comment, err := c.ds.FindCommentByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find comment by id '%d': %w", id, err)
	}
	//check if the user is the owner of the comment
	if comment.UserID != user.ID {
		return fmt.Errorf("user '%s' is not the owner of the comment '%d'", currentUser, id)
	}
	err = c.ds.DeleteComment(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete comment by id '%d': %w", id, err)
	}
	return nil
}

func (c *DiscussionComponent) ListDiscussionComments(ctx context.Context, discussionID int64) ([]*DiscussionResponse_Comment, error) {
	comments, err := c.ds.FindDiscussionComments(ctx, discussionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find discussion comments by discussion id '%d': %w", discussionID, err)
	}
	resp := make([]*DiscussionResponse_Comment, 0, len(comments))
	for _, comment := range comments {
		resp = append(resp, &DiscussionResponse_Comment{
			ID:      comment.ID,
			Content: comment.Content,
			User: &DiscussionResponse_User{
				ID:       comment.User.ID,
				Username: comment.User.Username,
				Avatar:   comment.User.Avatar,
			},
			CreatedAt: comment.CreatedAt,
		})
	}
	return resp, nil
}

//--- request and response ---//

type CreateRepoDiscussionRequest struct {
	Title       string               `json:"title" binding:"required"`
	RepoType    types.RepositoryType `json:"-"`
	Namespace   string               `json:"-"`
	Name        string               `json:"-"`
	CurrentUser string               `json:"-"`
}

type CreateDiscussionResponse struct {
	ID    int64                    `json:"id"`
	User  *DiscussionResponse_User `json:"user"`
	Title string                   `json:"title"`
	// DiscussionableID   int64     `json:"discussionable_id"`
	// DiscussionableType string    `json:"discussionable_type"`
	CommentCount int64     `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
	// UpdatedAt    time.Time `json:"updated_at"`
}

type DiscussionResponse_User struct {
	ID       int64  `json:"id"`
	Username string `json:"name"`
	Avatar   string `json:"avatar"`
}

type UpdateDiscussionRequest struct {
	ID          int64  `json:"-"`
	Title       string `json:"title" binding:"required"`
	CurrentUser string `json:"-"`
}

type ShowDiscussionResponse struct {
	ID           int64                         `json:"id"`
	Title        string                        `json:"title"`
	User         *DiscussionResponse_User      `json:"user"`
	CommentCount int64                         `json:"comment_count"`
	Comments     []*DiscussionResponse_Comment `json:"comments,omitempty"`
}

type DiscussionResponse_Comment struct {
	ID        int64                    `json:"id"`
	Content   string                   `json:"content"`
	User      *DiscussionResponse_User `json:"user"`
	CreatedAt time.Time                `json:"created_at"`
}

type ListRepoDiscussionRequest struct {
	RepoType    types.RepositoryType `json:"-"`
	Namespace   string               `json:"-"`
	Name        string               `json:"-"`
	CurrentUser string               `json:"-"`
}

type ListRepoDiscussionResponse struct {
	Discussions []*CreateDiscussionResponse `json:"discussions"`
}

type CreateCommentRequest struct {
	Content         string `json:"content" binding:"required"`
	CommentableID   int64  `json:"commentable_id"`
	CommentableType string `json:"commentable_type"`
	CurrentUser     string `json:"-"`
}

type CreateCommentResponse struct {
	ID              int64                    `json:"id"`
	CommentableID   int64                    `json:"commentable_id"`
	CommentableType string                   `json:"commentable_type"`
	CreatedAt       time.Time                `json:"created_at"`
	User            *DiscussionResponse_User `json:"user"`
}

type UpdateCommentRequest struct {
	ID      int64  `json:"-"`
	Content string `json:"content" binding:"required"`
}
