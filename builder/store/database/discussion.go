package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type Discussion struct {
	ID                 int64     `bun:"id,pk,autoincrement"`
	UserID             int64     `bun:"user_id,notnull"`
	User               *User     `bun:"rel:belongs-to,join:user_id=id"`
	Title              string    `bun:"title,notnull"`
	DiscussionableID   int64     `bun:"discussionable_id,notnull"`
	DiscussionableType string    `bun:"discussionable_type,notnull"`
	CommentCount       int64     `bun:"comment_count,notnull,default:0"`
	DeletedAt          time.Time `bun:",nullzero"`
	times
}

type Comment struct {
	ID              int64     `bun:"id,pk,autoincrement"`
	Content         string    `bun:"content"`
	CommentableType string    `bun:"commentable_type,notnull"`
	CommentableID   int64     `bun:"commentable_id,notnull"`
	UserID          int64     `bun:"user_id,notnull"`
	User            *User     `bun:"rel:belongs-to,join:user_id=id"`
	DeletedAt       time.Time `bun:",nullzero"`
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

type discussionStoreImpl struct {
	db *DB
}

type DiscussionStore interface {
	Create(ctx context.Context, discussion Discussion) (*Discussion, error)
	FindByID(ctx context.Context, id int64) (*Discussion, error)
	FindByDiscussionableID(ctx context.Context, discussionableType string, discussionableID int64, per int, page int) ([]Discussion, int, error)
	UpdateByID(ctx context.Context, id int64, title string) error
	DeleteByID(ctx context.Context, id int64) error
	FindDiscussionComments(ctx context.Context, discussionID int64, per int, page int) ([]Comment, error)
	FindVisibleDiscussionComments(ctx context.Context, req FindVisibleCommentsReq) ([]Comment, int, error)
	CreateComment(ctx context.Context, comment Comment) (*Comment, error)
	CreateCommentWithMedia(ctx context.Context, comment Comment, items []types.CommentMediaItem) (*Comment, error)
	UpdateComment(ctx context.Context, id int64, content string) error
	FindCommentByID(ctx context.Context, id int64) (*Comment, error)
	DeleteComment(ctx context.Context, id int64) error
}

type FindVisibleCommentsReq struct {
	DiscussionID int64
	CurrentUser  string
	Per          int
	Page         int
}

func NewDiscussionStore() DiscussionStore {
	return &discussionStoreImpl{
		db: defaultDB,
	}
}

func NewDiscussionStoreWithDB(db *DB) DiscussionStore {
	return &discussionStoreImpl{
		db: db,
	}
}

func (s *discussionStoreImpl) Create(ctx context.Context, discussion Discussion) (*Discussion, error) {
	_, err := s.db.Core.NewInsert().Model(&discussion).Exec(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &discussion, nil
}

func (s *discussionStoreImpl) FindByID(ctx context.Context, id int64) (*Discussion, error) {
	discussion := Discussion{}
	err := s.db.Core.NewSelect().Model(&discussion).
		Where("discussion.id = ?", id).
		Relation("User").
		Scan(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", id))
		return nil, err
	}
	return &discussion, nil
}

func (s *discussionStoreImpl) FindByDiscussionableID(ctx context.Context, discussionableType string, discussionableID int64, per int, page int) ([]Discussion, int, error) {
	discussions := []Discussion{}
	q := s.db.Core.NewSelect().Model(&discussions).
		Where("discussionable_type = ? AND discussionable_id = ?", discussionableType, discussionableID).
		Relation("User").
		Order("created_at DESC")

	total, err := q.Count(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", discussionableID).
			Set("type", discussionableType))
		return nil, 0, err
	}

	if err := q.Limit(per).Offset((page - 1) * per).Scan(ctx); err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", discussionableID).
			Set("type", discussionableType))
		return nil, 0, err
	}
	return discussions, total, nil
}

func (s *discussionStoreImpl) UpdateByID(ctx context.Context, id int64, title string) error {
	_, err := s.db.Core.NewUpdate().Model(&Discussion{}).Set("title = ?", title).Where("id = ?", id).Exec(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", id))
		return err
	}
	return nil
}

func (s *discussionStoreImpl) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model(&Discussion{}).Where("id = ?", id).ForceDelete().Exec(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", id))
		return err
	}
	return nil
}

