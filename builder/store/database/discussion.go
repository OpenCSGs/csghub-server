package database

import "context"

type Discussion struct {
	ID                 int64  `bun:"id,pk,autoincrement"`
	UserID             int64  `bun:"user_id,notnull"`
	User               *User  `bun:"rel:belongs-to,join:user_id=id"`
	Title              string `bun:"title,notnull"`
	DiscussionableID   int64  `bun:"discussionable_id,notnull"`
	DiscussionableType string `bun:"discussionable_type,notnull"`
	CommentCount       int64  `bun:"comment_count,notnull,default:0"`
	times
}

type Comment struct {
	ID              int64  `bun:"id,pk,autoincrement"`
	Content         string `bun:"content"`
	CommentableType string `bun:"commentable_type,notnull"`
	CommentableID   int64  `bun:"commentable_id,notnull"`
	UserID          int64  `bun:"user_id,notnull"`
	User            *User  `bun:"rel:belongs-to,join:user_id=id"`
	times
}

const (
	CommentableTypeDiscussion = "discussion"
	CommentableTypeArticle    = "article"
)

const (
	DiscussionableTypeRepo       = "repo"
	DiscussionableTypeCollection = "collection"
)

type DiscussionStore struct {
	db *DB
}

func NewDiscussionStore() *DiscussionStore {
	return &DiscussionStore{
		db: defaultDB,
	}
}

func (s *DiscussionStore) Create(ctx context.Context, discussion Discussion) (*Discussion, error) {
	_, err := s.db.Core.NewInsert().Model(&discussion).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &discussion, nil
}

func (s *DiscussionStore) FindByID(ctx context.Context, id int64) (*Discussion, error) {
	discussion := Discussion{}
	err := s.db.Core.NewSelect().Model(&discussion).
		Where("discussion.id = ?", id).
		Relation("User").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &discussion, nil
}

func (s *DiscussionStore) FindByDiscussionableID(ctx context.Context, discussionableType string, discussionableID int64) ([]Discussion, error) {
	discussions := []Discussion{}
	err := s.db.Core.NewSelect().Model(&discussions).
		Where("discussionable_type = ? AND discussionable_id = ?", discussionableType, discussionableID).
		Relation("User").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return discussions, nil
}

func (s *DiscussionStore) UpdateByID(ctx context.Context, id int64, title string) error {
	_, err := s.db.Core.NewUpdate().Model(&Discussion{}).Set("title = ?", title).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *DiscussionStore) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model(&Discussion{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *DiscussionStore) FindDiscussionComments(ctx context.Context, discussionID int64) ([]Comment, error) {
	comments := []Comment{}
	err := s.db.Core.NewSelect().Model(&comments).
		Relation("User").
		Where("commentable_type=? AND	commentable_id = ?", CommentableTypeDiscussion, discussionID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (s *DiscussionStore) CreateComment(ctx context.Context, comment Comment) (*Comment, error) {
	_, err := s.db.Core.NewInsert().Model(&comment).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *DiscussionStore) UpdateComment(ctx context.Context, id int64, content string) error {
	_, err := s.db.Core.NewUpdate().Model(&Comment{}).Set("content = ?", content).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *DiscussionStore) FindCommentByID(ctx context.Context, id int64) (*Comment, error) {
	comment := Comment{}
	err := s.db.Core.NewSelect().Model(&comment).
		Where("comment.id = ?", id).
		Relation("User").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *DiscussionStore) DeleteComment(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model(&Comment{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}
