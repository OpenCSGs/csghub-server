package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

const (
	MediaModerationStatusPending = "pending"
	MediaModerationStatusPass    = "pass"
	MediaModerationStatusReject  = "reject"
	MediaModerationStatusError   = "error"
)

// MediaModeration is the per-resource moderation record. resource_key is the
// idempotency key (hash of bucket+objectKey); data_id is the provider callback
// identity; task_id is the provider task handle used to poll the result.
type MediaModeration struct {
	bun.BaseModel `bun:"table:media_moderations,alias:mm"`

	ID            int64           `bun:"id,pk,autoincrement" json:"id"`
	ResourceKey   string          `bun:"resource_key,notnull,unique" json:"resource_key"`
	DataID        string          `bun:"data_id,notnull,unique" json:"data_id"`
	Seed          string          `bun:"seed,notnull" json:"seed"`
	TaskID        string          `bun:"task_id,notnull,default:''" json:"task_id"`
	MediaType     types.MediaType `bun:"media_type,notnull" json:"media_type"`
	Status        string          `bun:"status,notnull,default:'pending'" json:"status"`
	Reason        string          `bun:"reason,notnull,default:''" json:"reason"`
	LastAttemptAt *time.Time      `bun:"last_attempt_at,nullzero" json:"last_attempt_at"`
	SubmittedAt   *time.Time      `bun:"submitted_at,nullzero" json:"submitted_at,omitempty"`
	times
}

