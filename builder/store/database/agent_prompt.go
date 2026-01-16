package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AgentPrompt represents a prompt with pin information for agent context
type AgentPrompt struct {
	ID           int64      `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64      `bun:",notnull" json:"repository_id"`
	IsPinned     bool       `bun:",scanonly" json:"is_pinned"`   // Whether the prompt is pinned (from LEFT JOIN)
	PinnedAt     *time.Time `bun:",scanonly" json:"pinned_at"`   // When the prompt was pinned (from LEFT JOIN)
	Path         string     `bun:",scanonly" json:"path"`        // Repository path (from JOIN)
	Name         string     `bun:",scanonly" json:"name"`        // Repository name (from JOIN)
	Description  string     `bun:",scanonly" json:"description"` // Repository description (from JOIN)
	Private      bool       `bun:",scanonly" json:"private"`     // Repository private flag (from JOIN)
	UserUUID     string     `bun:",scanonly" json:"user_uuid"`   // Repository owner UUID (from JOIN)
	times
}

// AgentPromptStore provides database operations for AgentPrompt
type AgentPromptStore interface {
	ListByUsername(ctx context.Context, username string, userUUID string, search string, per, page int) (prompts []AgentPrompt, total int, err error)
	FindByID(ctx context.Context, promptID int64) (*AgentPrompt, error)
}

var _ AgentPromptStore = (*agentPromptStoreImpl)(nil)

// agentPromptStoreImpl is the implementation of AgentPromptStore
type agentPromptStoreImpl struct {
	db *DB
}

// NewAgentPromptStore creates a new AgentPromptStore
func NewAgentPromptStore() AgentPromptStore {
	return &agentPromptStoreImpl{
		db: defaultDB,
	}
}

// NewAgentPromptStoreWithDB creates a new AgentPromptStore with a specific DB
func NewAgentPromptStoreWithDB(db *DB) AgentPromptStore {
	return &agentPromptStoreImpl{
		db: db,
	}
}

// ListByUsername retrieves prompts by username with pin ordering
func (s *agentPromptStoreImpl) ListByUsername(ctx context.Context, username string, userUUID string, search string, per, page int) (prompts []AgentPrompt, total int, err error) {
	// Create query with LEFT JOIN to agent_user_preferences
	// Use Model() to set the model for Count() to work properly
	query := s.db.Operator.Core.NewSelect().
		TableExpr("prompts AS p").
		ColumnExpr("p.id, p.repository_id, p.created_at, p.updated_at").
		ColumnExpr("(pin_pref.id IS NOT NULL) AS is_pinned").
		ColumnExpr("pin_pref.created_at AS pinned_at").
		ColumnExpr("r.path AS path").
		ColumnExpr("r.name AS name").
		ColumnExpr("r.description AS description").
		ColumnExpr("r.private AS private").
		Join("JOIN repositories AS r ON p.repository_id = r.id").
		Join("JOIN users AS u ON r.user_id = u.id").
		Join(`
        LEFT JOIN agent_user_preferences pin_pref
          ON pin_pref.user_uuid = ?
         AND pin_pref.action = ?
         AND pin_pref.entity_type = ?
         AND pin_pref.entity_id = CAST(p.id AS TEXT)
    `, userUUID, types.AgentUserPreferenceActionPin, types.AgentUserPreferenceEntityTypePrompt).
		Where("u.username = ?", username).
		Where("r.repository_type = ?", types.PromptRepo)

	// Apply search filter on path if provided
	search = strings.TrimSpace(search)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("LOWER(r.path) LIKE LOWER(?)", searchPattern)
	}

	total, err = query.Count(ctx)
	if err != nil {
		err = errorx.HandleDBError(err,
			errorx.Ctx().
				Set("username", username),
		)
		return
	}

	err = query.
		OrderExpr("pin_pref.created_at DESC NULLS LAST, p.updated_at DESC").
		Limit(per).
		Offset((page-1)*per).
		Scan(ctx, &prompts)
	if err != nil {
		err = errorx.HandleDBError(err,
			errorx.Ctx().
				Set("username", username),
		)
		return
	}

	return prompts, total, nil
}

// FindByID finds a prompt by ID
func (s *agentPromptStoreImpl) FindByID(ctx context.Context, promptID int64) (*AgentPrompt, error) {
	var prompt AgentPrompt
	err := s.db.Operator.Core.NewSelect().
		TableExpr("prompts AS p").
		ColumnExpr("p.id, p.repository_id, p.created_at, p.updated_at").
		ColumnExpr("r.path AS path").
		ColumnExpr("r.name AS name").
		ColumnExpr("r.description AS description").
		ColumnExpr("r.private AS private").
		ColumnExpr("u.uuid AS user_uuid").
		Join("JOIN repositories AS r ON p.repository_id = r.id").
		Join("JOIN users AS u ON r.user_id = u.id").
		Where("p.id = ?", promptID).
		Scan(ctx, &prompt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"prompt_id": promptID,
		})
	}

	return &prompt, nil
}
