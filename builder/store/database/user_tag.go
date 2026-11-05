package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

type UserTag struct {
	ID     int64 `bun:"column:pk,autoincrement"`
	UserID int64 `bun:",notnull"`
	TagID  int64 `bun:",notnull"`

	Tag Tag `bun:"rel:belongs-to,join:tag_id=id"`

	times
}

type UserTagStore interface {
	ResetUserTags(ctx context.Context, userId int64, tagIDs []int64) error
	// ResetUserTagsAsOperator replaces the USER-SCOPE subset of the user's
	// tags (routing tags included) and preserves tags of other scopes. It is
	// the administrator write path (operators own the routing tiers),
	// distinct from the self-service reset which must never add or clear the
	// routing tags. It writes the audit record in the same transaction and
	// returns the exact user-scope set that was replaced.
	ResetUserTagsAsOperator(ctx context.Context, req UserTagOperatorResetRequest) ([]*Tag, error)
	GetUserTags(ctx context.Context, userId int64) ([]*Tag, error)
	// GetUserRoutingTagsByUUID returns ONLY the tags that affect upstream
	// channel routing (low_priority and priority:<n>) of the user with the
	// given uuid, resolved in a single join (users + user_tags + tags).
	// No matching user or no routing tag yields (nil, nil).
	GetUserRoutingTagsByUUID(ctx context.Context, userUUID string) ([]*Tag, error)
}

// UserTagOperatorResetRequest describes an administrator replacement. The
// builder uses the request to persist one logical before/after audit record
// atomically with the relation update.
type UserTagOperatorResetRequest struct {
	UserID     int64
	TagIDs     []int64
	OperatorID string
	UserUUID   string
	UserName   string
}

type userTagStoreImpl struct {
	db *DB
}

func NewUserTagStore() UserTagStore {
	return &userTagStoreImpl{
		db: defaultDB,
	}
}

func NewUserTagStoreWithDB(db *DB) UserTagStore {
	return &userTagStoreImpl{
		db: db,
	}
}

// ResetUserTags replaces the user's SELF-SERVICE tag set (personal tag
// endpoints and the profile update path). Upstream channel routing tags
// (low_priority / priority:<n>) are operator-managed entitlements, not
// profile labels: the gateway reads them to decide which upstream channels a
// caller may use. A self-service reset therefore neither grants nor clears
// them.
//
// Example: a user currently tagged [low_priority, Automotive] submitting
// [Automotive, priority:100] ends up with [low_priority, Automotive] (the
// routing grant is ignored, the interest tag is replaced); submitting []
// keeps [low_priority] (the isolation cannot be lifted).
//
// Contrast ResetUserTagsAsOperator, the administrator path.
func (s *userTagStoreImpl) ResetUserTags(ctx context.Context, userId int64, tagIDs []int64) error {
	selfServiceTagIDs, err := s.selfServiceTagIDs(ctx, tagIDs)
	if err != nil {
		return err
	}

	userTags := make([]UserTag, 0, len(selfServiceTagIDs))
	for _, tagID := range selfServiceTagIDs {
		userTags = append(userTags, UserTag{
			UserID: userId,
			TagID:  tagID,
		})
	}
	// Self-service resets preserve the operator-managed routing tags.
	_, err = s.replaceUserTags(ctx, userId, userTags, userTagReplaceSelfService, nil)
	return err
}

// userTagReplaceMode selects which of the user's existing tag rows a
// replacement deletes.
type userTagReplaceMode int

const (
	// userTagReplaceSelfService deletes every tag except the operator-managed
	// routing tags (user-scope low_priority / priority:<n>). The client sends
	// the user's full tag selection, so other tags are replaced as before.
	userTagReplaceSelfService userTagReplaceMode = iota
	// userTagReplaceUserScope deletes only the user-scope subset (routing tags
	// included) and preserves tags of other scopes. It is the administrator
	// path and matches what AdminGetUserTags displays.
	userTagReplaceUserScope
)

