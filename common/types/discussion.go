package types

import (
	"time"

	"opencsg.com/csghub-server/builder/sensitive"
)

type CreateRepoDiscussionRequest struct {
	Title       string         `json:"title" binding:"required"`
	RepoType    RepositoryType `json:"-"`
	Namespace   string         `json:"-"`
	Name        string         `json:"-"`
	CurrentUser string         `json:"-"`
}

// CreateRepoDiscussionRequest implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*CreateRepoDiscussionRequest)(nil)

func (req *CreateRepoDiscussionRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "title",
			Value: func() string {
				return req.Title
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		},
	}
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

// UpdateDiscussionRequest implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*UpdateDiscussionRequest)(nil)

func (req *UpdateDiscussionRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "title",
			Value: func() string {
				return req.Title
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		},
	}
}

type ShowDiscussionResponse struct {
	ID           int64                    `json:"id"`
	Title        string                   `json:"title"`
	User         *DiscussionResponse_User `json:"user"`
	CommentCount int64                    `json:"comment_count"`
	Comments     *CommentsWithPagination  `json:"comments,omitempty"`
}

type CommentsWithPagination struct {
	Data  []*DiscussionResponse_Comment `json:"data"`
	Total int                           `json:"total"`
	Page  int                           `json:"page"`
	Per   int                           `json:"per"`
}

type DiscussionResponse_Comment struct {
	ID        int64                    `json:"id"`
	Content   string                   `json:"content"`
	User      *DiscussionResponse_User `json:"user"`
	CreatedAt time.Time                `json:"created_at"`
}

type ListRepoDiscussionRequest struct {
	RepoType    RepositoryType `json:"-"`
	Namespace   string         `json:"-"`
	Name        string         `json:"-"`
	CurrentUser string         `json:"-"`
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

// CreateCommentRequest implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*CreateCommentRequest)(nil)

func (req *CreateCommentRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "content",
			Value: func() string {
				return req.Content
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		},
	}
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

// UpdateCommentRequest implements SensitiveRequestV2
var _ SensitiveRequestV2 = (*UpdateCommentRequest)(nil)

func (req *UpdateCommentRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "content",
			Value: func() string {
				return req.Content
			},
			Scenario: string(sensitive.ScenarioCommentDetection),
		},
	}
}