func (s *discussionStoreImpl) FindDiscussionComments(ctx context.Context, discussionID int64, per int, page int) ([]Comment, error) {
	comments := []Comment{}
	err := s.db.Core.NewSelect().Model(&comments).
		Relation("User").
		Where("commentable_type=? AND commentable_id=?", CommentableTypeDiscussion, discussionID).
		Order("comment.created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().
			Set("id", discussionID))
	}

	return comments, nil
}

func (s *discussionStoreImpl) FindVisibleDiscussionComments(ctx context.Context, req FindVisibleCommentsReq) ([]Comment, int, error) {
	comments := make([]Comment, 0)
	visibility := `NOT EXISTS (
		SELECT 1 FROM comment_media AS cm
		LEFT JOIN media_moderations AS mm ON mm.data_id = cm.data_id
		WHERE cm.comment_id = comment.id AND COALESCE(mm.status, '') <> ?
	) OR EXISTS (
		SELECT 1 FROM users AS u WHERE u.id = comment.user_id AND u.username = ?
	)`
	baseQuery := s.db.Core.NewSelect().Model(&comments).
		Where("commentable_type = ? AND commentable_id = ?", CommentableTypeDiscussion, req.DiscussionID).
		Where("("+visibility+")", MediaModerationStatusPass, req.CurrentUser)
	total, err := baseQuery.Clone().Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("id", req.DiscussionID))
	}
	err = baseQuery.Relation("User").
		Order("comment.created_at DESC").
		Limit(req.Per).
		Offset((req.Page - 1) * req.Per).
		Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("id", req.DiscussionID))
	}
	return comments, total, nil
}

func (s *discussionStoreImpl) CreateComment(ctx context.Context, comment Comment) (*Comment, error) {
	return s.CreateCommentWithMedia(ctx, comment, nil)
}

// CreateCommentWithMedia creates a comment and its moderation links in one
// transaction. Readers can therefore never observe a pending-media comment
// without the links that make it author-only.
func (s *discussionStoreImpl) CreateCommentWithMedia(ctx context.Context, comment Comment, items []types.CommentMediaItem) (*Comment, error) {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().Model(&comment).Exec(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}

		if err := insertCommentMedia(ctx, tx, comment.ID, items); err != nil {
			return err
		}

		discussion := Discussion{}
		err = tx.NewSelect().Model(&discussion).Where("id = ?", comment.CommentableID).For("UPDATE NOWAIT").Scan(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}
		_, err = tx.NewUpdate().Model(&discussion).Set("comment_count = comment_count + 1").Where("id = ?", comment.CommentableID).Exec(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func (s *discussionStoreImpl) UpdateComment(ctx context.Context, id int64, content string) error {
	_, err := s.db.Core.NewUpdate().Model(&Comment{}).Set("content = ?", content).Where("id = ?", id).Exec(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, nil)
		return err
	}
	return nil
}

func (s *discussionStoreImpl) FindCommentByID(ctx context.Context, id int64) (*Comment, error) {
	comment := Comment{}
	err := s.db.Core.NewSelect().Model(&comment).
		Where("comment.id = ?", id).
		Relation("User").
		Scan(ctx)
	if err != nil {
		err := errorx.HandleDBError(err, nil)
		return nil, err
	}
	return &comment, nil
}

func (s *discussionStoreImpl) DeleteComment(ctx context.Context, id int64) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		comment := Comment{}
		err := tx.NewSelect().Model(&comment).Where("id = ?", id).For("UPDATE NOWAIT").Scan(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}

		if _, err = tx.NewDelete().Model((*CommentMedia)(nil)).Where("comment_id = ?", id).Exec(ctx); err != nil {
			return errorx.HandleDBError(err, nil)
		}

		_, err = tx.NewDelete().Model(&Comment{}).Where("id = ?", id).ForceDelete().Exec(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}

		discussion := Discussion{}
		err = tx.NewSelect().Model(&discussion).Where("id = ?", comment.CommentableID).For("UPDATE NOWAIT").Scan(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}

		_, err = tx.NewUpdate().Model(&discussion).Set("comment_count = comment_count - 1").Where("id = ?", comment.CommentableID).Exec(ctx)
		if err != nil {
			err := errorx.HandleDBError(err, nil)
			return err
		}
		return nil
	})

	if err != nil {
		err := errorx.HandleDBError(err, errorx.Ctx().
			Set("id", id))
		return err
	}
	return nil
}