// replaceUserTags replaces the user's tag set in one transaction. The insert
// decision is made before the transaction opens, so the body runs only the
// statements it needs. A per-user advisory lock serializes concurrent
// replacements: without it two overlapping deletes can commit a union of both
// requests instead of one of them.
func (s *userTagStoreImpl) replaceUserTags(ctx context.Context, userId int64, userTags []UserTag, mode userTagReplaceMode, audit *AuditLog) ([]*Tag, error) {
	insertTags := len(userTags) > 0
	var before []*Tag
	if mode == userTagReplaceUserScope {
		// The administrator operation writes one aggregate audit record below.
		// Suppress per-row UPDATE/DELETE hook records for the same change.
		ctx = withUserTagAuditSuppressed(ctx)
	}
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if tx.Dialect().Name() == dialect.PG {
			if _, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
				fmt.Sprintf("user_tags:%d", userId)).Exec(ctx); err != nil {
				return fmt.Errorf("lock user tags: %w", err)
			}
		}
		if mode == userTagReplaceUserScope {
			// Capture the exact set this replacement deletes. Reading it in
			// the same transaction and lock keeps the audit record identical
			// to what is actually replaced (a read outside could see another
			// writer's intermediate state).
			if err := tx.NewSelect().Model(&before).
				Join("INNER JOIN user_tags ON user_tags.tag_id = tag.id").
				Where("user_tags.user_id = ?", userId).
				Where("tag.scope = ?", types.UserTagScope).
				Scan(ctx); err != nil {
				return fmt.Errorf("load user-scope tags before replacement: %w", err)
			}
		}
		deleteQuery := tx.NewDelete().Model((*UserTag)(nil)).Where("user_id = ?", userId)
		switch mode {
		case userTagReplaceSelfService:
			// Preserve the operator-managed routing tags across the reset.
			deleteQuery = deleteQuery.Where(
				"tag_id NOT IN (SELECT routing_tag.id FROM tags AS routing_tag WHERE routing_tag.scope = ? AND (routing_tag.name = ? OR routing_tag.name LIKE ?))",
				types.UserTagScope, types.UserTagLowPriority, types.UserTagPriorityPrefix+"%")
		case userTagReplaceUserScope:
			// Replace the user-scope subset only; interest tags from other
			// scopes are not part of the admin surface.
			deleteQuery = deleteQuery.Where(
				"tag_id IN (SELECT user_scope_tag.id FROM tags AS user_scope_tag WHERE user_scope_tag.scope = ?)",
				types.UserTagScope)
		}
		if _, err := deleteQuery.Exec(ctx); err != nil {
			return err
		}
		if insertTags {
			if _, err := tx.NewInsert().Model(&userTags).Exec(ctx); err != nil {
				return err
			}
		}
		if audit != nil {
			beforeIDs := make([]int64, 0, len(before))
			beforeNames := make([]string, 0, len(before))
			for _, tag := range before {
				beforeIDs = append(beforeIDs, tag.ID)
				beforeNames = append(beforeNames, tag.Name)
			}
			beforeJSON, err := json.Marshal(struct {
				TagIDs   []int64  `json:"tag_ids"`
				TagNames []string `json:"tag_names"`
			}{TagIDs: beforeIDs, TagNames: beforeNames})
			if err != nil {
				return fmt.Errorf("marshal user tag audit before image: %w", err)
			}
			audit.Before = beforeJSON
			if _, err := tx.NewInsert().Model(audit).Exec(ctx); err != nil {
				return fmt.Errorf("insert user tag audit log: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return before, nil
}

// selfServiceTagIDs keeps only the requested tag ids that exist and are not in
// the operator-managed routing namespace, preserving the caller's order.
func (s *userTagStoreImpl) selfServiceTagIDs(ctx context.Context, tagIDs []int64) ([]int64, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	var tags []*Tag
	err := s.db.Operator.Core.NewSelect().Model(&tags).
		Column("id", "name", "scope").
		Where("tag.id IN (?)", bun.In(tagIDs)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("load user tags to reset: %w", err)
	}
	allowed := make(map[int64]struct{}, len(tags))
	for _, tag := range tags {
		// Only user-scope routing tags are operator-managed entitlements
		// (the read side matches scope='user' too); a same-named tag from
		// another scope stays an ordinary self-service label.
		if tag.Scope == types.UserTagScope && types.IsUserRoutingTagName(tag.Name) {
			continue
		}
		allowed[tag.ID] = struct{}{}
	}
	ids := make([]int64, 0, len(allowed))
	for _, tagID := range tagIDs {
		if _, ok := allowed[tagID]; ok {
			ids = append(ids, tagID)
		}
	}
	return ids, nil
}

// ResetUserTagsAsOperator replaces the user's USER-SCOPE tag set, routing tags
// included: administrators own the upstream channel tiers, so every user-scope
// id passed here is written as requested and previously assigned user-scope
// tags (routing included) are replaced. Tags of other scopes (e.g. model-scope
// interest tags) are preserved — the admin API only shows and edits the
// user-scope set, so it must not delete what it cannot display. The before/
// after audit record commits atomically with this replacement.
//
// Example: an admin setting [priority:100] for a user tagged
// [low_priority, Automotive + a model-scope interest tag] leaves
// [priority:100, <interest tag>].
//
// Contrast ResetUserTags, the self-service path which must not touch routing
// tags.
func (s *userTagStoreImpl) ResetUserTagsAsOperator(ctx context.Context, req UserTagOperatorResetRequest) ([]*Tag, error) {
	tagIDs := make([]int64, 0, len(req.TagIDs))
	tagIDs = append(tagIDs, req.TagIDs...)
	userTags := make([]UserTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		userTags = append(userTags, UserTag{UserID: req.UserID, TagID: tagID})
	}
	afterJSON, err := json.Marshal(struct {
		TagIDs []int64 `json:"tag_ids"`
	}{TagIDs: tagIDs})
	if err != nil {
		return nil, fmt.Errorf("marshal user tag audit after image: %w", err)
	}
	audit := &AuditLog{
		TableName:  "user_tags",
		Action:     enum.AuditActionUpdate,
		OperatorID: req.OperatorID,
		After:      afterJSON,
		ResourceID: req.UserUUID,
		UserName:   req.UserName,
	}
	return s.replaceUserTags(ctx, req.UserID, userTags, userTagReplaceUserScope, audit)
}

func (s *userTagStoreImpl) GetUserTags(ctx context.Context, userId int64) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.Operator.Core.NewSelect().Model((*Tag)(nil)).
		Join("INNER JOIN user_tags ON user_tags.tag_id = tag.id").
		Where("user_tags.user_id = ?", userId).
		Scan(ctx, &tags)
	if err != nil {
		return nil, err
	}

	return tags, nil
}

// GetUserRoutingTagsByUUID resolves the routing-relevant tags of one user by
// uuid in a single statement. Row selection is restricted to the tag
// vocabulary the gateway consumes (low_priority and priority:<n>), so the
// caller never pays for the user's other tags; the numeric tier derivation
// stays in Go (types.UserRoutingPriorityFromTags), which keeps the query
// portable across the postgres/sqlite dialects instead of relying on
// dialect-specific regexp casts.
func (s *userTagStoreImpl) GetUserRoutingTagsByUUID(ctx context.Context, userUUID string) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.Operator.Core.NewSelect().Model((*Tag)(nil)).
		Join("INNER JOIN user_tags ON user_tags.tag_id = tag.id").
		Join("INNER JOIN users ON users.id = user_tags.user_id").
		Where("users.uuid = ?", userUUID).
		Where("(tag.name = ? OR tag.name LIKE ?)", types.UserTagLowPriority, types.UserTagPriorityPrefix+"%").
		Where("tag.scope = ?", types.UserTagScope).
		Scan(ctx, &tags)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return tags, nil
}