// CommentMedia links one discussion comment to one media moderation task. It is
// a many-to-many bridge: several comments can reference the same pending media
// resource, and each carries its own visibility and finalize decision. Rows are
// append-only (never updated), so they carry only created_at.
type CommentMedia struct {
	bun.BaseModel `bun:"table:comment_media,alias:cm"`

	CommentID int64           `bun:"comment_id,pk" json:"comment_id"`
	DataID    string          `bun:"data_id,pk" json:"data_id"`
	MediaType types.MediaType `bun:"media_type,notnull" json:"media_type"`
	CreatedAt time.Time       `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// CommentMediaView is the joined read model for the comment-media visibility
// and finalize logic: it carries the comment link plus the moderation status
// and provider task id resolved from media_moderations.
type CommentMediaView struct {
	CommentID int64           `bun:"comment_id" json:"comment_id"`
	DataID    string          `bun:"data_id" json:"data_id"`
	TaskID    string          `bun:"task_id" json:"task_id"`
	MediaType types.MediaType `bun:"media_type" json:"media_type"`
	Status    string          `bun:"status" json:"status"`
}

type MediaModerationStore interface {
	// CreateOrGet inserts a pending row; on unique(resource_key) conflict it
	// returns the existing row unchanged.
	CreateOrGet(ctx context.Context, m MediaModeration) (*MediaModeration, bool, error)
	FindByResourceKeys(ctx context.Context, keys []string) ([]MediaModeration, error)
	FindByDataID(ctx context.Context, dataID string) (*MediaModeration, error)
	// LinkCommentMedia idempotently associates comment→media rows.
	LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error
	// FindMediaByCommentIDs joins comment_media→media_moderations for visibility.
	FindMediaByCommentIDs(ctx context.Context, commentIDs []int64) ([]CommentMediaView, error)
	MarkSubmitted(ctx context.Context, dataID, taskID string, leaseAt time.Time) error
	UpdateResult(ctx context.Context, dataID, taskID, status, reason string) error
	// ClaimForResubmit atomically claims a row that is eligible for resubmission
	// (status 'error', or 'pending' with an empty task_id that was never
	// confirmed submitted) for re-submission, provided its last_attempt_at is
	// older than staleBefore. It resets the row to pending with an empty
	// task_id and refreshes last_attempt_at. Returns true when this caller won
	// the claim and may resubmit; false when another caller holds it or it is
	// not yet eligible.
	ClaimForResubmit(ctx context.Context, dataID string, staleBefore, leaseAt time.Time) (bool, error)
}

type mediaModerationStoreImpl struct {
	db *DB
}

func NewMediaModerationStore() MediaModerationStore {
	return &mediaModerationStoreImpl{db: defaultDB}
}

func NewMediaModerationStoreWithDB(db *DB) MediaModerationStore {
	return &mediaModerationStoreImpl{db: db}
}

func (s *mediaModerationStoreImpl) CreateOrGet(ctx context.Context, m MediaModeration) (*MediaModeration, bool, error) {
	result, err := s.db.Core.NewInsert().Model(&m).
		On("CONFLICT (resource_key) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("create media moderation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("get created media moderation row count: %w", err)
	}
	if affected == 1 {
		return &m, true, nil
	}
	existing := new(MediaModeration)
	err = s.db.Core.NewSelect().Model(existing).
		Where("resource_key = ?", m.ResourceKey).
		Scan(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("find existing media moderation: %w", err)
	}
	return existing, false, nil
}

func (s *mediaModerationStoreImpl) FindByResourceKeys(ctx context.Context, keys []string) ([]MediaModeration, error) {
	records := make([]MediaModeration, 0)
	if len(keys) == 0 {
		return records, nil
	}
	err := s.db.Core.NewSelect().Model(&records).
		Where("resource_key IN (?)", bun.In(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find media moderations by resource keys: %w", err)
	}
	return records, nil
}

func (s *mediaModerationStoreImpl) FindByDataID(ctx context.Context, dataID string) (*MediaModeration, error) {
	record := new(MediaModeration)
	err := s.db.Core.NewSelect().Model(record).
		Where("data_id = ?", dataID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find media moderation by data ID: %w", err)
	}
	return record, nil
}

func (s *mediaModerationStoreImpl) LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error {
	return insertCommentMedia(ctx, s.db.Core, commentID, items)
}

func insertCommentMedia(ctx context.Context, db bun.IDB, commentID int64, items []types.CommentMediaItem) error {
	if len(items) == 0 {
		return nil
	}
	rows := make([]CommentMedia, 0, len(items))
	for _, item := range items {
		rows = append(rows, CommentMedia{
			CommentID: commentID,
			DataID:    item.DataID,
			MediaType: item.MediaType,
		})
	}
	_, err := db.NewInsert().Model(&rows).
		On("CONFLICT (comment_id, data_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("link comment media: %w", err)
	}
	return nil
}

func (s *mediaModerationStoreImpl) FindMediaByCommentIDs(ctx context.Context, commentIDs []int64) ([]CommentMediaView, error) {
	views := make([]CommentMediaView, 0)
	if len(commentIDs) == 0 {
		return views, nil
	}
	// LEFT JOIN so a comment_media row whose data_id no longer exists in
	// media_moderations (orphan) is still returned with a NULL status. The
	// visibility logic treats NULL status as under-moderation (hidden from
	// non-authors) so an orphan link never silently makes a pending comment
	// public.
	err := s.db.Core.NewSelect().
		TableExpr("comment_media AS cm").
		ColumnExpr("cm.comment_id, cm.data_id, mm.task_id, mm.media_type, mm.status").
		Join("LEFT JOIN media_moderations AS mm ON mm.data_id = cm.data_id").
		Where("cm.comment_id IN (?)", bun.In(commentIDs)).
		Scan(ctx, &views)
	if err != nil {
		return nil, fmt.Errorf("find comment media by comment ids: %w", err)
	}
	return views, nil
}

func (s *mediaModerationStoreImpl) MarkSubmitted(ctx context.Context, dataID, taskID string, leaseAt time.Time) error {
	result, err := s.db.Core.NewUpdate().Model((*MediaModeration)(nil)).
		Set("task_id = ?", taskID).
		Set("submitted_at = CURRENT_TIMESTAMP").
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("data_id = ?", dataID).
		Where("status = ?", MediaModerationStatusPending).
		Where("task_id = ?", "").
		Where("last_attempt_at = ?", leaseAt).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark media moderation submitted: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get submitted media moderation row count: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("media moderation submission lease is no longer owned")
	}
	return nil
}

func (s *mediaModerationStoreImpl) UpdateResult(ctx context.Context, dataID, taskID, status, reason string) error {
	if status != MediaModerationStatusPass &&
		status != MediaModerationStatusReject &&
		status != MediaModerationStatusError {
		return fmt.Errorf("invalid terminal media moderation status %q", status)
	}
	_, err := s.db.Core.NewUpdate().Model((*MediaModeration)(nil)).
		Set("status = ?", status).
		Set("reason = ?", reason).
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("data_id = ?", dataID).
		Where("task_id = ?", taskID).
		Where("status IN (?)", bun.In([]string{MediaModerationStatusPending, MediaModerationStatusError})).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update media moderation result: %w", err)
	}
	return nil
}

func (s *mediaModerationStoreImpl) ClaimForResubmit(ctx context.Context, dataID string, staleBefore, leaseAt time.Time) (bool, error) {
	// Claim a row eligible for resubmission: either a previous 'error', or a
	// 'pending' row whose submission was never confirmed (empty task_id). The
	// last_attempt_at guard prevents concurrent callers from resubmitting the
	// same row within the lease window.
	result, err := s.db.Core.NewUpdate().Model((*MediaModeration)(nil)).
		Set("status = ?", MediaModerationStatusPending).
		Set("task_id = ?", "").
		Set("submitted_at = NULL").
		Set("last_attempt_at = ?", leaseAt).
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("data_id = ?", dataID).
		Where("status = ? OR (status = ? AND task_id = ?)", MediaModerationStatusError, MediaModerationStatusPending, "").
		Where("last_attempt_at IS NULL OR last_attempt_at < ?", staleBefore).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("claim media moderation for resubmit: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get claimed media moderation row count: %w", err)
	}
	return affected == 1, nil
}
